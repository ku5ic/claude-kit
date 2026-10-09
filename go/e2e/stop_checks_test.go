package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// stopChecksEnv is one stop-checks test's fixture. kit.yml has one file
// check, fakelint, claiming .ts files under a .fakelintrc. Its binary is a
// stub in the repo's node_modules/.bin that records its cwd and arguments,
// and fails when $REPO/fail exists. Each test writes a transcript JSONL
// describing the turn the hook inspects.
type stopChecksEnv struct {
	t                            *testing.T
	k                            *Kit
	tmp, repo, transcript, calls string
}

const stopChecksKitYML = `file_checks:
  - name: fakelint
    ext: [ts]
    signal_files: [.fakelintrc]
    bin: fakelint
    cmd: "{bin} --check {files}"
`

func stopChecksSetup(t *testing.T) *stopChecksEnv {
	k := NewPlugin(t)
	tmp := t.TempDir()
	e := &stopChecksEnv{t: t, k: k, tmp: tmp,
		transcript: filepath.Join(tmp, "transcript.jsonl"), calls: filepath.Join(tmp, "calls")}
	Mkdir(t, filepath.Join(tmp, "repo/node_modules/.bin"))
	e.repo = k.Repo(filepath.Join(tmp, "repo"))
	k.KitYML(stopChecksKitYML)
	e.bin("fakelint", fmt.Sprintf("echo \"$PWD|$*\" >>%q\necho \"lint noise\"\n[[ ! -e %q ]]\n", e.calls, filepath.Join(e.repo, "fail")))
	Touch(t, e.path(".fakelintrc"))
	for _, f := range []string{"a.ts", "b.ts", "notes.md"} {
		Write(t, e.path(f), "x\n")
	}
	return e
}

func (e *stopChecksEnv) path(rel string) string { return filepath.Join(e.repo, rel) }

// bin writes a stub into the repo's node_modules/.bin.
func (e *stopChecksEnv) bin(name, body string) {
	Stub(e.t, e.path("node_modules/.bin/"+name), body)
}

// runner is a recording stub for a built-in tool in node_modules/.bin.
func (e *stopChecksEnv) runner(name string) {
	e.bin(name, fmt.Sprintf("echo \"$PWD|%s $*\" >>%q\n", name, e.calls))
}

func (e *stopChecksEnv) line(v any) {
	e.t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		e.t.Fatal(err)
	}
	f, err := os.OpenFile(e.transcript, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		e.t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(append(raw, '\n')); err != nil {
		e.t.Fatal(err)
	}
}

// turn is a real user prompt, then one edit per path.
func (e *stopChecksEnv) turn(tool string, paths ...string) {
	e.line(map[string]any{"type": "user", "message": map[string]any{"content": "do it"}})
	for _, p := range paths {
		e.line(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
			map[string]any{"type": "tool_use", "name": tool, "input": map[string]any{"file_path": p}},
		}}})
		e.line(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result"}}}})
	}
}

func (e *stopChecksEnv) stop(active bool) Result {
	e.t.Helper()
	return e.k.Hook("stop-checks", map[string]any{
		"hook_event_name": "Stop", "session_id": "s1", "cwd": e.repo,
		"transcript_path": e.transcript, "stop_hook_active": active,
	})
}

func (e *stopChecksEnv) callsIs(want string) {
	e.t.Helper()
	if got := strings.TrimRight(Read(e.t, e.calls), "\n"); got != want {
		e.t.Errorf("calls %q, want %q", got, want)
	}
}

func (e *stopChecksEnv) callsHasLine(want string) {
	e.t.Helper()
	if slices.Contains(Lines(Read(e.t, e.calls)), want) {
		return
	}
	e.t.Errorf("calls lack line %q:\n%s", want, Read(e.t, e.calls))
}

func (e *stopChecksEnv) noCalls() {
	e.t.Helper()
	if Exists(e.calls) {
		e.t.Errorf("a check ran:\n%s", Read(e.t, e.calls))
	}
}

