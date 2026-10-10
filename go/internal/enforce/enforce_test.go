package enforce

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// env is one plan test: a git repo ignoring node_modules and bin, a PATH of
// its own (git aside), and a stub classifier answering with verdicts set by
// entry name.
type env struct {
	t                        *testing.T
	root, path, stubs, cache string
	cfg                      *config.Config
	verdicts                 map[string]map[string]any // by entry name
	proposals                []map[string]any
	managers                 []map[string]any
}

// Not parallel: each test sets PATH.
func setup(t *testing.T) *env {
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, root: filepath.Join(tmp, "repo"), path: filepath.Join(tmp, "path"), stubs: filepath.Join(tmp, "stubs"), cache: filepath.Join(tmp, "cache"), verdicts: map[string]map[string]any{}}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	testutil.Put(t, e.path, ".keep", "")
	if err := os.Symlink(git, filepath.Join(e.path, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", e.path+":/usr/bin:/bin")
	t.Setenv("HOME", tmp)
	testutil.Put(t, e.root, ".gitignore", "node_modules\n/bin\n")
	testutil.Git(t, e.root, "init", "-q")
	e.cfg = testutil.KitConfig(t)
	return e
}

func (e *env) write(name, body string) { testutil.Put(e.t, e.root, name, body) }

// tool puts an executable at path (relative to the root, or absolute).
func (e *env) tool(path string) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.root, path)
	}
	testutil.FakeTool(e.t, path, filepath.Join(e.stubs, "calls"), "")
}

// check sets the verdict for entries named name.
func (e *env) check(name string, v map[string]any) { e.verdicts[name] = v }

// plan builds the full gate, the stub answering each entry by its name.
// Subprojects are found among tracked files, so everything is staged.
func (e *env) plan() Plan {
	e.t.Helper()
	testutil.Git(e.t, e.root, "add", "-A")
	entries := sources.Entries(e.cfg, e.root, project.Subprojects(e.cfg, e.root))
	var answers []map[string]any
	for _, entry := range entries {
		if v, ok := e.verdicts[entry.Name]; ok {
			a := map[string]any{"id": gapfill.Key(entry), "mutates": false}
			for k, val := range v {
				a[k] = val
			}
			answers = append(answers, a)
		}
	}
	envelope, err := json.Marshal(map[string]any{"is_error": false, "subtype": "success", "structured_output": map[string]any{
		"entries": answers, "managers": orEmpty(e.managers), "proposals": orEmpty(e.proposals)}})
	if err != nil {
		e.t.Fatal(err)
	}
	testutil.Put(e.t, e.stubs, "answer.json", string(envelope))
	e.cfg.Classifier = testutil.Replayer(e.t, filepath.Join(e.stubs, "answer.json"))
	return Build(e.cfg, Options{Root: e.root, CacheDir: e.cache, Ask: true, Timeout: 10 * time.Second})
}

func orEmpty(m []map[string]any) []map[string]any {
	if m == nil {
		return []map[string]any{}
	}
	return m
}

// printed is p as --plan prints it.
func printed(p Plan) string {
	var b strings.Builder
	p.Print(&b)
	return b.String()
}

func has(t *testing.T, p Plan, parts ...string) {
	t.Helper()
	out := printed(p)
	for _, part := range parts {
		if !strings.Contains(out, part) {
			t.Errorf("plan lacks %q:\n%s", part, out)
		}
	}
}

func lacks(t *testing.T, p Plan, parts ...string) {
	t.Helper()
	out := printed(p)
	for _, part := range parts {
		if strings.Contains(out, part) {
			t.Errorf("plan has %q:\n%s", part, out)
		}
	}
}

func check(kind string) map[string]any { return map[string]any{"role": "check", "kind": kind} }

func TestAnEmptyProjectGetsTheDiscoveryInstruction(t *testing.T) {
	e := setup(t)
	e.write("README.md", "# x\n")
	p := e.plan()
	if p.Discovery == "" || len(p.Gates) != 0 {
		t.Errorf("%+v", p)
	}
	has(t, p, "SKIP everything (nothing in this project states a check")
}

