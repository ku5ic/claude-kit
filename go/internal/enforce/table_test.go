package enforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

// planCase is one project through the full gate's plan: its files, the
// binaries it has (a "PATH/" prefix puts one on PATH, else it's relative to
// the root), the verdicts by entry name, and gap-fill's facts.
type planCase struct {
	name      string
	files     map[string]string
	tools     []string
	verdicts  map[string]map[string]any
	managers  []map[string]any
	proposals []map[string]any
	only      []string
	cfg       func(*config.Config)
	has       []string
	lacks     []string
}

func (c planCase) run(t *testing.T) {
	e := setup(t)
	for name, body := range c.files {
		e.write(name, body)
	}
	for _, tool := range c.tools {
		if rest, ok := strings.CutPrefix(tool, "PATH/"); ok {
			tool = filepath.Join(e.path, rest)
		}
		e.tool(tool)
	}
	for name, v := range c.verdicts {
		e.check(name, v)
	}
	e.managers, e.proposals, e.only = c.managers, c.proposals, c.only
	if c.cfg != nil {
		c.cfg(e.cfg)
	}
	p := e.plan()
	has(t, p, c.has...)
	lacks(t, p, c.lacks...)
}

// manager is a lockfile directory's facts, its rivals as gap-fill states
// them.
func manager(dir, cites, name string) map[string]any {
	rivals := map[string][]string{"npm": {"pnpm", "yarn"}, "pnpm": {"npm", "yarn"}, "yarn": {"npm", "pnpm"}, "pdm": {"poetry", "uv"}, "poetry": {"pdm", "uv"}, "uv": {"pdm", "poetry"}}
	return map[string]any{"dir": dir, "cites": cites, "manager": name, "rivals": rivals[name]}
}

func proposal(kind, command, dir, evidence string) map[string]any {
	return map[string]any{"role": "check", "kind": kind, "command": command, "dir": dir, "evidence": evidence, "mutates": false}
}

