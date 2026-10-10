package gapfill

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// fixture is a project root, a cache dir, and a stub classifier that saves
// each request and answers with whatever answer last wrote.
type fixture struct {
	t                   *testing.T
	root, cacheDir, dir string
	cfg                 *config.Config
}

func newFixture(t *testing.T, extra string) *fixture {
	t.Helper()
	f := &fixture{t: t, root: t.TempDir(), cacheDir: t.TempDir(), dir: t.TempDir()}
	stub := filepath.Join(f.dir, "classifier")
	testutil.FakeTool(t, stub, filepath.Join(f.dir, "calls"), fmt.Sprintf("cat >%q\n%s\ncat %q", filepath.Join(f.dir, "request.json"), extra, filepath.Join(f.dir, "response.json")))
	f.cfg = &config.Config{
		Classifier:    []string{stub},
		LockfileGlobs: []string{"pnpm-lock.yaml", "package-lock.json", "yarn.lock"},
		GateDiscovery: config.GateDiscovery{DenyCommands: []string{"npm publish"}},
	}
	return f
}

// answer is the structured output the stub returns from now on.
func (f *fixture) answer(structured any) {
	f.t.Helper()
	data, err := json.Marshal(map[string]any{"is_error": false, "subtype": "success", "structured_output": structured})
	if err != nil {
		f.t.Fatal(err)
	}
	testutil.Put(f.t, f.dir, "response.json", string(data))
}

func (f *fixture) run(entries ...sources.Entry) Result {
	return Run(f.cfg, Options{Root: f.root, CacheDir: f.cacheDir, Entries: entries, Ask: true, Timeout: 10 * time.Second})
}

func (f *fixture) calls() []string { return testutil.Calls(f.t, filepath.Join(f.dir, "calls")) }

// asked is the ids of the entries the last request held.
func (f *fixture) asked() []string {
	f.t.Helper()
	var req struct {
		Entries []struct{ ID string } `json:"entries"`
	}
	if err := json.Unmarshal([]byte(testutil.Read(f.t, filepath.Join(f.dir, "request.json"))), &req); err != nil {
		f.t.Fatal(err)
	}
	var ids []string
	for _, e := range req.Entries {
		ids = append(ids, e.ID)
	}
	return ids
}

func entry(name, body string) sources.Entry {
	return sources.Entry{Source: "package-scripts", File: "package.json", Name: name, Dir: ".", Body: sources.Body{Text: body}}
}

func verdict(e sources.Entry, v map[string]any) map[string]any {
	v["id"] = Key(e)
	return v
}

func answers(entries ...map[string]any) map[string]any {
	return map[string]any{"entries": entries, "managers": []any{}, "proposals": []any{}}
}

func TestCacheHitAndPerBodyInvalidation(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	lint, test := entry("lint", "eslint ."), entry("test", "vitest run")
	f.answer(answers(verdict(lint, map[string]any{"role": "check", "kind": "lint", "mutates": false}), verdict(test, map[string]any{"role": "check", "kind": "test", "mutates": false})))
	r := f.run(lint, test)
	if got := r.Verdicts[Key(lint)]; got.Role != "check" || got.Kind != "lint" || r.Unclassified() != 0 {
		t.Fatalf("first run: %+v", r)
	}
	if calls := f.calls(); len(calls) != 1 || !strings.HasPrefix(calls[0], filepath.Join(fsx.PhysicalPath(f.cacheDir), cache.Enforce)+"|") {
		t.Fatalf("the classifier runs once, from the cache dir: %q", calls)
	}

	if r := f.run(lint, test); len(f.calls()) != 1 || r.Verdicts[Key(test)].Kind != "test" {
		t.Errorf("a second run asks again (%d calls) or loses the answer: %+v", len(f.calls()), r)
	}

	changed := entry("test", "vitest run --coverage")
	f.answer(answers(verdict(changed, map[string]any{"role": "check", "kind": "test", "mutates": false})))
	f.run(lint, changed)
	if asked := f.asked(); len(f.calls()) != 2 || len(asked) != 1 || asked[0] != Key(changed) {
		t.Errorf("a changed body asks for that entry alone: %d calls, asked %q", len(f.calls()), asked)
	}
}