func TestACIStepWithACommandSubstitutionRunsWhole(t *testing.T) {
	e := setup(t)
	e.write("go.mod", "module x\n")
	e.tool(filepath.Join(e.path, "go"))
	e.write(".github/workflows/ci.yml", "jobs:\n  go:\n    steps:\n      - name: deadcode\n        run: |\n          out=\"$(go tool deadcode -test ./...)\"\n          [ -z \"$out\" ] || { echo \"$out\"; exit 1; }\n")
	e.check("go/deadcode", check("deadcode"))
	has(t, e.plan(), "RUN deadcode (.github/workflows/ci.yml: go/deadcode)", "  cmd: out=\"$(go tool deadcode -test ./...)\"", "(toolchain)", "  scope: only findings on lines changed")
}

func TestFindExecResolvesBothCommands(t *testing.T) {
	e := setup(t)
	e.tool("bin/shellcheck")
	e.write(".github/workflows/lint.yml", "jobs:\n  shell:\n    steps:\n      - name: shellcheck\n        run: find scripts -name '*.sh' -exec shellcheck -S warning {} +\n")
	e.check("shell/shellcheck", check("lint"))
	has(t, e.plan(), "RUN lint (.github/workflows/lint.yml: shell/shellcheck)", "find (system)", filepath.Join(e.root, "bin/shellcheck")+" (local)")
}

func TestAnAggregateWithABuildRunsWhole(t *testing.T) {
	e := setup(t)
	e.tool(filepath.Join(e.path, "npm"))
	e.write("package.json", `{"scripts":{"build":"tsc -b","test":"vitest run","ci":"npm run build && npm test"}}`)
	e.write(".github/workflows/ci.yml", "jobs:\n  ci:\n    steps:\n      - run: npm run ci\n")
	e.check("ci/1", map[string]any{"role": "check"})
	e.check("ci", map[string]any{"role": "check"})
	e.check("build", map[string]any{"role": "build"})
	e.check("test", check("test"))
	p := e.plan()
	has(t, p, "RUN test (package.json: ci)\n  cmd: npm run ci\n")
	lacks(t, p, "cmd: npm test\n", "cmd: npm run build")
}

func TestAMakefileAggregateOfChecksRunsAsItsParts(t *testing.T) {
	e := setup(t)
	e.write("Makefile", "check: lint test\n\nlint:\n\tgolangci-lint run\n\ntest:\n\tgo test ./...\n")
	e.write("go.mod", "module x\n")
	e.write(".github/workflows/ci.yml", "jobs:\n  ci:\n    steps:\n      - run: make check\n")
	e.check("ci/1", map[string]any{"role": "check"})
	e.check("check", map[string]any{"role": "check"})
	e.check("lint", check("lint"))
	e.check("test", check("test"))
	p := e.plan()
	has(t, p, "RUN lint (Makefile: lint)\n  cmd: make lint\n", "RUN test (Makefile: test)\n  cmd: make test\n")
	lacks(t, p, "cmd: make check")
}

func TestTurboAndNxRunTheirAffectedForms(t *testing.T) {
	e := setup(t)
	e.tool("node_modules/.bin/turbo")
	e.tool("node_modules/.bin/nx")
	e.write("package.json", `{"private":true}`)
	e.write("turbo.json", `{"tasks":{"lint":{}}}`)
	e.write("nx.json", `{"targetDefaults":{"test":{}}}`)
	e.check("lint", map[string]any{"role": "check", "kind": "lint", "affected_form": "turbo run lint --filter=...[{base}]"})
	e.check("test", map[string]any{"role": "check", "kind": "test", "affected_form": "nx affected -t test --base={base}"})
	has(t, e.plan(), "RUN lint (turbo.json: lint)\n  cmd: turbo run lint --filter=...[{base}]\n", "RUN test (nx.json: test)\n  cmd: nx affected -t test --base={base}\n")
}

func TestNpxWithNoLocalCopyIsASkip(t *testing.T) {
	e := setup(t)
	e.write("package.json", `{}`)
	e.tool(filepath.Join(e.path, "npx"))
	e.write(".github/workflows/ci.yml", "jobs:\n  lint:\n    steps:\n      - run: npx eslint .\n")
	e.check("lint/1", check("lint"))
	has(t, e.plan(), "SKIP lint (.github/workflows/ci.yml: lint/1) (fetches a package (npx eslint with no local copy))")
}