// The projects the old engine's run-checks suite planned, each through the
// new plan: a task runs by its runner, through the nearest verified
// package manager, in its own subproject.
func TestTaskRunnerPlans(t *testing.T) {
	for _, c := range []planCase{
		{name: "a lint script runs through npm when no lockfile names a manager",
			files:    map[string]string{"package.json": `{"scripts":{"lint":"eslint ."}}`},
			tools:    []string{"PATH/npm"},
			verdicts: map[string]map[string]any{"lint": check("lint")},
			has:      []string{"RUN lint (package.json: lint)\n  cmd: npm run lint\n"}},
		{name: "every lint script runs, a fixer never does",
			files:    map[string]string{"package.json": `{"scripts":{"lint":"eslint .","stylelint":"stylelint","lint:css":"x","lint:fix":"eslint --fix"}}`},
			tools:    []string{"PATH/npm"},
			verdicts: map[string]map[string]any{"lint": check("lint"), "stylelint": check("lint"), "lint:css": check("lint"), "lint:fix": fixer()},
			has:      []string{"RUN lint (package.json: lint)\n", "RUN lint (package.json: stylelint)\n", "RUN lint (package.json: lint:css)\n"},
			lacks:    []string{"lint:fix"}},
		{name: "typecheck and format-check scripts run by their verdicts, whatever their names",
			files:    map[string]string{"package.json": `{"scripts":{"typecheck":"tsc --noEmit","type-check":"tsc -b","format:check":"prettier --check .","format":"prettier --write ."}}`},
			tools:    []string{"PATH/npm"},
			verdicts: map[string]map[string]any{"typecheck": check("typecheck"), "type-check": check("typecheck"), "format:check": check("format-check"), "format": fixer()},
			has:      []string{"RUN typecheck (package.json: typecheck)\n", "RUN typecheck (package.json: type-check)\n", "RUN format-check (package.json: format:check)\n  cmd: npm run format:check\n"},
			lacks:    []string{"package.json: format)"}},
		{name: "a kind nothing states is one line, and a tool's config alone runs nothing",
			files: map[string]string{"package.json": `{}`, "eslint.config.js": "export default [];\n", "tsconfig.json": "{}", "Gemfile": "source 'https://rubygems.org'\n", ".rubocop.yml": "AllCops:\n"},
			tools: []string{"PATH/npm", "node_modules/.bin/eslint", "node_modules/.bin/tsc"},
			has:   []string{"SKIP lint, typecheck, format-check, test (nothing enforces it)\n"},
			lacks: []string{"RUN", "eslint", "rubocop"}},
		{name: "a tool's config states a check once gap-fill proposes it",
			files: map[string]string{"package.json": `{}`, "eslint.config.js": "export default [];\n", "tsconfig.json": "{}", "pyproject.toml": "[tool.ruff]\n"},
			tools: []string{"node_modules/.bin/eslint", "node_modules/.bin/tsc", ".venv/bin/ruff"},
			proposals: []map[string]any{
				proposal("lint", "eslint .", ".", "eslint.config.js"),
				proposal("typecheck", "tsc --noEmit", ".", "tsconfig.json"),
			},
			has: []string{"RUN lint (evidence eslint.config.js)\n  cmd: eslint .\n", "RUN typecheck (evidence tsconfig.json)\n  cmd: tsc --noEmit\n"}},
		{name: "a test script runs through the manager the lockfile names",
			files:    map[string]string{"package.json": `{"scripts":{"test":"vitest run"}}`, "pnpm-lock.yaml": ""},
			tools:    []string{"PATH/pnpm"},
			verdicts: map[string]map[string]any{"test": check("test")},
			managers: []map[string]any{manager(".", "pnpm-lock.yaml", "pnpm")},
			has:      []string{"RUN test (package.json: test)\n  cmd: pnpm run test\n"}},
		{name: "a subproject's own lockfile beats the root's",
			files: map[string]string{
				"package.json": `{"name":"root","private":true}`, "pnpm-lock.yaml": "",
				"web/package.json": `{"name":"web","scripts":{"test":"vitest"}}`, "web/yarn.lock": "",
			},
			tools:    []string{"PATH/pnpm", "PATH/yarn"},
			verdicts: map[string]map[string]any{"test": check("test")},
			managers: []map[string]any{manager(".", "pnpm-lock.yaml", "pnpm"), manager("web", "yarn.lock", "yarn")},
			has:      []string{"RUN test (web/package.json: test) [web]\n  cmd: yarn run test\n  dir: web\n"}},
		{name: "a script beside a Python lockfile runs through the JS manager, whichever fact comes first",
			files:    map[string]string{"package.json": `{"scripts":{"lint":"eslint ."}}`, "pnpm-lock.yaml": "", "pyproject.toml": "[project]\nname = \"x\"\n", "poetry.lock": ""},
			tools:    []string{"PATH/pnpm"},
			verdicts: map[string]map[string]any{"lint": check("lint")},
			managers: []map[string]any{manager(".", "poetry.lock", "poetry"), manager(".", "pnpm-lock.yaml", "pnpm")},
			has:      []string{"RUN lint (package.json: lint)\n  cmd: pnpm run lint\n"}},
		{name: "a package.json with no lockfile under a uv root runs through npm",
			files:    map[string]string{"pyproject.toml": "[project]\nname = \"x\"\n", "uv.lock": "", "docs/package.json": `{"scripts":{"lint":"markdownlint ."}}`},
			tools:    []string{"PATH/npm"},
			verdicts: map[string]map[string]any{"lint": check("lint")},
			managers: []map[string]any{manager(".", "uv.lock", "uv")},
			has:      []string{"cmd: npm run lint\n  dir: docs\n"}},
		{name: "pdm and poe tasks run through their runners",
			files:    map[string]string{"pyproject.toml": "[tool.pdm.scripts]\nlint = \"ruff check .\"\n[tool.poe.tasks]\ntest = \"pytest\"\n", "pdm.lock": "", ".gitignore": ".venv\n"},
			tools:    []string{"PATH/pdm", ".venv/bin/poe"},
			verdicts: map[string]map[string]any{"lint": check("lint"), "test": check("test")},
			managers: []map[string]any{manager(".", "pdm.lock", "pdm")},
			has:      []string{"RUN lint (pyproject.toml: lint)\n  cmd: pdm run lint\n", "RUN test (pyproject.toml: test)\n  cmd: poe test\n"}},
		{name: "a poetry service under a pnpm root runs poe through poetry",
			files: map[string]string{
				"package.json": `{"name":"root","private":true}`, "pnpm-lock.yaml": "",
				"svc/pyproject.toml": "[tool.poe.tasks]\nlint = \"ruff check\"\n", "svc/poetry.lock": "",
			},
			tools:    []string{"PATH/pnpm", "PATH/poetry"},
			verdicts: map[string]map[string]any{"lint": check("lint")},
			managers: []map[string]any{manager(".", "pnpm-lock.yaml", "pnpm"), manager("svc", "poetry.lock", "poetry")},
			has:      []string{"RUN lint (svc/pyproject.toml: lint) [svc]\n  cmd: poetry run poe lint\n  dir: svc\n"}},
		{name: "Makefile targets run through make, beside a requirements.txt",
			files:    map[string]string{"requirements.txt": "django\n", "Makefile": "lint:\n\truff check .\ntypecheck:\n\tpyright apps/\ntest:\n\tpytest\nbuild:\n\techo build\n"},
			verdicts: map[string]map[string]any{"lint": check("lint"), "typecheck": check("typecheck"), "test": check("test"), "build": {"role": "build"}},
			has:      []string{"RUN lint (Makefile: lint)\n  cmd: make lint\n", "RUN typecheck (Makefile: typecheck)\n  cmd: make typecheck\n", "RUN test (Makefile: test)\n  cmd: make test\n"},
			lacks:    []string{"make build"}},
		{name: "a requirements.txt alone states nothing to check",
			files: map[string]string{"requirements.txt": "requests\n"},
			has:   []string{"SKIP everything (nothing in this project states a check"}},
		{name: "a rake task runs through bundler",
			files:    map[string]string{"Gemfile": "source 'https://rubygems.org'\n", "Rakefile": "task :lint do\nend\n"},
			tools:    []string{"PATH/bundle"},
			verdicts: map[string]map[string]any{"lint": check("lint")},
			has:      []string{"RUN lint (Rakefile: lint)\n  cmd: bundle exec rake lint\n"}},
		{name: "composer scripts run through composer",
			files:    map[string]string{"composer.json": `{"scripts":{"test":"phpunit"}}`},
			tools:    []string{"PATH/composer"},
			verdicts: map[string]map[string]any{"test": check("test")},
			has:      []string{"RUN test (composer.json: test)\n  cmd: composer run test\n"}},
		{name: "a manifest's own toolchain checks run by evidence",
			files: map[string]string{"go.mod": "module x\n", "crate/Cargo.toml": "[package]\nname = \"x\"\n"},
			tools: []string{"PATH/go", "PATH/cargo"},
			proposals: []map[string]any{
				proposal("lint", "go vet ./...", ".", "go.mod"), proposal("test", "go test ./...", ".", "go.mod"),
				proposal("lint", "cargo clippy -- -D warnings", "crate", "crate/Cargo.toml"), proposal("test", "cargo test", "crate", "crate/Cargo.toml"),
				proposal("format-check", "cargo fmt --check", "crate", "crate/Cargo.toml"),
			},
			has: []string{"RUN lint (evidence go.mod)\n  cmd: go vet ./...\n", "RUN test (evidence go.mod)\n  cmd: go test ./...\n",
				"RUN lint (evidence crate/Cargo.toml) [crate]\n  cmd: cargo clippy -- -D warnings\n", "RUN test (evidence crate/Cargo.toml) [crate]\n", "RUN format-check (evidence crate/Cargo.toml) [crate]\n"}},
		{name: "every subproject's tasks run, labeled and in their own directory",
			files: map[string]string{
				"package.json":           `{"scripts":{"lint":"eslint ."}}`,
				"frontend/package.json":  `{"scripts":{"lint":"eslint .","test":"vitest run"}}`,
				"backend/pyproject.toml": "[tool.pdm.scripts]\ntest = \"pytest\"\n", "backend/pdm.lock": "",
			},
			tools:    []string{"PATH/npm", "PATH/pdm"},
			verdicts: map[string]map[string]any{"lint": check("lint"), "test": check("test")},
			managers: []map[string]any{manager("backend", "pdm.lock", "pdm")},
			has: []string{"RUN lint (package.json: lint)\n  cmd: npm run lint\n  bin:", "RUN lint (frontend/package.json: lint) [frontend]\n  cmd: npm run lint\n  dir: frontend\n",
				"RUN test (frontend/package.json: test) [frontend]\n", "RUN test (backend/pyproject.toml: test) [backend]\n  cmd: pdm run test\n  dir: backend\n"}},
		{name: "a pnpm workspace package's test runs in its own directory",
			files: map[string]string{
				"package.json": `{"name":"root","private":true}`, "pnpm-workspace.yaml": "packages:\n  - \"packages/*\"\n", "pnpm-lock.yaml": "",
				"packages/a/package.json": `{"name":"a","scripts":{"test":"vitest run"}}`,
			},
			tools:    []string{"PATH/pnpm"},
			verdicts: map[string]map[string]any{"test": check("test")},
			managers: []map[string]any{manager(".", "pnpm-lock.yaml", "pnpm")},
			has:      []string{"RUN test (packages/a/package.json: test) [packages/a]\n  cmd: pnpm run test\n  dir: packages/a\n"}},
		{name: "only the named subprojects are planned",
			files: map[string]string{
				"frontend/package.json":  `{"scripts":{"lint":"eslint ."}}`,
				"backend/pyproject.toml": "[tool.pdm.scripts]\nlint = \"ruff check .\"\n", "backend/pdm.lock": "",
			},
			tools:    []string{"PATH/npm", "PATH/pdm"},
			verdicts: map[string]map[string]any{"lint": check("lint")},
			managers: []map[string]any{manager("backend", "pdm.lock", "pdm")},
			only:     []string{"backend"},
			has:      []string{"RUN lint (backend/pyproject.toml: lint) [backend]"},
			lacks:    []string{"[frontend]"}},
		{name: "disabled_task_providers stops a provider's tasks being read",
			files:    map[string]string{"package.json": `{"scripts":{"lint":"eslint ."}}`, "Makefile": "lint:\n\teslint .\n"},
			tools:    []string{"PATH/npm"},
			verdicts: map[string]map[string]any{"lint": check("lint")},
			cfg:      func(cfg *config.Config) { cfg.DisabledTaskProviders = []string{"make"} },
			has:      []string{"RUN lint (package.json: lint)\n"},
			lacks:    []string{"Makefile"}},
	} {
		t.Run(c.name, c.run)
	}
}