func TestMalformedOutputLeavesEntriesUnclassifiedAndUncached(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	testutil.Put(t, f.dir, "response.json", "not json")
	lint := entry("lint", "eslint .")
	r := f.run(lint)
	if why := r.Skipped[Key(lint)]; !strings.HasPrefix(why, "unclassified (classifier: malformed output") || r.Unclassified() != 1 {
		t.Errorf("skipped = %q", why)
	}
	f.run(lint)
	if len(f.calls()) != 2 {
		t.Errorf("a failed answer is cached: %d calls", len(f.calls()))
	}
}

func TestErrorEnvelopeIsNoAnswer(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	testutil.Put(t, f.dir, "response.json", `{"is_error":true,"subtype":"error_max_budget_usd"}`)
	lint := entry("lint", "eslint .")
	if why := f.run(lint).Skipped[Key(lint)]; why != "unclassified (classifier: no answer (error_max_budget_usd))" {
		t.Errorf("skipped = %q", why)
	}
}

// Not parallel: it sets the guard in the process environment.
func TestTheGuardNeverAsks(t *testing.T) {
	f := newFixture(t, "")
	t.Setenv(Guard, "1")
	lint := entry("lint", "eslint .")
	if r := f.run(lint); r.Skipped[Key(lint)] != "unclassified" || len(f.calls()) != 0 {
		t.Errorf("under the guard: %+v, %d calls", r, len(f.calls()))
	}
}

func TestConcurrentRunsAskOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "sleep 0.5")
	lint := entry("lint", "eslint .")
	f.answer(answers(verdict(lint, map[string]any{"role": "check", "kind": "lint", "mutates": false})))
	var wg sync.WaitGroup
	results := make([]Result, 2)
	for i := range results {
		wg.Go(func() { results[i] = f.run(lint) })
	}
	wg.Wait()
	if len(f.calls()) != 1 {
		t.Errorf("%d calls; the waiting run should read the first one's answer", len(f.calls()))
	}
	for _, r := range results {
		if r.Verdicts[Key(lint)].Kind != "lint" {
			t.Errorf("a run lacks the answer: %+v", r)
		}
	}
}

func TestDeniedEntriesAreNeverAsked(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	release, lint := entry("release", "npm run build && npm publish"), entry("lint", "eslint .")
	f.answer(answers(verdict(lint, map[string]any{"role": "check", "kind": "lint", "mutates": false})))
	r := f.run(release, lint)
	if r.Skipped[Key(release)] != "denied (npm publish)" || r.Unclassified() != 0 {
		t.Errorf("result: %+v", r)
	}
	if asked := f.asked(); len(asked) != 1 || asked[0] != Key(lint) {
		t.Errorf("asked %q", asked)
	}
}