// A kind is covered per subproject: the root's JS tests leave the Go
// module's test to its evidence.
func TestEachSubprojectFillsItsOwnKinds(t *testing.T) {
	e := setup(t)
	e.tool(filepath.Join(e.path, "npm"))
	e.tool(filepath.Join(e.path, "go"))
	e.write("package.json", `{"scripts":{"test":"vitest run"}}`)
	e.write("svc/go.mod", "module svc\n")
	e.check("test", check("test"))
	e.proposals = []map[string]any{{"role": "check", "kind": "test", "command": "go test ./...", "dir": "svc", "evidence": "svc/go.mod", "mutates": false}}
	has(t, e.plan(), "RUN test (package.json: test)\n  cmd: npm run test\n", "RUN test (evidence svc/go.mod) [svc]\n  cmd: go test ./...\n  dir: svc\n")
}

// CI comes first: a kind it covers takes no task or evidence.
func TestCICoversItsKindsBeforeTasksAndEvidence(t *testing.T) {
	e := setup(t)
	e.tool("node_modules/.bin/eslint")
	e.tool(filepath.Join(e.path, "npm"))
	e.write("package.json", `{"scripts":{"lint":"eslint ."}}`)
	e.write(".github/workflows/ci.yml", "jobs:\n  lint:\n    steps:\n      - run: eslint src\n")
	e.check("lint/1", check("lint"))
	e.check("lint", check("lint"))
	e.proposals = []map[string]any{{"role": "check", "kind": "lint", "command": "eslint .", "dir": ".", "evidence": "package.json", "mutates": false}}
	p := e.plan()
	has(t, p, "RUN lint (.github/workflows/ci.yml: lint/1)")
	lacks(t, p, "package.json: lint", "evidence package.json")
}

func TestAGateSeenChangingFilesIsASkipUntilReset(t *testing.T) {
	e := setup(t)
	e.tool("bin/black")
	e.write(".github/workflows/ci.yml", "jobs:\n  fmt:\n    steps:\n      - run: black --check .\n")
	e.check("fmt/1", check("format-check"))
	p := e.plan()
	if err := AddMark(e.cache, e.root, p.Gates[0], Mark{Label: p.Gates[0].Label, Paths: []string{"a.py", "b.py"}}); err != nil {
		t.Fatal(err)
	}
	has(t, e.plan(), "SKIP format-check (.github/workflows/ci.yml: fmt/1) (changed a.py, b.py when it last ran; kit gates reset lets it run again)")
	if err := ResetMarks(e.cache, e.root); err != nil {
		t.Fatal(err)
	}
	has(t, e.plan(), "RUN format-check (.github/workflows/ci.yml: fmt/1)")
}

// disabled_checks turns a kind off everywhere, and one entry by its label.
func TestDisabledChecksTurnsAKindOrAnEntryOff(t *testing.T) {
	e := setup(t)
	e.tool(filepath.Join(e.path, "go"))
	e.write("go.mod", "module x\n")
	e.write(".github/workflows/ci.yml", "jobs:\n  go:\n    steps:\n      - run: go vet ./...\n      - run: go test ./...\n")
	e.check("go/1", check("lint"))
	e.check("go/2", check("test"))
	e.cfg.DisabledChecks = []string{"lint", "test (.github/workflows/ci.yml: go/2)"}
	p := e.plan()
	has(t, p, "SKIP lint (.github/workflows/ci.yml: go/1) (disabled_checks)", "SKIP test (.github/workflows/ci.yml: go/2) (disabled_checks)")
}

func TestAnUnclassifiedEntryIsASkipAndCounted(t *testing.T) {
	e := setup(t)
	e.write("package.json", `{"scripts":{"lint":"eslint ."}}`)
	p := e.plan()
	has(t, p, "SKIP package.json: lint (unclassified)")
	if p.Unclassified != 1 {
		t.Errorf("unclassified = %d", p.Unclassified)
	}
}
