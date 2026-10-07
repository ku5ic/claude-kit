package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runChecksEnv is one run-checks test's fixture: a git repo with every file
// added before a run (subprojects come from tracked files), and a stub dir
// first on PATH so tests do not depend on real toolchains.
type runChecksEnv struct {
	t              *testing.T
	k              *Kit
	project, stubs string
}

func runChecksSetup(t *testing.T) *runChecksEnv { return runChecksSetupIn(t, "project") }

// runChecksSetupIn is runChecksSetup with the repo at <tmp>/<name>.
func runChecksSetupIn(t *testing.T, name string) *runChecksEnv {
	k := New(t)
	tmp := t.TempDir()
	e := &runChecksEnv{t: t, k: k, project: filepath.Join(tmp, name), stubs: filepath.Join(tmp, "stubs")}
	Mkdir(t, e.project)
	Mkdir(t, e.stubs)
	k.Git(e.project, "init", "-q", "-b", "main")
	k.PrependPath(e.stubs)
	k.Dir = e.project
	return e
}

func (e *runChecksEnv) write(rel, body string) { Write(e.t, filepath.Join(e.project, rel), body) }

// stub fakes a tool binary that records "$PWD $*" to <stubs>/<name>.calls.
func (e *runChecksEnv) stub(name string, code int) {
	Stub(e.t, filepath.Join(e.stubs, name),
		fmt.Sprintf("echo \"$PWD $*\" >>%q\nexit %d\n", filepath.Join(e.stubs, name+".calls"), code))
}

// calls is a stub's recorded calls, trailing newlines trimmed as $(cat) does.
func (e *runChecksEnv) calls(name string) string {
	return strings.TrimRight(Read(e.t, filepath.Join(e.stubs, name+".calls")), "\n")
}

func (e *runChecksEnv) called(name string) bool {
	return Exists(filepath.Join(e.stubs, name+".calls"))
}

func (e *runChecksEnv) run(args ...string) Result {
	e.t.Helper()
	e.k.Git(e.project, "add", "-A")
	return e.k.Run("", append([]string{"run-checks"}, args...)...)
}

func (e *runChecksEnv) callsEndWith(name, suffix string) {
	e.t.Helper()
	if c := e.calls(name); !strings.HasSuffix(c, suffix) {
		e.t.Errorf("%s calls %q, want suffix %q", name, c, suffix)
	}
}

func (e *runChecksEnv) callsEqual(name, want string) {
	e.t.Helper()
	if c := e.calls(name); c != want {
		e.t.Errorf("%s calls %q, want %q", name, c, want)
	}
}

// pnpm workspace with <name>.json holding config, a Python service, and a
// recording orchestrator binary <name> in node_modules/.bin.
func (e *runChecksEnv) orchestrated(name, config string) {
	e.write("package.json", `{"name":"root","private":true,"scripts":{"test":"turbo run test"}}`+"\n")
	e.write("pnpm-workspace.yaml", "packages:\n  - \"packages/*\"\n")
	e.write("pnpm-lock.yaml", "")
	e.write(name+".json", config+"\n")
	e.write("packages/a/package.json", `{"name":"a","scripts":{"test":"vitest","lint":"eslint ."}}`+"\n")
	e.write("services/api/pyproject.toml", "[tool.pdm.scripts]\ntest = \"pytest\"\n")
	Stub(e.t, filepath.Join(e.project, "node_modules/.bin", name),
		fmt.Sprintf("echo \"$*\" >>%q\n", filepath.Join(e.stubs, name+".calls")))
	e.write(".gitignore", "node_modules\n")
	e.stub("pnpm", 0)
	e.stub("pdm", 0)
}

// localFakefmt adds a js toolchain check on fakefmt, with a recording copy
// in <prefix>node_modules/.bin and another on PATH, both writing to
// fakefmt.calls.
func (e *runChecksEnv) localFakefmt(prefix string) {
	e.k.Overlay("toolchain_checks:\n  - {stack: js, name: fmt, cmd: \"{bin} --check .\", bin: [fakefmt]}\n")
	e.write(prefix+"package.json", "{}\n")
	e.write(".gitignore", "node_modules\n")
	calls := filepath.Join(e.stubs, "fakefmt.calls")
	Stub(e.t, filepath.Join(e.project, prefix+"node_modules/.bin/fakefmt"), fmt.Sprintf("echo \"local $*\" >>%q\n", calls))
	Stub(e.t, filepath.Join(e.stubs, "fakefmt"), fmt.Sprintf("echo \"path $*\" >>%q\n", calls))
}