func TestVerification(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	testutil.Put(t, f.root, "src/a.ts", "")
	testutil.Put(t, f.root, "pnpm-lock.yaml", "")
	testutil.Put(t, f.root, "package.json", `{"devDependencies":{"eslint":"9.12.0"}}`)
	ci := entry("ci", "eslint src && tsc --noEmit && vitest run")
	fetch := entry("fmt", "prettier --check .")
	invent := entry("test", "jest")
	strayed := entry("types", "tsc --noEmit")
	badKind := entry("check", "make check")
	aggregate := entry("all", "make lint\nmake test")
	f.answer(map[string]any{
		"entries": []any{
			verdict(ci, map[string]any{"role": "check", "kind": "lint", "mutates": false, "file_form": "pnpm exec eslint --max-warnings=0 {files}", "segments": []any{
				map[string]any{"text": "eslint src", "role": "check", "kind": "lint"},
				map[string]any{"text": "tsc --noEmit", "role": "check", "kind": "typecheck"},
				map[string]any{"text": "vitest run", "role": "check", "kind": "test"},
			}}),
			verdict(fetch, map[string]any{"role": "check", "kind": "format-check", "mutates": false, "file_form": "npx prettier --check {files}"}),
			verdict(invent, map[string]any{"role": "check", "kind": "test", "mutates": false, "file_form": "node -e require('jest').run({files})"}),
			verdict(strayed, map[string]any{"role": "check", "kind": "typecheck", "mutates": false, "segments": []any{map[string]any{"text": "tsc -b", "role": "check", "kind": "typecheck"}}}),
			verdict(badKind, map[string]any{"role": "check", "kind": "style", "mutates": false}),
			verdict(aggregate, map[string]any{"role": "check", "mutates": false}),
		},
		"managers": []any{
			map[string]any{"dir": ".", "cites": "pnpm-lock.yaml", "manager": "pnpm", "run_prefix": "pnpm exec", "add_verbs": []string{"pnpm add"}, "dlx": "pnpm dlx", "dir_flags": []string{"--filter"}, "rivals": []string{"npm"}},
			map[string]any{"dir": ".", "cites": "yarn.lock", "manager": "yarn", "run_prefix": "yarn", "add_verbs": []string{}, "dlx": "", "dir_flags": []string{}, "rivals": []string{}},
		},
		"proposals": []any{
			map[string]any{"role": "check", "kind": "lint", "command": "pnpm exec eslint .", "dir": ".", "evidence": "package.json", "mutates": false},
			map[string]any{"role": "fixer", "command": "pnpm dlx prettier --write .", "dir": ".", "evidence": "pnpm-lock.yaml", "mutates": true},
			map[string]any{"role": "check", "kind": "security", "command": "gitleaks detect", "dir": ".", "evidence": ".gitleaks.toml", "mutates": false},
		},
	})
	r := f.run(ci, fetch, invent, strayed, badKind, aggregate)
	if v, ok := r.Verdicts[Key(aggregate)]; !ok || v.Role != "check" {
		t.Errorf("an aggregate is a check of no one kind: %+v", r.Skipped[Key(aggregate)])
	}

	v := r.Verdicts[Key(ci)]
	if v.FileForm != "pnpm exec eslint --max-warnings=0 {files}" || len(v.Segments) != 3 || v.Segments[1].Kind != "typecheck" {
		t.Errorf("segments and a prefixed form with an added flag hold up: %+v", v)
	}
	if v := r.Verdicts[Key(fetch)]; v.Role != "check" || v.FileForm != "" {
		t.Errorf("npx with no local copy: the form goes, the verdict stays: %+v", v)
	}
	if v := r.Verdicts[Key(invent)]; v.FileForm != "" {
		t.Errorf("a form with words the body lacks goes: %+v", v)
	}
	if why := r.Skipped[Key(strayed)]; why != `rejected (segment "tsc -b" isn't in the body)` {
		t.Errorf("strayed: %q", why)
	}
	if why := r.Skipped[Key(badKind)]; why != `rejected (check kind "style")` {
		t.Errorf("bad kind: %q", why)
	}
	if len(r.Managers) != 1 || r.Managers[0].Manager != "pnpm" {
		t.Errorf("only the manager whose lockfile exists stands: %+v", r.Managers)
	}
	if len(r.Proposals) != 1 || r.Proposals[0].Command != "pnpm exec eslint ." {
		t.Errorf("a fetching fixer and missing evidence go; a lint the entries cover stays, for the plan to weigh: %+v", r.Proposals)
	}
	for _, want := range []string{
		`package-scripts fmt: file form "npx prettier --check {files}": fetches (npx prettier with no local copy)`,
		`package-scripts test: file form "node -e require('jest').run({files})": not the body's own words`,
		`manager for .: no lockfile yarn.lock`,
		`proposal "pnpm dlx prettier --write .": fetches (pnpm dlx)`,
		`proposal "gitleaks detect": no evidence file ".gitleaks.toml"`,
	} {
		if !strings.Contains(strings.Join(r.Dropped, "\n"), want) {
			t.Errorf("dropped lacks %q:\n%s", want, strings.Join(r.Dropped, "\n"))
		}
	}

	testutil.Put(t, f.root, "yarn.lock", "")
	if r := f.run(ci, fetch, invent, strayed, badKind, aggregate); len(r.Managers) != 2 {
		t.Errorf("verified on every read: the lockfile it cites now exists: %+v", r.Managers)
	}
}

