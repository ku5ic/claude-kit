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

func runChecksSetup(t *testing.T) *runChecksEnv {
	k := New(t)
	tmp := t.TempDir()
	e := &runChecksEnv{t: t, k: k, project: filepath.Join(tmp, "project"), stubs: filepath.Join(tmp, "stubs")}
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
