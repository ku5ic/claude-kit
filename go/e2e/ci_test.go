package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// CI discovery: GitHub Actions and GitLab CI steps read like task bodies.
// Task references run the task; a tool run directly runs only when it
// resolves from the project, and only when nothing filled its check first.

func (e *runChecksEnv) workflow(body string) {
	e.write(".github/workflows/ci.yml", body)
}

// localTool puts a recording stub for name in the repo's .venv/bin or
// node_modules/.bin (by dir), gitignored.
func (e *runChecksEnv) localTool(dir, name string) string {
	e.write(".gitignore", "node_modules\n.venv\n")
	path := filepath.Join(e.project, dir, name)
	Stub(e.t, path, "echo \""+name+" $*\" >>"+filepath.Join(e.stubs, name+".calls")+"\n")
	return Physical(e.t, path)
}

func TestCIDiscovery(t *testing.T) {
	t.Parallel()
	t.Run("a tool a CI step runs directly runs when it resolves from the project", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[project]\nname = \"x\"\n")
		pytest := e.localTool(".venv/bin", "pytest")
		e.workflow("jobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - run: pytest -q\n")
		r := e.run()
		r.Has(t, "PASS python: test (.github/workflows/ci.yml: pytest)\n  bin: "+pytest+" (local)\n")
		e.callsEqual("pytest", "pytest -q")
	})
	t.Run("a CI-only tool with no project copy is listed, not run", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[project]\nname = \"x\"\n")
		e.stub("pytest", 0)
		e.workflow("jobs:\n  test:\n    steps:\n      - run: pytest\n")
		e.run().Has(t, "SKIP python: test (.github/workflows/ci.yml: pytest) (pytest only on PATH (")
		if e.called("pytest") {
			t.Error("pytest ran from PATH")
		}
	})
	t.Run("a task CI runs is the project's task, run once", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"test":"vitest run"}}`+"\n")
		e.stub("npm", 0)
		e.workflow("jobs:\n  test:\n    steps:\n      - run: npm test\n")
		r := e.run()
		if n := strings.Count(r.Output, "PASS js: test"); n != 1 {
			t.Errorf("test ran %d times:\n%s", n, r.Output)
		}
		e.callsEqual("npm", e.phys(".")+" run test")
	})
	t.Run("a CI tool line yields to the project task that fills its check", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"test":"vitest run"}}`+"\n")
		e.localTool("node_modules/.bin", "vitest")
		e.stub("npm", 0)
		e.workflow("jobs:\n  test:\n    steps:\n      - run: vitest run --coverage\n")
		r := e.run()
		r.Has(t, "PASS js: test (test)", "SKIP js: test (.github/workflows/ci.yml: vitest) (covered by js: test (test))")
		if e.called("vitest") {
			t.Errorf("vitest ran: %s", e.calls("vitest"))
		}
	})
	t.Run("a CI gate covers the toolchain check that stands in for its check", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("Cargo.toml", "[package]\nname = \"x\"\n")
		e.stub("cargo", 0)
		e.workflow("jobs:\n  test:\n    steps:\n      - run: cargo test --release --workspace\n")
		r := e.run()
		r.Has(t, "PASS rust: test (.github/workflows/ci.yml: cargo)", "SKIP rust: test (covered by rust: test (.github/workflows/ci.yml: cargo))")
	})
	t.Run("npx in CI reads as the tool, run from the project copy", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		eslint := e.localTool("node_modules/.bin", "eslint")
		e.workflow("jobs:\n  lint:\n    steps:\n      - run: npx eslint .\n")
		e.run().Has(t, "PASS js: lint (.github/workflows/ci.yml: eslint)\n  bin: "+eslint+" (local)\n")
		e.callsEqual("eslint", "eslint .")
	})
	t.Run("jobs a laptop shouldn't repeat are skipped whole", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[project]\nname = \"x\"\n")
		e.localTool(".venv/bin", "pytest")
		e.workflow(`jobs:
  db:
    services:
      postgres: {image: postgres}
    steps:
      - run: pytest
  oidc:
    permissions:
      id-token: write
    steps:
      - run: pytest
  secret:
    env:
      TOKEN: ${{ secrets.TOKEN }}
    steps:
      - run: pytest
  cloud:
    steps:
      - uses: aws-actions/configure-aws-credentials@v4
      - run: pytest
  deploy-docs:
    steps:
      - run: pytest
  prod:
    environment: production
    steps:
      - run: pytest
`)
		e.run().Lacks(t, "ci.yml: pytest")
		if e.called("pytest") {
			t.Errorf("ran: %s", e.calls("pytest"))
		}
	})
	t.Run("steps that use an action, deploy, publish, or need CI expressions are skipped", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		e.localTool("node_modules/.bin", "eslint")
		e.workflow(`jobs:
  build:
    steps:
      - uses: actions/setup-node@v4
      - run: npm publish
      - run: eslint . && docker push img
      - run: eslint ${{ matrix.dir }}
      - name: Upload coverage
        run: eslint .
      - run: |
          eslint src
        shell: pwsh
`)
		e.run().Lacks(t, "ci.yml: eslint")
		if e.called("eslint") {
			t.Errorf("ran: %s", e.calls("eslint"))
		}
	})
	t.Run("gitlab: a job's extends template supplies its script", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[project]\nname = \"x\"\n")
		e.localTool(".venv/bin", "pytest")
		e.write(".gitlab-ci.yml", ".py:\n  script:\n    - pytest\n\ntest:\n  extends: .py\n\nrelease:\n  script:\n    - pytest\n")
		r := e.run()
		r.Has(t, "PASS python: test (.gitlab-ci.yml: pytest)")
		if strings.Count(r.Output, ".gitlab-ci.yml: pytest") != 1 {
			t.Errorf("the release job ran too:\n%s", r.Output)
		}
	})
	t.Run("workflow-level defaults.run.working-directory holds for every step", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("services/api/pyproject.toml", "[project]\nname = \"api\"\n")
		e.localTool("services/api/.venv/bin", "pytest")
		e.workflow("defaults:\n  run:\n    working-directory: services/api\njobs:\n  test:\n    steps:\n      - run: pytest\n")
		e.run().Has(t, "PASS python: test (.github/workflows/ci.yml: pytest) [services/api]")
	})
	t.Run("workflow-level secrets, OIDC, or a pwsh default shell skip it all", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[project]\nname = \"x\"\n")
		e.localTool(".venv/bin", "pytest")
		e.write(".github/workflows/a.yml", "env:\n  TOKEN: ${{ secrets.T }}\njobs:\n  t:\n    steps:\n      - run: pytest\n")
		e.write(".github/workflows/b.yml", "permissions:\n  id-token: write\njobs:\n  t:\n    steps:\n      - run: pytest\n")
		e.write(".github/workflows/c.yml", "defaults:\n  run:\n    shell: pwsh\njobs:\n  t:\n    steps:\n      - run: pytest\n")
		e.run().Lacks(t, ": pytest)")
	})
	t.Run("a path-qualified tool in CI runs from that path", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		eslint := e.localTool("node_modules/.bin", "eslint")
		e.workflow("jobs:\n  lint:\n    steps:\n      - run: ./node_modules/.bin/eslint .\n")
		e.run().Has(t, "PASS js: lint (.github/workflows/ci.yml: eslint)\n  bin: "+eslint+" (local)\n")
	})
	t.Run("gitlab: before_script and script run in one shell", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("tools/Makefile", "lint:\n\tgolangci-lint run\n")
		e.write("go.mod", "module example.com/x\n")
		e.write(".gitlab-ci.yml", "lint:\n  before_script:\n    - cd tools\n  script:\n    - make lint\n")
		e.run("--plan").Has(t, "RUN go: lint (lint) [tools]\n  cmd: make lint\n  dir: tools\n")
	})
	t.Run("working-directory files a step under its subproject", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("services/api/pyproject.toml", "[project]\nname = \"api\"\n")
		e.localTool("services/api/.venv/bin", "pytest")
		e.workflow("jobs:\n  test:\n    steps:\n      - run: pytest\n        working-directory: services/api\n")
		e.run().Has(t, "PASS python: test (.github/workflows/ci.yml: pytest) [services/api]")
	})
}