func TestManagersReadsVerifiedFactsWithoutAsking(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	testutil.Put(t, f.root, "pnpm-lock.yaml", "")
	if ms := Managers(f.cfg, f.root, f.cacheDir); len(ms) != 0 || len(f.calls()) != 0 {
		t.Errorf("a cold cache: %+v, %d calls", ms, len(f.calls()))
	}
	f.answer(map[string]any{"entries": []any{}, "proposals": []any{}, "managers": []any{
		map[string]any{"dir": ".", "cites": "pnpm-lock.yaml", "manager": "pnpm@9.1.0", "add_verbs": []string{"pnpm add"}},
		map[string]any{"dir": ".", "cites": "yarn.lock", "manager": "yarn"},
	}})
	f.run(entry("lint", "eslint ."))
	if ms := Managers(f.cfg, f.root, f.cacheDir); len(ms) != 1 || ms[0].Manager != "pnpm" || len(f.calls()) != 1 {
		t.Errorf("only the fact whose lockfile exists, named alone: %+v, %d calls", ms, len(f.calls()))
	}
}

func TestAPlaceholderReplacesAListOfPaths(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	testutil.Put(t, f.root, "bin/kit", "")
	testutil.Put(t, f.root, "build.sh", "")
	sh := entry("shellcheck", "shellcheck -x bin/kit build.sh")
	f.answer(answers(verdict(sh, map[string]any{"role": "check", "kind": "lint", "mutates": false, "file_form": "shellcheck -x {files}"})))
	if v := f.run(sh).Verdicts[Key(sh)]; v.FileForm != "shellcheck -x {files}" {
		t.Errorf("%+v", v)
	}
}

// A tool taking packages runs on the edited files' directories.
func TestADirsPlaceholderReplacesAPackagePattern(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	vet := entry("vet", "go vet ./...")
	f.answer(answers(verdict(vet, map[string]any{"role": "check", "kind": "lint", "mutates": false, "file_form": "go vet {dirs}"})))
	if v := f.run(vet).Verdicts[Key(vet)]; v.FileForm != "go vet {dirs}" {
		t.Errorf("%+v", v)
	}
}

// npx is npm's run prefix, and it fetches what isn't installed: a form
// through it holds up only once the project has its own copy.
func TestNpxHoldsUpWithALocalCopy(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	testutil.Put(t, f.root, "package-lock.json", "{}")
	testutil.Put(t, f.root, ".gitignore", "node_modules\n")
	testutil.Git(t, f.root, "init", "-q")
	format := entry("fmt", "prettier --check .")
	f.answer(map[string]any{
		"entries":   []any{verdict(format, map[string]any{"role": "check", "kind": "format-check", "mutates": false, "file_form": "npx prettier --check {files}"})},
		"managers":  []any{map[string]any{"dir": ".", "cites": "package-lock.json", "manager": "npm", "run_prefix": "npx", "add_verbs": []string{"npm install"}, "dlx": "npx", "dir_flags": []string{"--prefix"}, "rivals": []string{}}},
		"proposals": []any{},
	})
	if v := f.run(format).Verdicts[Key(format)]; v.FileForm != "" {
		t.Errorf("no local copy: %+v", v)
	}
	testutil.FakeTool(t, filepath.Join(f.root, "node_modules/.bin/prettier"), filepath.Join(f.dir, "prettier.calls"), "")
	if v := f.run(format).Verdicts[Key(format)]; v.FileForm != "npx prettier --check {files}" {
		t.Errorf("a local copy: %+v", v)
	}
}