// usePMs puts fake poetry and yarn first on PATH, standing in for the real
// ones. `poetry env info -p` prints $REPO/env; `yarn bin <name>` succeeds
// unless $REPO/nopm exists, and `yarn run <name> ...` records.
func (e *stopChecksEnv) usePMs() {
	pm := filepath.Join(e.tmp, "pm")
	Stub(e.t, filepath.Join(pm, "poetry"), fmt.Sprintf("[[ \"$*\" == \"env info -p\" ]] && echo %q\n", e.path("env")))
	Stub(e.t, filepath.Join(pm, "yarn"), fmt.Sprintf("case \"$1\" in\nbin) [[ ! -e %q ]] ;;\nrun) shift; echo \"$PWD|yarn run $*\" >>%q ;;\nesac\n",
		e.path("nopm"), e.calls))
	if err := os.Remove(e.path("node_modules/.bin/fakelint")); err != nil {
		e.t.Fatal(err)
	}
	e.k.PrependPath(pm)
}

// oneCheck is a kit.yml with fakelint plus the given keys.
func (e *stopChecksEnv) oneCheck(extra string) {
	e.k.KitYML("file_checks:\n  - name: fakelint\n    ext: [ts]\n    signal_files: [.fakelintrc]\n    bin: fakelint\n    cmd: \"{bin} {files}\"\n" + extra + "\n")
}

// fakelintOnPath moves fakelint out of node_modules/.bin onto PATH.
func (e *stopChecksEnv) fakelintOnPath() {
	dir := filepath.Join(e.tmp, "path")
	Mkdir(e.t, dir)
	if err := os.Rename(e.path("node_modules/.bin/fakelint"), filepath.Join(dir, "fakelint")); err != nil {
		e.t.Fatal(err)
	}
	e.k.PrependPath(dir)
}

// lintLines: a linter with a findings parser blocks only on lines the tree
// changed. The shellcheck stub reports a.sh lines 1 and 3, relative to
// where it runs.
func (e *stopChecksEnv) lintLines() {
	e.k.KitYML("disabled_file_checks: [fakelint]\n")
	Touch(e.t, e.path(".shellcheckrc"))
	e.bin("shellcheck", "echo \"a.sh:1:1: warning: old finding [SC1]\"\necho \"a.sh:3:1: warning: new finding [SC3]\"\nexit 1\n")
	Write(e.t, e.path("a.sh"), "one\ntwo\nthree\n")
	e.k.Git(e.repo, "add", "a.sh")
	e.k.Git(e.repo, "commit", "-q", "-m", "a.sh")
}

// plan writes a plan file in the repo's plans dir and returns its path.
func (e *stopChecksEnv) plan(body string) string {
	path := e.path(".claude/plans/plan-x.md")
	Write(e.t, path, body)
	return path
}

// review is a /code-review the model invoked.
func (e *stopChecksEnv) review() {
	e.line(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_use", "name": "Skill", "input": map[string]any{"skill": "code-review"}},
	}}})
}

func TestStopPlanDone(t *testing.T) {
	const done = "## Steps\n\n- [x] 1. a\n- [x] 2. b\n"
	t.Run("ticking the last step with no review since the last edit blocks once", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Edit", e.plan(done))
		r := e.stop(false)
		r.Want(t, 2)
		r.Has(t, "plan-x.md is done, but /code-review hasn't run since the last code edit")
		e.stop(true).Want(t, 0)
	})
	t.Run("a review after the last edit, even in an earlier turn, lets it through", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.review()
		e.turn("Edit", e.plan(done))
		e.stop(false).Want(t, 0)
	})
	t.Run("a typed /code-review counts", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.line(map[string]any{"type": "user", "message": map[string]any{"content": "/code-review high"}})
		e.turn("Edit", e.plan(done))
		e.stop(false).Want(t, 0)
	})
	t.Run("an edit after the review needs a new one", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.review()
		e.turn("Edit", e.path("a.ts"), e.plan(done))
		e.stop(false).Want(t, 2)
	})
	t.Run("a plan with an open step, or one ticked in an earlier turn, doesn't gate", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Edit", e.plan("## Steps\n\n- [x] 1. a\n- [ ] 2. b\n"))
		e.stop(false).Want(t, 0)
		e.plan(done)
		e.turn("Edit", e.path("b.ts"))
		e.stop(false).Want(t, 0)
	})
}