// orchestrated is a pnpm workspace whose orchestrator, turbo or nx, has its
// binary in node_modules/.bin, beside a Python service outside the
// workspace.
func orchestrated(name, config string) map[string]string {
	return map[string]string{
		"package.json":                `{"name":"root","private":true,"scripts":{"test":"turbo run test"}}`,
		"pnpm-workspace.yaml":         "packages:\n  - \"packages/*\"\n",
		"pnpm-lock.yaml":              "",
		name + ".json":                config,
		"packages/a/package.json":     `{"name":"a","scripts":{"test":"vitest","lint":"eslint ."}}`,
		"services/api/pyproject.toml": "[tool.pdm.scripts]\ntest = \"pytest\"\n",
		"services/api/pdm.lock":       "",
	}
}

// A task graph runs once per check across the workspace packages it fans
// out over; a package's own task of a kind it covers never runs, and
// anything outside the workspace keeps its own.
func TestTaskGraphPlans(t *testing.T) {
	affected := func(kind string) map[string]any {
		return map[string]any{"role": "check", "kind": kind, "affected_form": "turbo run " + kind + " --filter=...[{base}]"}
	}
	managers := []map[string]any{manager(".", "pnpm-lock.yaml", "pnpm"), manager("services/api", "pdm.lock", "pdm")}
	tools := []string{"node_modules/.bin/turbo", "PATH/pnpm", "PATH/pdm"}
	for _, c := range []planCase{
		{name: "turbo runs once per check, and no package runs its own",
			files:    orchestrated("turbo", `{"tasks":{"test":{},"lint":{},"build":{}}}`),
			tools:    tools,
			verdicts: map[string]map[string]any{"test": affected("test"), "lint": affected("lint"), "build": {"role": "build"}},
			managers: managers,
			has:      []string{"RUN test (turbo.json: test)\n  cmd: turbo run test --filter=...[{base}]\n", "RUN lint (turbo.json: lint)\n  cmd: turbo run lint --filter=...[{base}]\n"},
			lacks:    []string{"(packages/a/package.json: test)", "(packages/a/package.json: lint)", "turbo run build"}},
		{name: "a check turbo declares no task for still runs per package",
			files:    orchestrated("turbo", `{"tasks":{"test":{}}}`),
			tools:    tools,
			verdicts: map[string]map[string]any{"test": affected("test"), "lint": check("lint")},
			managers: managers,
			has:      []string{"RUN test (turbo.json: test)\n", "RUN lint (packages/a/package.json: lint) [packages/a]\n"}},
		{name: "a service outside the workspace runs its own test",
			files:    orchestrated("turbo", `{"tasks":{"test":{}}}`),
			tools:    tools,
			verdicts: map[string]map[string]any{"test": affected("test")},
			managers: managers,
			has:      []string{"RUN test (services/api/pyproject.toml: test) [services/api]\n  cmd: pdm run test\n"}},
		{name: "--only a workspace package keeps the task graph",
			files:    orchestrated("turbo", `{"tasks":{"test":{}}}`),
			tools:    tools,
			verdicts: map[string]map[string]any{"test": affected("test")},
			managers: managers,
			only:     []string{"packages/a"},
			has:      []string{"RUN test (turbo.json: test)\n"},
			lacks:    []string{"[services/api]"}},
		{name: "--only the service leaves the task graph out",
			files:    orchestrated("turbo", `{"tasks":{"test":{}}}`),
			tools:    tools,
			verdicts: map[string]map[string]any{"test": affected("test")},
			managers: managers,
			only:     []string{"services/api"},
			has:      []string{"RUN test (services/api/pyproject.toml: test) [services/api]\n"},
			lacks:    []string{"turbo.json"}},
	} {
		t.Run(c.name, c.run)
	}
}

