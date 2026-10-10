package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// Which gate covers which: a gate covers another only when it runs the same
// tool for the same check, a disabled task covers nothing, and arguments an
// aggregate passes count only while the gate is still one.

func TestGateCoverage(t *testing.T) {
	t.Parallel()
	t.Run("arguments that turn a gate into a fix or a watch run nothing", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint .","test":"vitest run","format":"npm run lint -- --fix && prettier --write .","dev":"npm run test -- --watch"}}`+"\n")
		r := e.run("--plan")
		r.Has(t, "RUN js: lint (lint)\n  cmd: npm run lint\n", "RUN js: test (test)\n  cmd: npm run test\n")
		r.Lacks(t, "--fix", "--watch")
	})
	t.Run("arguments that keep a gate a gate are passed on", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"unit":"vitest run","ci":"npm run unit -- --coverage"}}`+"\n")
		e.run("--plan").Has(t, "RUN js: test (unit)\n  cmd: npm run unit -- --coverage\n")
	})
	t.Run("arguments after a stateful command leave the task its plain run", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"unit":"vitest run","ci":"source .env && npm run unit -- --coverage"}}`+"\n")
		e.stub("npm", 0)
		r := e.run()
		r.Want(t, 0)
		r.Has(t, "PASS js: test (unit)")
		e.callsEqual("npm", e.phys(".")+" run unit")
	})
	t.Run("a fallback a task glob also matches is the primary, not covered by itself", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"test:unit":"vitest run","test:ci":"vitest run --coverage"}}`+"\n")
		e.k.Overlay("checks:\n  - {name: test, tasks: [test, \"test:unit\"]}\n")
		e.run("--plan").Has(t, "RUN js: test (test:unit)", "SKIP js: test (test:ci) (covered by test:unit)")
	})
	t.Run("a JS test script doesn't cover the Go toolchain test beside it", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n")
		e.write("package.json", `{"scripts":{"test":"jest"}}`+"\n")
		e.stub("go", 0)
		e.run("--plan").Has(t, "RUN js: test (test)", "RUN go: test\n")
	})
	t.Run("a Makefile test target doesn't cover a JS fallback test", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"test:unit":"vitest run"}}`+"\n")
		e.write("Makefile", "test:\n\tpytest\n")
		e.run("--plan").Has(t, "RUN make: test (test)", "RUN js: test (test:unit)")
	})
	t.Run("a cd on one recipe line ends with that line", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n")
		e.write("Makefile", "check:\n\tcd web\n\tgo test ./...\n\tgolangci-lint run\n")
		e.stub("go", 0)
		r := e.run("--plan")
		r.Has(t, "RUN make: test (check: go)\n")
		r.Lacks(t, "dir: web")
	})
	t.Run("a task disabled by name covers nothing it calls", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"test":"npm run typecheck && vitest run","typecheck":"tsc --noEmit"}}`+"\n")
		e.k.Overlay("disabled_checks: [test]\n")
		e.run("--plan").Has(t, "RUN js: typecheck (typecheck)", "SKIP js: test (test) (disabled_checks)")
	})
	t.Run("an aggregate's inline gate yields to the task already running its tool", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint .","typecheck":"tsc --noEmit","check":"tsc --noEmit && eslint ."}}`+"\n")
		e.run("--plan").Has(t,
			"SKIP js: typecheck (check: tsc) (covered by js: typecheck (typecheck))",
			"SKIP js: lint (check: eslint) (covered by js: lint (lint))")
	})
	t.Run("dotenv loads env the gate needs, so its gate isn't run bare", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"ci":"dotenv -- vitest run && eslint ."}}`+"\n")
		e.run("--plan").Lacks(t, "(ci: vitest)")
	})
	t.Run("disabled_checks turns off the toolchain check for its slot", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n")
		e.stub("go", 0)
		e.k.Overlay("disabled_checks: [test]\n")
		e.run("--plan").Has(t, "SKIP go: test (disabled_checks)", "RUN go: vet")
	})
	t.Run("run-s globs expand to the scripts they match", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"check":"run-s lint:* typecheck","lint:js":"eslint .","lint:css":"stylelint .","typecheck":"tsc --noEmit"}}`+"\n")
		r := e.run("--plan")
		r.Has(t, "RUN js: lint (lint:js)", "RUN js: lint (lint:css)", "RUN js: typecheck (typecheck)")
		r.Lacks(t, "references missing task")
	})
	t.Run("a just {{var}} is an unknown value, never a literal word", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("Cargo.toml", "[package]\nname = \"x\"\n")
		e.write("justfile", "ci:\n    cargo clippy --workspace {{flags}}\n    cargo test\n")
		e.stub("cargo", 0)
		e.run("--plan").Lacks(t, "{{flags}}")
	})
	t.Run("lint:prettier is a format check, run once", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint:prettier":"prettier --check ."}}`+"\n")
		r := e.run("--plan")
		r.Has(t, "RUN js: format-check (lint:prettier)")
		r.Lacks(t, "RUN js: lint (lint:prettier)")
	})
	t.Run("a CI step the same tool already covers names the command it skipped", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"devDependencies":{"typescript":"5"}}`+"\n")
		e.localTool("node_modules/.bin", "tsc")
		e.workflow("jobs:\n  c:\n    steps:\n      - run: npx tsc --noEmit\n      - run: npx tsc --noEmit -p tsconfig.other.json\n")
		e.run("--plan").Has(t, "SKIP js: typecheck (.github/workflows/ci.yml: tsc) (covered by an earlier step running tsc; skipped `tsc --noEmit -p tsconfig.other.json`)")
	})
	t.Run("a root task that cds into a subproject covers that subproject's toolchain check", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("Makefile", "test:\n\tcd svc && go test ./...\n")
		e.write("svc/go.mod", "module example.com/svc\n")
		e.stub("go", 0)
		r := e.run("--plan")
		r.Has(t, "RUN make: test (test)", "SKIP go: test [svc] (covered by make: test (test))", "RUN go: vet [svc]")
		e.run("--plan", "--only", "svc").Has(t, "RUN go: test [svc]")
	})
}

func TestCIShellSemantics(t *testing.T) {
	t.Parallel()
	t.Run("a glob in a CI step expands as the shell would", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("scripts/a.sh", "#!/bin/sh\n")
		e.write("scripts/b.sh", "#!/bin/sh\n")
		e.stub("shellcheck", 0)
		e.workflow("jobs:\n  lint:\n    steps:\n      - run: shellcheck scripts/*.sh\n")
		e.run().Has(t, "PASS ci: lint (.github/workflows/ci.yml: shellcheck)")
		e.callsEqual("shellcheck", e.phys(".")+" scripts/a.sh scripts/b.sh")
	})
	t.Run("set -euo pipefail doesn't block the gates after it", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("a.sh", "#!/bin/sh\n")
		e.stub("shellcheck", 0)
		e.workflow("jobs:\n  lint:\n    steps:\n      - run: |\n          set -euo pipefail\n          shellcheck a.sh\n")
		e.run("--plan").Has(t, "RUN ci: lint (.github/workflows/ci.yml: shellcheck)")
	})
	t.Run("a cd outside the repo blocks the gates after it", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.stub("shellcheck", 0)
		e.workflow("jobs:\n  lint:\n    steps:\n      - run: cd /tmp && shellcheck a.sh\n")
		e.run("--plan").Has(t, "SKIP ci: lint (.github/workflows/ci.yml: shellcheck) (depends on `cd /tmp`")
	})
	t.Run("a bare export PATH re-exports, never empties it", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("a.sh", "#!/bin/sh\n")
		e.stub("shellcheck", 0)
		e.workflow("jobs:\n  lint:\n    steps:\n      - run: |\n          export PATH\n          shellcheck a.sh\n")
		r := e.run("--plan")
		r.Has(t, "RUN ci: lint (.github/workflows/ci.yml: shellcheck)\n  cmd: "+filepath.Join(e.stubs, "shellcheck")+" a.sh\n")
		r.Lacks(t, "PATH=")
	})
	t.Run("an inline gate after a cd into a subproject belongs to that subproject", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("web/package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		e.localTool("web/node_modules/.bin", "eslint")
		e.workflow("jobs:\n  lint:\n    steps:\n      - run: cd web && npx eslint .\n")
		e.run("--plan").Has(t, "RUN js: lint (lint) [web]",
			"SKIP js: lint (.github/workflows/ci.yml: eslint) [web] (covered by js: lint (lint) [web])")
	})
	t.Run("a CI-only gate in a subproject it cds into still runs there", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("web/package.json", "{}\n")
		e.localTool("web/node_modules/.bin", "tsc")
		e.workflow("jobs:\n  types:\n    steps:\n      - run: cd web && npx tsc --noEmit\n")
		r := e.run("--plan")
		r.Has(t, "RUN js: typecheck (.github/workflows/ci.yml: tsc) [web]\n")
		r.Lacks(t, "SKIP js: typecheck [web] (no typecheck task)")
	})
	t.Run("a CI tool the project lacks is one SKIP line for its check", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		e.workflow("jobs:\n  lint:\n    steps:\n      - run: npx oxlint .\n")
		r := e.run("--plan")
		if n := strings.Count(r.Output, "js: lint"); n != 1 {
			t.Errorf("js: lint reported %d times:\n%s", n, r.Output)
		}
		r.Has(t, "SKIP js: lint (.github/workflows/ci.yml: oxlint) (")
	})
	t.Run("a CI tool the project has runs beside a task running another tool", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		e.localTool("node_modules/.bin", "oxlint")
		e.workflow("jobs:\n  lint:\n    steps:\n      - run: npx oxlint .\n")
		e.run("--plan").Has(t, "RUN js: lint (lint)", "RUN js: lint (.github/workflows/ci.yml: oxlint)")
	})
	t.Run("a Windows job's steps are pwsh, never read as sh", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("a.sh", "#!/bin/sh\n")
		e.stub("shellcheck", 0)
		e.workflow("jobs:\n  lint:\n    runs-on: windows-latest\n    steps:\n      - run: shellcheck a.sh\n")
		e.run("--plan").Lacks(t, "shellcheck")
	})
	t.Run("GitLab default services hold for every job", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("a.sh", "#!/bin/sh\n")
		e.stub("shellcheck", 0)
		e.write(".gitlab-ci.yml", "default:\n  services: [postgres]\nlint:\n  script:\n    - shellcheck a.sh\n")
		if out := e.run("--plan").Output; strings.Contains(out, "shellcheck") {
			t.Errorf("a service-backed job was read:\n%s", out)
		}
	})
	t.Run("GitLab global variables reach every job", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("a.sh", "#!/bin/sh\n")
		e.stub("shellcheck", 0)
		e.write(".gitlab-ci.yml", "variables:\n  SHELLCHECK_OPTS: -x\nlint:\n  script:\n    - shellcheck a.sh\n")
		e.run("--plan").Has(t, "cmd: env SHELLCHECK_OPTS=-x ")
	})
}