func TestStopChecks(t *testing.T) {
	t.Run("missing transcript runs nothing", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.stop(false).Want(t, 0)
		e.noCalls()
	})

	t.Run("stop_hook_active lets the stop through even when checks would fail", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		Touch(t, e.path("fail"))
		e.stop(true).Want(t, 0)
		e.noCalls()
	})

	t.Run("an edit runs the check on only the edited file, from the signal directory", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, "PASS fakelint (1 file)")
		// The hook name gets its own line, so every check starts one.
		r.Has(t, `stop-checks:\nPASS fakelint`)
		// And the binary that ran, with where it came from.
		r.Has(t, `PASS fakelint (1 file)\n  bin: `+e.path("node_modules/.bin/fakelint")+` (local)`)
		e.callsIs(e.repo + "|--check " + e.path("a.ts"))
	})

	t.Run("kit explain stop names the binary's source", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.k.Dir = e.repo
		r := e.k.Run("", "explain", "stop", e.path("a.ts"))
		r.Want(t, 0)
		r.Has(t, "  source   "+e.path("node_modules/.bin/fakelint")+" (local)")
	})

	t.Run("several edited files go to one call, each file once", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Write", e.path("a.ts"), e.path("b.ts"), e.path("a.ts"))
		r := e.stop(false)
		e.callsIs(e.repo + "|--check " + e.path("a.ts") + " " + e.path("b.ts"))
		r.Has(t, "PASS fakelint (2 files)")
	})

	t.Run("a failing check blocks with its output", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		Touch(t, e.path("fail"))
		r := e.stop(false)
		r.Want(t, 2)
		r.Has(t, "FAIL fakelint (1 file)", "lint noise", "checks: 0 passed, 1 failed, 0 skipped")
	})

	t.Run("a nested signal file groups its files and runs from there", func(t *testing.T) {
		e := stopChecksSetup(t)
		Touch(t, e.path("packages/a/.fakelintrc"))
		Write(t, e.path("packages/a/c.ts"), "x\n")
		e.turn("Edit", e.path("a.ts"), e.path("packages/a/c.ts"))
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, "PASS fakelint (1 file) [packages/a]")
		e.callsHasLine(e.path("packages/a") + "|--check " + e.path("packages/a/c.ts"))
		e.callsHasLine(e.repo + "|--check " + e.path("a.ts"))
	})

	t.Run("{dirs} passes each edited file's directory once, relative to the signal", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.k.KitYML(`file_checks:
  - name: fakevet
    ext: [ts]
    signal_files: [.fakelintrc]
    bin: fakelint
    cmd: "{bin} vet {dirs}"
`)
		Write(t, e.path("pkg/x/c.ts"), "x\n")
		Write(t, e.path("pkg/x/d.ts"), "x\n")
		e.turn("Edit", e.path("a.ts"), e.path("pkg/x/c.ts"), e.path("pkg/x/d.ts"))
		e.stop(false).Want(t, 0)
		e.callsIs(e.repo + "|vet . ./pkg/x")
	})

	t.Run("a word holding {files} repeats once per file, prefix kept", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.k.KitYML(`file_checks:
  - name: fakelint
    ext: [ts]
    signal_files: [.fakelintrc]
    bin: fakelint
    cmd: "{bin} :{files}"
`)
		e.turn("Edit", e.path("a.ts"), e.path("b.ts"))
		e.stop(false)
		e.callsIs(e.repo + "|:" + e.path("a.ts") + " :" + e.path("b.ts"))
	})

	t.Run("an & in a path survives the {files} substitution", func(t *testing.T) {
		e := stopChecksSetup(t)
		Write(t, e.path("R&D/a.ts"), "x\n")
		e.turn("Edit", e.path("R&D/a.ts"))
		e.stop(false)
		e.callsIs(e.repo + "|--check " + e.path("R&D/a.ts"))
	})

	// Built-in test runners: claimed by the declared dependency; with both
	// jest and vitest declared, the package.json test script picks.

	t.Run("a declared test runner runs the edited file's related tests", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.runner("vitest")
		Write(t, e.path("package.json"), `{"devDependencies":{"vitest":"^4"},"scripts":{"test":"vitest run"}}`+"\n")
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.callsHasLine(e.repo + "|vitest related --run --passWithNoTests " + e.path("a.ts"))
	})

	t.Run("with jest and vitest both declared, the test script picks the runner", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.runner("vitest")
		e.runner("jest")
		Write(t, e.path("package.json"), `{"devDependencies":{"vitest":"^4","jest":"^30"},"scripts":{"test":"NODE_ENV=test jest","storybook":"vitest"}}`+"\n")
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.callsHasLine(e.repo + "|jest --ci --findRelatedTests --passWithNoTests " + e.path("a.ts"))
		if strings.Contains(Read(t, e.calls), "|vitest ") {
			t.Errorf("vitest ran:\n%s", Read(t, e.calls))
		}
	})

	t.Run("the project's own test script carries its env and allow-listed flags", func(t *testing.T) {
		e := stopChecksSetup(t)
		// Derive reads the scripts through kit.yml's task provider.
		e.k.KitYML(stopChecksKitYML + `task_providers:
  - {name: package-scripts, stack: js, manifests: [package.json], extractor: json_keys, arg: .scripts, run: "{pm} run {task}"}
`)
		e.bin("jest", fmt.Sprintf("echo \"$PWD|NODE_ENV=$NODE_ENV jest $*\" >>%q\n", e.calls))
		Write(t, e.path("package.json"), `{"devDependencies":{"jest":"^30"},"scripts":{"test":"NODE_ENV=test jest --maxWorkers 2 --coverage src"}}`+"\n")
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.callsHasLine(e.repo + "|NODE_ENV=test jest --ci --findRelatedTests --passWithNoTests --maxWorkers 2 " + e.path("a.ts"))
	})

	t.Run("a malformed package.json drops the test runners, not the other checks", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.runner("jest")
		Write(t, e.path("package.json"), `{"devDependencies": {"jest": "^30",}}`+"\n")
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.callsIs(e.repo + "|--check " + e.path("a.ts"))
	})

	t.Run("signal_toml claims a file when the pyproject table exists, else not", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.k.KitYML(`file_checks:
  - name: fakelint
    ext: [ts]
    signal_toml: "pyproject.toml .tool.fakelint"
    bin: fakelint
    cmd: "{bin} {files}"
`)
		Write(t, e.path("pyproject.toml"), "[tool.other]\nx = 1\n")
		Write(t, e.path("backend/pyproject.toml"), "[tool.fakelint]\nfix = true\n")
		Write(t, e.path("backend/c.ts"), "x\n")
		e.turn("Edit", e.path("a.ts"), e.path("backend/c.ts"))
		e.stop(false).Want(t, 0)
		e.callsIs(e.path("backend") + "|" + e.path("backend/c.ts"))
	})

	t.Run("signal_toml walks past a nearer pyproject.toml without the table", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.k.KitYML(`file_checks:
  - name: fakelint
    ext: [ts]
    signal_toml: "pyproject.toml .tool.fakelint"
    bin: fakelint
    cmd: "{bin} {files}"
`)
		Write(t, e.path("pyproject.toml"), "[tool.fakelint]\nx = 1\n")
		Write(t, e.path("packages/foo/pyproject.toml"), "[project]\nname = \"foo\"\n")
		Write(t, e.path("packages/foo/c.ts"), "x\n")
		e.turn("Edit", e.path("packages/foo/c.ts"))
		e.stop(false).Want(t, 0)
		e.callsIs(e.repo + "|" + e.path("packages/foo/c.ts"))
	})

	t.Run("a poetry project runs the bin from the environment poetry reports", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.usePMs()
		Touch(t, e.path("poetry.lock"))
		Stub(t, e.path("env/bin/fakelint"), fmt.Sprintf("echo \"$PWD|venv $*\" >>%q\n", e.calls))
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.callsIs(e.repo + "|venv --check " + e.path("a.ts"))
	})

	t.Run("a Yarn PnP project wraps the bin in yarn run", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.usePMs()
		Touch(t, e.path(".pnp.cjs"))
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.callsIs(e.repo + "|yarn run fakelint --check " + e.path("a.ts"))
	})

	t.Run("a package manager without the bin falls through to a skip", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.usePMs()
		Touch(t, e.path(".pnp.cjs"), e.path("nopm"))
		e.turn("Edit", e.path("a.ts"))
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, "SKIP fakelint (1 file) (fakelint not installed)")
		e.noCalls()
	})

	t.Run("a project-local bin wins over a package-manager environment", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.usePMs()
		Touch(t, e.path(".pnp.cjs"))
		e.bin("fakelint", fmt.Sprintf("echo \"$PWD|local $*\" >>%q\n", e.calls))
		e.turn("Edit", e.path("a.ts"))
		e.stop(false)
		e.callsIs(e.repo + "|local --check " + e.path("a.ts"))
	})

	t.Run("disabled_file_checks in the overlay turns a check off", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.k.Overlay("disabled_file_checks: [fakelint]\n")
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.noCalls()
	})

	t.Run("golangci-lint runs from the Go module, only with a .golangci config at or above it", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.k.KitYML("disabled_file_checks: [go-vet]\ntool_resolution:\n  path_fallback: [golangci-lint]\n")
		Write(t, e.path("mod/go.mod"), "module example.com/m\n")
		Write(t, e.path("mod/pkg/c.go"), "package pkg\n")
		dir := filepath.Join(e.tmp, "path")
		Stub(t, filepath.Join(dir, "golangci-lint"), fmt.Sprintf("echo \"$PWD|golangci-lint $*\" >>%q\n", e.calls))
		e.k.PrependPath(dir)
		e.turn("Edit", e.path("mod/pkg/c.go"))
		e.stop(false)
		e.noCalls()
		Touch(t, e.path(".golangci.yml"))
		e.stop(false)
		e.callsIs(e.path("mod") + "|golangci-lint run --fix=false --max-issues-per-linter=0 --max-same-issues=0 ./pkg")
	})

	t.Run("a finding on a changed line blocks; one on an unchanged line doesn't", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.lintLines()
		Write(t, e.path("a.sh"), "one\ntwo\nTHREE\n")
		e.turn("Edit", e.path("a.sh"))
		r := e.stop(false)
		r.Want(t, 2)
		r.Has(t, "a.sh:3:1: warning: new finding [SC3]", "(1 more on unchanged lines don't block)")
		r.Lacks(t, "old finding")
	})

	t.Run("findings only on unchanged lines pass, and say so", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.lintLines()
		Write(t, e.path("a.sh"), "one\nTWO\nthree\n")
		e.turn("Edit", e.path("a.sh"))
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, "PASS shellcheck (1 file) (2 findings on unchanged lines)")
	})

	t.Run("every line of a file HEAD doesn't have counts as changed", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.lintLines()
		e.k.Git(e.repo, "reset", "-q", "--soft", "HEAD~1")
		e.turn("Edit", e.path("a.sh"))
		r := e.stop(false)
		r.Want(t, 2)
		r.Has(t, "old finding")
	})

	t.Run("a failure the parser can't read blocks with the output tail", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.lintLines()
		e.bin("shellcheck", "echo \"shellcheck: crashed\"\nexit 1\n")
		Write(t, e.path("a.sh"), "one\nTWO\nthree\n")
		e.turn("Edit", e.path("a.sh"))
		r := e.stop(false)
		r.Want(t, 2)
		r.Has(t, "shellcheck: crashed")
	})

	t.Run("exclude_toml drops files matching the project's exclude regexes", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.oneCheck(`    exclude_toml: "pyproject.toml .tool.fake.exclude"`)
		Write(t, e.path("pyproject.toml"), "[tool.fake]\nexclude = [\"^migrations/\", \"_gen\\\\.ts$\"]\n")
		Write(t, e.path("migrations/m.ts"), "x\n")
		Write(t, e.path("api_gen.ts"), "x\n")
		e.turn("Edit", e.path("migrations/m.ts"), e.path("api_gen.ts"), e.path("a.ts"))
		e.stop(false)
		e.callsIs(e.repo + "|" + e.path("a.ts"))
	})

	t.Run("exclude_toml takes a single-string exclude too", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.oneCheck(`    exclude_toml: "pyproject.toml .tool.fake.exclude"`)
		Write(t, e.path("pyproject.toml"), "[tool.fake]\nexclude = \"^a\\\\.ts$\"\n")
		e.turn("Edit", e.path("a.ts"))
		e.stop(false)
		e.noCalls()
	})

	t.Run("local_only skips a bin found only on PATH", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.oneCheck("    local_only: true")
		e.fakelintOnPath()
		e.turn("Edit", e.path("a.ts"))
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, "SKIP fakelint (1 file) (fakelint not in the project environment)")
		e.noCalls()
	})

	t.Run("a bin found only on PATH, undeclared and unpinned, is skipped with the reason", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.oneCheck("")
		e.fakelintOnPath()
		e.turn("Edit", e.path("a.ts"))
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, "SKIP fakelint (1 file) (fakelint only on PATH (", "add it to tool_resolution.path_fallback in ~/.claude/claude-kit.local.yml to allow")
		e.noCalls()
	})

	t.Run("a declared but uninstalled bin is skipped with the install command, not run from PATH", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.oneCheck("")
		Write(t, e.path("package.json"), `{"devDependencies":{"fakelint":"1.0.0"}}`+"\n")
		e.fakelintOnPath()
		e.turn("Edit", e.path("a.ts"))
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, "SKIP fakelint (1 file) (fakelint declared in package.json but not installed; run npm install)")
		e.noCalls()
	})

	t.Run("a pinned bin runs from the version manager's shims", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.oneCheck("tool_resolution:\n  pin_files: [.tool-versions]\n  manager_dirs: [\"$ASDF_DATA_DIR/shims\"]")
		Write(t, e.path(".tool-versions"), "fakelint 1.0.0\n")
		asdf := filepath.Join(e.tmp, "asdf")
		shim := filepath.Join(asdf, "shims/fakelint")
		Mkdir(t, filepath.Dir(shim))
		if err := os.Rename(e.path("node_modules/.bin/fakelint"), shim); err != nil {
			t.Fatal(err)
		}
		e.k.Setenv("ASDF_DATA_DIR", asdf)
		e.k.PrependPath(filepath.Dir(shim))
		e.turn("Edit", e.path("a.ts"))
		r := e.stop(false)
		r.Has(t, `PASS fakelint (1 file)\n  bin: `+shim+` (version manager)`)
	})

	t.Run("a bin in path_fallback runs from PATH", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.oneCheck("tool_resolution:\n  path_fallback: [fakelint]")
		e.fakelintOnPath()
		e.turn("Edit", e.path("a.ts"))
		e.stop(false)
		if !Exists(e.calls) {
			t.Error("fakelint on PATH did not run")
		}
	})

	t.Run("a gitignored file is not checked", func(t *testing.T) {
		e := stopChecksSetup(t)
		Write(t, e.path(".gitignore"), "scratch/\n")
		Write(t, e.path("scratch/tmp.ts"), "x\n")
		e.turn("Edit", e.path("scratch/tmp.ts"), e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.callsIs(e.repo + "|--check " + e.path("a.ts"))
	})

	t.Run("an edit outside the repo doesn't unignore the ones after it", func(t *testing.T) {
		e := stopChecksSetup(t)
		Write(t, e.path(".gitignore"), "scratch/\n")
		Write(t, e.path("scratch/tmp.ts"), "x\n")
		x := filepath.Join(e.tmp, "elsewhere/x.ts")
		Write(t, x, "x\n")
		e.turn("Edit", x, e.path("scratch/tmp.ts"), e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.callsIs(e.repo + "|--check " + e.path("a.ts"))
	})

	t.Run("a file check without a cmd is skipped, not a crash", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.k.KitYML("file_checks:\n  - name: fakelint\n    ext: [ts]\n    signal_files: [.fakelintrc]\n    bin: fakelint\n")
		e.turn("Edit", e.path("a.ts"))
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, "SKIP fakelint (1 file) (no cmd in kit.yml)")
	})

	t.Run("a block names why the skipped checks skipped", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.k.KitYML(stopChecksKitYML + "  - name: nocmd\n    ext: [ts]\n    signal_files: [.fakelintrc]\n    bin: fakelint\n")
		Touch(t, e.path("fail"))
		e.turn("Edit", e.path("a.ts"))
		r := e.stop(false)
		r.Want(t, 2)
		r.Has(t, "SKIP nocmd (1 file) (no cmd in kit.yml)")
	})

	t.Run("a file no check claims runs nothing", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Write", e.path("notes.md"))
		e.stop(false).Want(t, 0)
		e.noCalls()
	})

	t.Run("a file without the signal above it runs nothing", func(t *testing.T) {
		e := stopChecksSetup(t)
		if err := os.Remove(e.path(".fakelintrc")); err != nil {
			t.Fatal(err)
		}
		e.turn("Edit", e.path("a.ts"))
		e.stop(false)
		e.noCalls()
	})

	t.Run("a missing binary is skipped, not failed", func(t *testing.T) {
		e := stopChecksSetup(t)
		if err := os.Remove(e.path("node_modules/.bin/fakelint")); err != nil {
			t.Fatal(err)
		}
		e.turn("Edit", e.path("a.ts"))
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, "SKIP fakelint (1 file) (fakelint not installed)")
	})

	t.Run("a deleted file is not checked", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("gone.ts"))
		e.stop(false)
		e.noCalls()
	})

	t.Run("an edit outside the repo checks nothing", func(t *testing.T) {
		e := stopChecksSetup(t)
		x := filepath.Join(e.tmp, "elsewhere/x.ts")
		Write(t, x, "x\n")
		e.turn("Write", x)
		e.stop(false)
		e.noCalls()
	})

	t.Run("edits committed in the turn skip the checks", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		// The hook's HOME: a developer's global excludes must not hide node_modules.
		e.k.Git(e.repo, "add", "-A")
		e.k.Git(e.repo, "commit", "-q", "-m", "edit")
		e.stop(false).Want(t, 0)
		e.noCalls()
	})

	t.Run("a turn without edit tools skips the checks", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Read", e.path("a.ts"))
		e.stop(false)
		e.noCalls()
	})

	t.Run("an edit in an earlier turn does not count", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Read", e.path("a.ts"))
		e.stop(false)
		e.noCalls()
	})

	t.Run("a meta user entry does not start a new turn", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.line(map[string]any{"type": "user", "isMeta": true, "message": map[string]any{"content": "skill loaded"}})
		e.stop(false)
		if !Exists(e.calls) {
			t.Error("the edit's check did not run")
		}
	})

	t.Run("outside a git worktree exits clean", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.repo = filepath.Join(e.tmp, "plain")
		Write(t, e.path("a.ts"), "x\n")
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.noCalls()
	})

	t.Run("a hung check times out as a skip, its children killed with it", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.k.KitYML(stopChecksKitYML + "check_timeout: 1\n")
		pid := filepath.Join(e.tmp, "child.pid")
		e.bin("fakelint", fmt.Sprintf("sleep 30 &\necho $! >%q\nwait\n", pid))
		e.turn("Edit", e.path("a.ts"))
		start := time.Now()
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, "SKIP fakelint (1 file) (timed out after 1s)")
		if took := time.Since(start); took > 10*time.Second {
			t.Errorf("stop took %s", took)
		}
		child, err := strconv.Atoi(strings.TrimSpace(Read(t, pid)))
		if err != nil {
			t.Fatal(err)
		}
		if syscall.Kill(child, 0) == nil {
			syscall.Kill(child, syscall.SIGKILL)
			t.Error("the check's child process outlived the timeout")
		}
	})

	t.Run("checks run in parallel", func(t *testing.T) {
		e := stopChecksSetup(t)
		e.k.KitYML(stopChecksKitYML + "  - name: slowlint\n    ext: [ts]\n    signal_files: [.fakelintrc]\n    bin: slowlint\n    cmd: \"{bin} {files}\"\n")
		for _, name := range []string{"fakelint", "slowlint"} {
			e.bin(name, "sleep 2\n")
		}
		e.turn("Edit", e.path("a.ts"))
		start := time.Now()
		r := e.stop(false)
		r.Has(t, "PASS fakelint (1 file)", "PASS slowlint (1 file)")
		if took := time.Since(start); took > 3500*time.Millisecond {
			t.Errorf("two 2 s checks took %s", took)
		}
	})
}