// What covers what: CI first, then a kind's tasks, each body read through
// classify.
func TestCoveragePlans(t *testing.T) {
	const workflow = ".github/workflows/ci.yml"
	for _, c := range []planCase{
		{name: "a fixer or a watcher runs nothing, whatever it calls",
			files:    map[string]string{"package.json": `{"scripts":{"lint":"eslint .","test":"vitest run","format":"npm run lint -- --fix && prettier --write .","dev":"npm run test -- --watch"}}`},
			tools:    []string{"PATH/npm"},
			verdicts: map[string]map[string]any{"lint": check("lint"), "test": check("test"), "format": fixer(), "dev": {"role": "dev"}},
			has:      []string{"RUN lint (package.json: lint)\n", "RUN test (package.json: test)\n"},
			lacks:    []string{"--fix", "--watch"}},
		{name: "a task that calls one of its kind with arguments runs, and the one it calls doesn't",
			files:    map[string]string{"package.json": `{"scripts":{"unit":"vitest run","ci":"npm run unit -- --coverage"}}`},
			tools:    []string{"PATH/npm"},
			verdicts: map[string]map[string]any{"unit": check("test"), "ci": check("test")},
			has:      []string{"RUN test (package.json: ci)\n  cmd: npm run ci\n"},
			lacks:    []string{"package.json: unit)"}},
		{name: "a kind disabled leaves the task an aggregate calls of another kind",
			files:    map[string]string{"package.json": `{"scripts":{"test":"npm run typecheck && vitest run","typecheck":"tsc --noEmit"}}`},
			tools:    []string{"PATH/npm", "node_modules/.bin/vitest"},
			verdicts: map[string]map[string]any{"test": aggregate(segment("vitest run", "test")), "typecheck": check("typecheck")},
			cfg:      func(cfg *config.Config) { cfg.DisabledChecks = []string{"test"} },
			has:      []string{"RUN typecheck (package.json: typecheck)\n", "SKIP test (package.json: test: vitest run) (disabled_checks)"}},
		{name: "an aggregate's kinds its tasks already run aren't run again",
			files:    map[string]string{"package.json": `{"scripts":{"lint":"eslint .","typecheck":"tsc --noEmit","check":"tsc --noEmit && eslint ."}}`},
			tools:    []string{"PATH/npm"},
			verdicts: map[string]map[string]any{"lint": check("lint"), "typecheck": check("typecheck"), "check": aggregate(segment("tsc --noEmit", "typecheck"), segment("eslint .", "lint"))},
			has:      []string{"RUN lint (package.json: lint)\n", "RUN typecheck (package.json: typecheck)\n"},
			lacks:    []string{"package.json: check"}},
		{name: "a gate dotenv loads env for runs with it",
			files:    map[string]string{"package.json": `{"scripts":{"ci":"dotenv -- vitest run && eslint ."}}`},
			tools:    []string{"node_modules/.bin/dotenv", "node_modules/.bin/vitest", "node_modules/.bin/eslint"},
			verdicts: map[string]map[string]any{"ci": aggregate(segment("dotenv -- vitest run", "test"), segment("eslint .", "lint"))},
			has:      []string{"  cmd: dotenv -- vitest run\n"},
			lacks:    []string{"  cmd: vitest run\n"}},
		{name: "run-s globs expand to the scripts they match",
			files:    map[string]string{"package.json": `{"scripts":{"check":"run-s lint:* typecheck","lint:js":"eslint .","lint:css":"stylelint .","typecheck":"tsc --noEmit"}}`},
			tools:    []string{"PATH/npm", "node_modules/.bin/run-s"},
			verdicts: map[string]map[string]any{"check": aggregate(), "lint:js": check("lint"), "lint:css": check("lint"), "typecheck": check("typecheck")},
			has:      []string{"RUN lint (package.json: lint:js)\n", "RUN lint (package.json: lint:css)\n", "RUN typecheck (package.json: typecheck)\n"},
			lacks:    []string{"package.json: check"}},
		{name: "a just {{var}} is never passed on as a literal word",
			files:    map[string]string{"Cargo.toml": "[package]\nname = \"x\"\n", "justfile": "ci:\n    cargo clippy --workspace {{flags}}\n    cargo test\n"},
			tools:    []string{"PATH/cargo"},
			verdicts: map[string]map[string]any{"ci": aggregate(segment("cargo clippy --workspace {{flags}}", "lint"), segment("cargo test", "test"))},
			lacks:    []string{"RUN lint"}},
		{name: "every CI check step runs as CI runs it, before any task of its kind",
			files: map[string]string{"package.json": `{"scripts":{"lint":"eslint .","test":"vitest run"}}`, workflow: "jobs:\n  c:\n    steps:\n      - run: npx tsc --noEmit\n      - run: npx tsc --noEmit -p tsconfig.other.json\n      - run: npx oxlint .\n      - run: npm test\n"},
			tools: []string{"PATH/npm", "PATH/npx", "node_modules/.bin/tsc", "node_modules/.bin/oxlint"},
			verdicts: map[string]map[string]any{"c/1": check("typecheck"), "c/2": check("typecheck"), "c/3": check("lint"), "c/4": check("test"),
				"lint": check("lint"), "test": check("test")},
			has: []string{"RUN typecheck (" + workflow + ": c/1)\n", "RUN typecheck (" + workflow + ": c/2)\n  cmd: npx tsc --noEmit -p tsconfig.other.json\n",
				"RUN lint (" + workflow + ": c/3)\n", "RUN test (package.json: test)\n  cmd: npm test\n"},
			lacks: []string{"package.json: lint)", "cmd: npm run test"}},
		{name: "a CI step runs whole when it sets things up, leaves the repo, or re-exports PATH",
			files:    map[string]string{"a.sh": "#!/bin/sh\n", workflow: "jobs:\n  sh:\n    steps:\n      - run: |\n          set -euo pipefail\n          shellcheck a.sh\n      - run: cd /tmp && shellcheck a.sh\n      - run: |\n          export PATH\n          shellcheck scripts/*.sh\n"},
			tools:    []string{"bin/shellcheck"},
			verdicts: map[string]map[string]any{"sh/1": check("lint"), "sh/2": check("lint"), "sh/3": check("lint")},
			has:      []string{"RUN lint (" + workflow + ": sh/1)\n  cmd: set -euo pipefail\n", "RUN lint (" + workflow + ": sh/2)\n  cmd: cd /tmp && shellcheck a.sh\n", "  cmd: export PATH\n       shellcheck scripts/*.sh\n"},
			lacks:    []string{"env: PATH"}},
		{name: "a CI step's cd or working-directory files it under that subproject",
			files: map[string]string{"web/package.json": `{"scripts":{"lint":"eslint ."}}`, "api/package.json": "{}",
				workflow: "jobs:\n  web:\n    steps:\n      - run: cd web && npx eslint .\n  api:\n    defaults:\n      run:\n        working-directory: api\n    steps:\n      - run: npx tsc --noEmit\n"},
			tools:    []string{"web/node_modules/.bin/eslint", "api/node_modules/.bin/tsc", "PATH/npm", "PATH/npx"},
			verdicts: map[string]map[string]any{"web/1": {"role": "check", "kind": "lint", "segments": []map[string]any{segment("npx eslint .", "lint")}}, "api/1": check("typecheck"), "lint": check("lint")},
			has:      []string{"  cmd: npx eslint .\n  dir: web\n", "RUN typecheck (" + workflow + ": api/1) [api]\n  cmd: npx tsc --noEmit\n  dir: api\n"},
			lacks:    []string{"web/package.json: lint"}},
		{name: "dead code from evidence is judged on changed lines only",
			files:     map[string]string{"go.mod": "module x\n\ntool golang.org/x/tools/cmd/deadcode\n"},
			tools:     []string{"PATH/go"},
			proposals: []map[string]any{proposal("deadcode", `test -z "$(go tool deadcode -test ./...)"`, ".", "go.mod")},
			has:       []string{"RUN deadcode (evidence go.mod)\n", "  scope: only findings on lines changed since the git base fail it\n"}},
		{name: "a dead-code target runs through make, which names its tool, scoped",
			files:    map[string]string{"go.mod": "module x\n", "Makefile": "deadcode:\n\t$(DEADCODE) ./...\n"},
			verdicts: map[string]map[string]any{"deadcode": check("deadcode")},
			has:      []string{"RUN deadcode (Makefile: deadcode)\n  cmd: make deadcode\n", "  scope: only findings on lines changed"}},
		{name: "a Dockerfile alone states nothing to check",
			files: map[string]string{"Dockerfile": "FROM scratch\n"},
			has:   []string{"SKIP everything (nothing in this project states a check"}},
	} {
		t.Run(c.name, c.run)
	}
}