// runChecksCase is a test that writes files, stubs binaries, runs
// run-checks with no arguments, and checks the output.
type runChecksCase struct {
	name  string
	files map[string]string
	stubs map[string]int
	has   []string
	lacks []string
	fails bool // status >= 1
	check func(e *runChecksEnv, r Result)
}

func TestRunChecks(t *testing.T) {
	cases := []runChecksCase{
		// JS/TS: declared package.json scripts run via the package manager.
		{name: "js: lint script runs via the package manager",
			files: map[string]string{"package.json": `{"scripts": {"lint": "eslint ."}}`},
			stubs: map[string]int{"npm": 0},
			has:   []string{"PASS js: lint (lint)"}},
		{name: "js: lint script failure is reflected in exit code",
			files: map[string]string{"package.json": `{"scripts": {"lint": "eslint ."}}`},
			stubs: map[string]int{"npm": 1},
			has:   []string{"FAIL js: lint (lint)"}, fails: true},
		{name: "js: every lint-like script runs, :fix variants never do",
			files: map[string]string{"package.json": `{"scripts": {"lint": "eslint .", "stylelint": "stylelint", "lint:css": "x", "lint:fix": "eslint --fix"}}`},
			stubs: map[string]int{"npm": 0},
			has:   []string{"PASS js: lint (lint)", "PASS js: lint (stylelint)", "PASS js: lint (lint:css)"},
			lacks: []string{"lint:fix"}},
		{name: "js: no lint script skips lint",
			files: map[string]string{"package.json": `{}`},
			has:   []string{"SKIP js: lint (no lint task)"}},
		{name: "js: eslint config without a lint script is still skipped (no direct linter run)",
			files: map[string]string{"package.json": `{}`, "eslint.config.js": "export default [];\n"},
			has:   []string{"SKIP js: lint (no lint task)"},
			lacks: []string{"eslint"}},
		{name: "js: typecheck script runs",
			files: map[string]string{"package.json": `{"scripts": {"typecheck": "tsc --noEmit"}}`},
			stubs: map[string]int{"npm": 0},
			has:   []string{"PASS js: typecheck (typecheck)"}},
		{name: "js: type-check (hyphenated) script runs",
			files: map[string]string{"package.json": `{"scripts": {"type-check": "tsc --noEmit"}}`},
			stubs: map[string]int{"npm": 0},
			has:   []string{"PASS js: typecheck (type-check)"}},
		{name: "js: tsconfig without a typecheck script is skipped (no direct tsc run)",
			files: map[string]string{"package.json": `{}`, "tsconfig.json": `{}`},
			has:   []string{"SKIP js: typecheck (no typecheck task)"}},
		{name: "js: format:check script runs",
			files: map[string]string{"package.json": `{"scripts": {"format:check": "prettier --check ."}}`},
			stubs: map[string]int{"npm": 0},
			has:   []string{"PASS js: format-check (format:check)"}},
		{name: "js: only a mutating format script skips format-check",
			files: map[string]string{"package.json": `{"scripts": {"format": "prettier --write ."}}`},
			stubs: map[string]int{"npm": 0},
			has:   []string{"SKIP js: format-check (no format-check task)"},
			check: func(e *runChecksEnv, r Result) {
				if e.called("npm") {
					e.t.Errorf("npm ran: %s", e.calls("npm"))
				}
			}},
		{name: "js: test script runs via the lockfile's package manager",
			files: map[string]string{"package.json": `{"scripts": {"test": "vitest run"}}`, "pnpm-lock.yaml": ""},
			stubs: map[string]int{"pnpm": 0},
			has:   []string{"PASS js: test (test)"},
			check: func(e *runChecksEnv, r Result) { e.callsEndWith("pnpm", "run test") }},
		{name: "js: no test script skips test",
			files: map[string]string{"package.json": `{}`},
			has:   []string{"SKIP js: test (no test task)"}},

		// Python: declared pdm/poe tasks and Makefile targets.
		{name: "py: pdm script runs via pdm run",
			files: map[string]string{"pyproject.toml": "[tool.pdm.scripts]\nlint = \"ruff check .\"\n"},
			stubs: map[string]int{"pdm": 0},
			has:   []string{"PASS python: lint (lint)"}},
		{name: "py: poe task runs via the poe runner",
			files: map[string]string{"pyproject.toml": "[tool.poe.tasks]\ntest = \"pytest\"\n"},
			stubs: map[string]int{"poe": 0},
			has:   []string{"PASS python: test (test)"}},
		{name: "py: a poetry service under a pnpm root runs poe through poetry",
			files: map[string]string{
				"package.json":       `{"name":"root","private":true}` + "\n",
				"pnpm-lock.yaml":     "",
				"svc/pyproject.toml": "[tool.poe.tasks]\nlint = \"ruff check\"\n",
				"svc/poetry.lock":    "",
			},
			stubs: map[string]int{"poetry": 0},
			has:   []string{"PASS python: lint (lint) [svc]"},
			check: func(e *runChecksEnv, r Result) { e.callsEndWith("poetry", "run poe lint") }},
		{name: "js: a subproject's own lockfile beats the root's",
			files: map[string]string{
				"package.json":     `{"name":"root","private":true}` + "\n",
				"pnpm-lock.yaml":   "",
				"web/package.json": `{"name":"web","scripts":{"test":"vitest"}}` + "\n",
				"web/yarn.lock":    "",
			},
			stubs: map[string]int{"yarn": 0},
			has:   []string{"PASS js: test (test) [web]"},
			check: func(e *runChecksEnv, r Result) { e.callsEndWith("yarn", "run test") }},
		{name: "py: poe task runs through poetry in a poetry project",
			files: map[string]string{"pyproject.toml": "[tool.poe.tasks]\ntest = \"pytest\"\n", "poetry.lock": ""},
			stubs: map[string]int{"poetry": 0},
			has:   []string{"PASS python: test (test)"},
			check: func(e *runChecksEnv, r Result) { e.callsEndWith("poetry", "run poe test") }},
		{name: "py: Makefile lint target runs via make",
			files: map[string]string{"pyproject.toml": "[tool.ruff]\n", "Makefile": "lint:\n\truff check .\n"},
			stubs: map[string]int{"make": 0},
			has:   []string{"PASS make: lint (lint)"}},
		{name: "py: Makefile typecheck and test targets run via make",
			files: map[string]string{"pyproject.toml": "[tool.pyright]\n", "Makefile": "typecheck:\n\tpyright apps/\ntest:\n\tpytest\n"},
			stubs: map[string]int{"make": 0},
			has:   []string{"PASS make: typecheck (typecheck)", "PASS make: test (test)"}},
		{name: "py: Makefile without a matching target still skips that check",
			files: map[string]string{"pyproject.toml": "[tool.ruff]\n", "Makefile": "build:\n\techo build\n"},
			has:   []string{"SKIP python: lint (no lint task)"}},
		{name: "py: requirements.txt plus a Makefile lint target runs via make",
			files: map[string]string{"requirements.txt": "django\n", "Makefile": "lint:\n\truff check .\n"},
			stubs: map[string]int{"make": 0},
			has:   []string{"PASS make: lint (lint)"}},
		{name: "py: no declared task skips the check (no direct tool run)",
			files: map[string]string{"pyproject.toml": "[tool.ruff]\n"},
			has:   []string{"SKIP python: lint (no lint task)"},
			lacks: []string{"ruff"}},
		{name: "py: requirements.txt alone declares no tasks, so nothing runs",
			files: map[string]string{"requirements.txt": "requests\n"},
			has:   []string{"checks: 0 passed, 0 failed, 0 skipped"},
			check: func(e *runChecksEnv, r Result) { r.Want(e.t, 0) }},

		// Ruby: declared rake tasks run via bundler.
		{name: "rb: rake lint task runs via bundler",
			files: map[string]string{"Gemfile": "source 'https://rubygems.org'\n", "Rakefile": "task :lint do\nend\n"},
			stubs: map[string]int{"bundle": 0},
			has:   []string{"PASS ruby: lint (lint)"}},
		{name: "rb: rubocop config without a Rakefile runs nothing (no direct rubocop run)",
			files: map[string]string{"Gemfile": "source 'https://rubygems.org'\n", ".rubocop.yml": "AllCops:\n"},
			has:   []string{"checks: 0 passed"},
			lacks: []string{"rubocop"}},

		// Toolchain checks: the stack's own subcommands.
		{name: "go: vet and test run",
			files: map[string]string{"go.mod": "module example.com/fixture\n\ngo 1.22\n"},
			stubs: map[string]int{"go": 0},
			has:   []string{"PASS go: vet", "PASS go: test"}},
		{name: "go: vet failure is reported and reflected in exit code",
			files: map[string]string{"go.mod": "module example.com/fixture\n\ngo 1.22\n"},
			stubs: map[string]int{"go": 1},
			has:   []string{"FAIL go: vet"}, fails: true},
		{name: "rust: cargo checks run where Cargo.toml is",
			files: map[string]string{"Cargo.toml": "[package]\nname = \"x\"\n"},
			stubs: map[string]int{"cargo": 0},
			has:   []string{"PASS rust: check", "PASS rust: clippy", "PASS rust: fmt", "PASS rust: test"}},
		{name: "opentofu: fmt runs, validate skips until init made .terraform/",
			files: map[string]string{".terraform.lock.hcl": ""},
			stubs: map[string]int{"tofu": 0},
			has:   []string{"PASS opentofu: fmt", "SKIP opentofu: validate (no .terraform/ yet)"},
			check: func(e *runChecksEnv, r Result) {
				found := false
				for _, l := range Lines(e.calls("tofu")) {
					found = found || strings.HasSuffix(l, " fmt -check -recursive")
				}
				if !found {
					e.t.Errorf("tofu calls lack fmt -check -recursive:\n%s", e.calls("tofu"))
				}
				Mkdir(e.t, filepath.Join(e.project, ".terraform"))
				e.run().Has(e.t, "PASS opentofu: validate")
			}},

		// Monorepo: every subproject, at any depth.
		{name: "monorepo: subdir package.json is discovered and labeled",
			files: map[string]string{"frontend/package.json": `{"scripts": {"lint": "eslint ."}}`},
			stubs: map[string]int{"npm": 0},
			has:   []string{"PASS js: lint (lint) [frontend]"}},
		{name: "monorepo: subdir pyproject task is discovered and labeled",
			files: map[string]string{"backend/pyproject.toml": "[tool.pdm.scripts]\nlint = \"ruff check .\"\n"},
			stubs: map[string]int{"pdm": 0},
			has:   []string{"PASS python: lint (lint) [backend]"}},
		{name: "monorepo: frontend and backend both run in one invocation",
			files: map[string]string{
				"frontend/package.json":  `{"scripts": {"test": "vitest run"}}`,
				"backend/pyproject.toml": "[tool.pdm.scripts]\ntest = \"pytest\"\n",
			},
			stubs: map[string]int{"npm": 0, "pdm": 0},
			has:   []string{"PASS js: test (test) [frontend]", "PASS python: test (test) [backend]"}},
		{name: "monorepo: a failing subdir check drives the overall exit code",
			files: map[string]string{"frontend/package.json": `{"scripts": {"lint": "eslint ."}}`},
			stubs: map[string]int{"npm": 1},
			has:   []string{"FAIL js: lint (lint) [frontend]"}, fails: true},
		{name: "monorepo: root and subdir manifests both run",
			files: map[string]string{
				"package.json":          `{"scripts": {"lint": "eslint ."}}`,
				"frontend/package.json": `{"scripts": {"lint": "eslint ."}}`,
			},
			stubs: map[string]int{"npm": 0},
			has:   []string{"PASS js: lint (lint)", "PASS js: lint (lint) [frontend]"}},
		{name: "monorepo: a nested pnpm workspace package's test runs, in its own dir",
			files: map[string]string{
				"package.json":            `{"name":"root","private":true}` + "\n",
				"pnpm-workspace.yaml":     "packages:\n  - \"packages/*\"\n",
				"pnpm-lock.yaml":          "",
				"packages/a/package.json": `{"name":"a","scripts":{"test":"vitest run"}}` + "\n",
			},
			stubs: map[string]int{"pnpm": 0},
			has:   []string{"PASS js: test (test) [packages/a]"},
			check: func(e *runChecksEnv, r Result) { e.callsEndWith("pnpm", "/packages/a run test") }},

		// summary line
		{name: "summary line reports pass/fail/skip counts",
			files: map[string]string{"package.json": `{}`},
			check: func(e *runChecksEnv, r Result) {
				i := strings.Index(r.Output, "checks:")
				rest := r.Output[max(i, 0):]
				ok := i >= 0
				for _, w := range []string{"passed", "failed", "skipped"} {
					j := strings.Index(rest, w)
					ok = ok && j >= 0
					if j >= 0 {
						rest = rest[j+len(w):]
					}
				}
				if !ok {
					e.t.Errorf("no checks:*passed*failed*skipped summary:\n%s", r.Output)
				}
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := runChecksSetup(t)
			for rel, body := range c.files {
				e.write(rel, body)
			}
			for name, code := range c.stubs {
				e.stub(name, code)
			}
			r := e.run()
			r.Has(t, c.has...)
			r.Lacks(t, c.lacks...)
			if c.fails && r.Status < 1 {
				t.Errorf("status %d, want >= 1; output:\n%s", r.Status, r.Output)
			}
			if c.check != nil {
				c.check(e, r)
			}
		})
	}

	t.Run("--only checks just the named subprojects", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		e.write("packages/a/package.json", `{"scripts":{"test":"vitest"}}`+"\n")
		e.write("services/api/pyproject.toml", "[tool.pdm.scripts]\ntest = \"pytest\"\n")
		e.stub("npm", 0)
		e.stub("pdm", 0)
		r := e.run("--only", "services/api")
		r.Want(t, 0)
		r.Has(t, "PASS python: test (test) [services/api]")
		r.Lacks(t, "packages/a", "js: lint")
	})

	t.Run("a bad argument is a usage error and runs nothing", func(t *testing.T) {
		for _, args := range [][]string{{"--plann"}, {"--only"}, {"--only", "services/nope"}, {"--plan", "extra"}} {
			e := runChecksSetup(t)
			e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
			e.stub("npm", 0)
			r := e.run(args...)
			r.Want(t, 2)
			r.Has(t, "kit run-checks: ")
			if e.called("npm") {
				t.Errorf("%v ran npm: %s", args, e.calls("npm"))
			}
		}
	})

	t.Run("a flag after --only says where it belongs", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		r := e.run("--only", ".", "--plan")
		r.Want(t, 2)
		r.Has(t, "kit run-checks: --plan must come before --only")
	})

	t.Run("pnpm runs a task with its install-before-run turned off", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		e.write("pnpm-lock.yaml", "")
		Stub(t, filepath.Join(e.stubs, "pnpm"),
			fmt.Sprintf("echo \"$pnpm_config_verify_deps_before_run $*\" >>%q\n", filepath.Join(e.stubs, "pnpm.calls")))
		e.run().Want(t, 0)
		e.callsEqual("pnpm", "false run lint")
	})

	// Orchestrators: turbo or nx run JS checks once, for affected packages.
	t.Run("turbo: an edit in packages/a runs turbo once per check, no per-package task", func(t *testing.T) {
		e := runChecksSetup(t)
		e.orchestrated("turbo", `{"tasks":{"test":{},"lint":{},"build":{}}}`)
		r := e.run("--only", "packages/a")
		r.Want(t, 0)
		r.Has(t, "PASS js: test (turbo affected: test)", "PASS js: lint (turbo affected: lint)")
		e.callsEqual("turbo", "run lint --filter=...[HEAD]\nrun test --filter=...[HEAD]")
		if e.called("pnpm") {
			t.Errorf("pnpm ran: %s", e.calls("pnpm"))
		}
		r.Lacks(t, "(test) [packages/a]", "(lint) [packages/a]")
	})
	t.Run("turbo: a 1.x pipeline key is read too", func(t *testing.T) {
		e := runChecksSetup(t)
		e.orchestrated("turbo", `{"pipeline":{"test":{}}}`)
		e.run("--only", "packages/a").Has(t, "PASS js: test (turbo affected: test)")
	})
	t.Run("turbo: checks turbo declares no task for still run per package", func(t *testing.T) {
		e := runChecksSetup(t)
		e.orchestrated("turbo", `{"tasks":{"test":{}}}`)
		e.run("--only", "packages/a").Has(t, "PASS js: test (turbo affected: test)", "PASS js: lint (lint) [packages/a]")
	})
	t.Run("turbo: a Python-only scope never runs turbo", func(t *testing.T) {
		e := runChecksSetup(t)
		e.orchestrated("turbo", `{"tasks":{"test":{}}}`)
		e.run("--only", "services/api").Has(t, "PASS python: test (test) [services/api]")
		if e.called("turbo") {
			t.Errorf("turbo ran: %s", e.calls("turbo"))
		}
	})
	t.Run("turbo: without its binary in node_modules/.bin, packages run their own tasks", func(t *testing.T) {
		e := runChecksSetup(t)
		e.orchestrated("turbo", `{"tasks":{"test":{}}}`)
		if err := os.Remove(filepath.Join(e.project, "node_modules/.bin/turbo")); err != nil {
			t.Fatal(err)
		}
		r := e.run("--only", "packages/a")
		r.Has(t, "PASS js: test (test) [packages/a]")
		r.Lacks(t, "turbo affected")
	})
	t.Run("nx: targetDefaults run through nx affected --uncommitted", func(t *testing.T) {
		e := runChecksSetup(t)
		e.orchestrated("nx", `{"targetDefaults":{"test":{},"typecheck":{}}}`)
		r := e.run("--only", "packages/a")
		r.Has(t, "PASS js: typecheck (nx affected: typecheck)", "PASS js: test (nx affected: test)")
		e.callsEqual("nx", "affected -t typecheck --uncommitted\naffected -t test --uncommitted")
	})

	// kit.yml overlay: a new provider is data, not code.
	t.Run("an overlay-defined composer provider runs its test script", func(t *testing.T) {
		e := runChecksSetup(t)
		e.k.Overlay(`task_providers:
  - name: composer
    stack: php
    manifests: [composer.json]
    extractor: json_keys
    arg: .scripts
    run: "composer run {task}"
`)
		e.write("composer.json", `{"scripts": {"test": "phpunit"}}`)
		e.stub("composer", 0)
		e.run().Has(t, "PASS php: test (test)")
		e.callsEndWith("composer", "run test")
	})

	// Toolchain checks resolve {bin} like file checks: the project's copy first.
	t.Run("a toolchain check runs the project-local bin before a PATH copy", func(t *testing.T) {
		e := runChecksSetup(t)
		e.localFakefmt("")
		local := Physical(t, filepath.Join(e.project, "node_modules/.bin/fakefmt"))
		e.run().Has(t, "PASS js: fmt\n  bin: "+local+" (local)\n")
		e.callsEqual("fakefmt", "local --check .")
	})
	t.Run("a failing toolchain check names its binary before the output", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n")
		Stub(t, filepath.Join(e.stubs, "go"), "echo vet output\nexit 1\n")
		e.run().Has(t, "FAIL go: vet ("+filepath.Join(e.stubs, "go")+" vet ./...)\n  bin: "+filepath.Join(e.stubs, "go")+" (PATH)\nvet output\n")
	})
	t.Run("go test skips an end-to-end module, not a lookalike name", func(t *testing.T) {
		e := runChecksSetup(t)
		for _, dir := range []string{"", "e2e/end2end/", "services/delivery/"} {
			e.write(dir+"go.mod", "module example.com/x\n")
		}
		e.stub("go", 0)
		r := e.run("--plan")
		r.Want(t, 0)
		r.Has(t, "SKIP go: test [e2e/end2end] (e2e looks like a test suite to leave out (exclude_dirs))",
			"RUN go: vet [e2e/end2end]", "RUN go: test [services/delivery]", "RUN go: test\n")
	})
	t.Run("a resolved bin path with a space stays one word", func(t *testing.T) {
		e := runChecksSetup(t)
		e.localFakefmt("my app/")
		e.run().Has(t, "PASS js: fmt [my app]")
		e.callsEqual("fakefmt", "local --check .")
	})
	t.Run("turbo: a repo path with a space still orchestrates", func(t *testing.T) {
		e := runChecksSetupIn(t, "my project")
		e.orchestrated("turbo", `{"tasks":{"test":{}}}`)
		e.run("--only", "packages/a").Has(t, "PASS js: test (turbo affected: test)")
		e.callsEqual("turbo", "run test --filter=...[HEAD]")
	})

	// Task bodies: a task whose name matches no slot counts by what it runs.
	t.Run("an odd-named task whose body is one gate fills that slot, run as itself", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"verify-style":"cross-env CI=1 eslint .","types":"tsc --noEmit"}}`+"\n")
		e.stub("npm", 0)
		r := e.run()
		r.Has(t, "PASS js: lint (verify-style)", "PASS js: typecheck (types)")
		e.callsEqual("npm", e.phys(".")+" run types\n"+e.phys(".")+" run verify-style")
	})
	t.Run("a body that isn't one readable gate never runs", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"fixup":"eslint --fix .","piped":"eslint . | tee out","build":"tsc -b"}}`+"\n")
		e.stub("npm", 0)
		r := e.run()
		r.Has(t, "SKIP js: lint (no lint task)", "SKIP js: typecheck (no typecheck task)")
		if e.called("npm") {
			t.Errorf("ran: %s", e.calls("npm"))
		}
	})
	t.Run("a slot-named task whose body fixes or watches is skipped, and covers nothing", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"check":"npm run lint && npm run unit","lint":"eslint . --fix","test":"jest --watch","unit":"vitest run"}}`+"\n")
		e.stub("npm", 0)
		r := e.run()
		r.Has(t, "SKIP js: lint (lint) (runs `eslint --fix`, which a gate never runs)",
			"SKIP js: test (test) (runs `jest --watch`, which a gate never runs)", "PASS js: test (unit)")
		e.callsEqual("npm", e.phys(".")+" run unit")
	})
	t.Run("a slot's exclude globs hold for body-classified tasks too", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"unit-watch":"vitest"}}`+"\n")
		e.stub("npm", 0)
		e.run().Has(t, "SKIP js: test (no test task)")
	})
	t.Run("a Make target whose recipe is one gate fills that slot", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n")
		e.write("Makefile", "golint:\n\t@golangci-lint run ./...\n")
		e.stub("make", 0)
		e.stub("go", 0)
		e.run().Has(t, "PASS make: lint (golint)")
	})

	// Cargo aliases are tasks, read as the cargo command they expand to.
	t.Run("rust: a cargo alias fills its slot by name or by what it runs", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("Cargo.toml", "[package]\nname = \"x\"\n")
		e.write(".cargo/config.toml", "[alias]\nlint = \"clippy --all-targets -- -D warnings\"\nck = [\"fmt\", \"--check\"]\nxtask = \"run --package xtask --\"\n")
		e.stub("cargo", 0)
		r := e.run()
		r.Has(t, "PASS rust: lint (lint)", "PASS rust: format-check (ck)")
		r.Lacks(t, "(xtask)")
	})
	t.Run("rust: no .cargo/config.toml, no alias tasks", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("Cargo.toml", "[package]\nname = \"x\"\n")
		e.stub("cargo", 0)
		r := e.run()
		r.Lacks(t, "rust: lint (", "rust: format-check (")
	})

	t.Run("--plan lists every check with its command and runs none", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		e.write("go.mod", "module example.com/x\n")
		e.stub("npm", 0)
		e.stub("go", 0)
		r := e.run("--plan")
		r.Want(t, 0)
		r.Has(t,
			"RUN js: lint (lint)\n  cmd: npm run lint\n",
			"SKIP js: test (no test task)\n",
			"RUN go: vet\n  cmd: "+filepath.Join(e.stubs, "go")+" vet ./...\n  bin: "+filepath.Join(e.stubs, "go")+" (PATH)\n")
		r.Lacks(t, "checks:")
		if e.called("npm") || e.called("go") {
			t.Errorf("--plan ran something: npm %q go %q", e.calls("npm"), e.calls("go"))
		}
	})
	t.Run("--plan takes --only too", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		e.write("services/api/pyproject.toml", "[tool.pdm.scripts]\ntest = \"pytest\"\n")
		r := e.run("--plan", "--only", "services/api")
		r.Has(t, "RUN python: test (test) [services/api]")
		r.Lacks(t, "js: lint")
	})

	// Overlay control: turning checks off, and updating a default by key.
	t.Run("disabled_checks turns a slot off, and one task by its label", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"typecheck":"tsc","lint":"eslint .","lint:css":"stylelint"}}`+"\n")
		e.stub("npm", 0)
		e.k.Overlay("disabled_checks: [typecheck, \"lint (lint:css)\"]\n")
		r := e.run()
		r.Has(t, "SKIP js: typecheck (typecheck) (disabled_checks)", "PASS js: lint (lint)", "SKIP js: lint (lint:css) (disabled_checks)")
		e.callsEqual("npm", e.phys(".")+" run lint")
	})
	t.Run("disabled_checks on a slot with no task says so", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		e.k.Overlay("disabled_checks: [test]\n")
		e.run().Has(t, "SKIP js: test (disabled_checks)")
	})
	t.Run("disabled_toolchain_checks skips one by <stack>:<name>", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n")
		e.stub("go", 0)
		e.k.Overlay("disabled_toolchain_checks: [\"go:vet\"]\n")
		r := e.run()
		r.Has(t, "SKIP go: vet (disabled_toolchain_checks)", "PASS go: test")
		e.callsEqual("go", e.phys(".")+" test ./...")
	})
	t.Run("disabled_task_providers stops a provider's tasks being read", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		e.write("Makefile", "lint:\n\techo lint\n")
		e.stub("npm", 0)
		e.stub("make", 0)
		e.k.Overlay("disabled_task_providers: [make]\n")
		r := e.run()
		r.Has(t, "PASS js: lint (lint)")
		r.Lacks(t, "make: lint")
	})
	t.Run("an overlay toolchain check with a default's key replaces it, not runs beside it", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n")
		e.stub("go", 0)
		e.k.Overlay("toolchain_checks:\n  - {stack: go, name: test, cmd: \"{bin} test -race ./...\"}\n")
		e.run()
		e.callsEqual("go", e.phys(".")+" vet ./...\n"+e.phys(".")+" test -race ./...")
	})
	t.Run("an overlay check with a default's name updates its globs", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"verify-types":"tsc"}}`+"\n")
		e.stub("npm", 0)
		e.k.Overlay("checks:\n  - {name: typecheck, tasks: [verify-types]}\n")
		r := e.run()
		r.Has(t, "PASS js: typecheck (verify-types)")
		if n := strings.Count(r.Output, "js: typecheck"); n != 1 {
			t.Errorf("typecheck reported %d times:\n%s", n, r.Output)
		}
	})

	// A copy of the launcher beside the freshly built binary, so its
	// relative-path resolution is tested against this tree's code.
	t.Run("a relative launcher path from a subdirectory still finds the binary", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/fixture\n\ngo 1.22\n")
		Mkdir(t, filepath.Join(e.project, "sub"))
		e.stub("go", 0)
		e.k.Git(e.project, "add", "-A")
		bin := filepath.Join(Tree(t), "bin")
		// ../ up to /, then the launcher's absolute path: relative from sub/.
		sub := Physical(t, filepath.Join(e.project, "sub"))
		up := strings.Repeat("../", strings.Count(sub, "/"))
		launcher := filepath.Join(bin, "kit")
		e.k.Dir = sub
		e.k.Shell("", up+strings.TrimPrefix(launcher, "/")+" run-checks").Has(t, "PASS go: vet")
	})
}