func aggregate(segments ...map[string]any) map[string]any {
	v := map[string]any{"role": "check"}
	if len(segments) > 0 {
		v["segments"] = segments
	}
	return v
}

func segment(text, kind string) map[string]any {
	return map[string]any{"text": text, "role": "check", "kind": kind}
}

// An aggregate whose parts all check runs as its parts, each once; one
// holding anything else runs whole or not at all.
func TestAggregatePlans(t *testing.T) {
	for _, c := range []planCase{
		{name: "a lint task that calls lint tasks runs once, as itself",
			files:    map[string]string{"package.json": `{"scripts":{"lint":"pnpm lint:js && pnpm lint:css","lint:js":"eslint .","lint:css":"stylelint ."}}`},
			tools:    []string{"PATH/npm", "PATH/pnpm"},
			verdicts: map[string]map[string]any{"lint": check("lint"), "lint:js": check("lint"), "lint:css": check("lint")},
			has:      []string{"RUN lint (package.json: lint)\n  cmd: npm run lint\n"},
			lacks:    []string{"package.json: lint:js", "package.json: lint:css"}},
		{name: "a test that builds first runs whole, build and all",
			files:    map[string]string{"package.json": `{"scripts":{"test":"npm run build && npm run jest","build":"tsc","jest":"jest"}}`},
			tools:    []string{"PATH/npm"},
			verdicts: map[string]map[string]any{"test": check("test"), "build": {"role": "build"}, "jest": check("test")},
			has:      []string{"RUN test (package.json: test)\n  cmd: npm run test\n"},
			lacks:    []string{"cmd: npm run jest", "cmd: npm run build"}},
		{name: "an aggregate's checks run separately, its inline one in place, and the aggregate never runs",
			files:    map[string]string{"package.json": `{"scripts":{"ci":"npm run lint && npm run types && vitest run","lint":"eslint .","types":"tsc --noEmit"}}`},
			tools:    []string{"PATH/npm", "node_modules/.bin/vitest"},
			verdicts: map[string]map[string]any{"ci": aggregate(segment("vitest run", "test")), "lint": check("lint"), "types": check("typecheck")},
			has:      []string{"RUN lint (package.json: lint)\n", "RUN typecheck (package.json: types)\n", "RUN test (package.json: ci: vitest run)\n  cmd: vitest run\n"},
			lacks:    []string{"cmd: npm run ci"}},
		{name: "make prerequisites are the aggregate's parts",
			files:    map[string]string{"go.mod": "module x\n", "Makefile": "ci: golint unit\n\ngolint:\n\tgolangci-lint run ./...\n\nunit:\n\tgo test ./...\n"},
			verdicts: map[string]map[string]any{"ci": aggregate(), "golint": check("lint"), "unit": check("test")},
			has:      []string{"RUN lint (Makefile: golint)\n  cmd: make golint\n", "RUN test (Makefile: unit)\n  cmd: make unit\n"},
			lacks:    []string{"cmd: make ci"}},
		{name: "just dependencies and body gates are the aggregate's parts",
			files: map[string]string{"package.json": "{}", "justfile": "verify: fmtcheck\n    eslint .\n\nfmtcheck:\n    prettier --check .\n"},
			// No just binary: with one, the recipes come from just --summary.
			tools:    []string{"node_modules/.bin/eslint"},
			verdicts: map[string]map[string]any{"verify": aggregate(segment("eslint .", "lint")), "fmtcheck": check("format-check")},
			has:      []string{"SKIP format-check (justfile: fmtcheck) (just not installed)", "RUN lint (justfile: verify: eslint .)\n  cmd: eslint .\n"},
			lacks:    []string{"justfile: verify)"}},
		{name: "an inline gate carries the cd and exports before it",
			files:    map[string]string{"package.json": `{"scripts":{"ci":"export CI=1 && cd web && vitest run && eslint ."}}`, "web/.keep": ""},
			tools:    []string{"web/node_modules/.bin/vitest", "web/node_modules/.bin/eslint"},
			verdicts: map[string]map[string]any{"ci": aggregate(segment("vitest run", "test"), segment("eslint .", "lint"))},
			has:      []string{"RUN test (package.json: ci: vitest run) [web]\n  cmd: vitest run\n  dir: web\n  env: CI=1\n"}},
		{name: "a body with a setup statement runs whole, never split from it",
			files:    map[string]string{"package.json": `{"scripts":{"ci":"source .env && vitest run && eslint ."}}`},
			tools:    []string{"node_modules/.bin/vitest", "node_modules/.bin/eslint"},
			verdicts: map[string]map[string]any{"ci": {"role": "check", "segments": []map[string]any{{"text": "source .env", "role": "setup"}, segment("vitest run", "test"), segment("eslint .", "lint")}}},
			has:      []string{"RUN test+lint (package.json: ci)\n  cmd: source .env && vitest run && eslint .\n"},
			lacks:    []string{"ci: vitest run"}},
		{name: "a task reference follows the cd before it",
			files:    map[string]string{"package.json": `{"scripts":{"ci":"cd tools && make lint && eslint ."}}`, "tools/go.mod": "module tools\n", "tools/Makefile": "lint:\n\tgolangci-lint run\n"},
			tools:    []string{"node_modules/.bin/eslint"},
			verdicts: map[string]map[string]any{"ci": aggregate(), "lint": check("lint")},
			has:      []string{"RUN lint (tools/Makefile: lint) [tools]\n  cmd: make lint\n  dir: tools\n"}},
		{name: "a cd the kit can't follow runs nothing after it",
			files:    map[string]string{"package.json": `{"scripts":{"ci":"cd \"$APP_DIR\" && eslint . && tsc --noEmit"}}`},
			tools:    []string{"node_modules/.bin/eslint", "node_modules/.bin/tsc"},
			verdicts: map[string]map[string]any{"ci": aggregate(segment("eslint .", "lint"), segment("tsc --noEmit", "typecheck"))},
			lacks:    []string{"RUN lint (package.json: ci: eslint"}},
		{name: "a reference into another subproject is planned once",
			files:    map[string]string{"go.mod": "module x\n", "Makefile": "check:\n\t$(MAKE) -C web lint\n\tgo vet ./...\n", "web/go.mod": "module web\n", "web/Makefile": "lint:\n\tgolangci-lint run\n"},
			tools:    []string{"PATH/go"},
			verdicts: map[string]map[string]any{"check": aggregate(segment("go vet ./...", "lint")), "lint": check("lint")},
			has:      []string{"RUN lint (web/Makefile: lint) [web]\n"}},
		{name: "a cycle across runners ends, and an aggregate with a missing reference runs nothing",
			files:    map[string]string{"package.json": `{"scripts":{"ci":"make ci && npm run nope && eslint ."}}`, "Makefile": "ci:\n\tnpm run ci\n"},
			tools:    []string{"node_modules/.bin/eslint"},
			verdicts: map[string]map[string]any{"ci": aggregate()},
			lacks:    []string{"RUN"}},
		{name: "a body that runs only a script file, a fix, or a pipe runs nothing",
			files:    map[string]string{"package.json": `{"scripts":{"verify":"node scripts/verify.js","fixup":"eslint --fix .","piped":"eslint . | tee out"}}`},
			tools:    []string{"PATH/npm"},
			verdicts: map[string]map[string]any{"verify": {"role": "other"}, "fixup": fixer(), "piped": {"role": "other"}},
			lacks:    []string{"RUN"}},
	} {
		t.Run(c.name, c.run)
	}
}
