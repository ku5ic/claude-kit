package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// Precedence and aggregates: a slot-named task runs as itself and covers
// the slot-named tasks it calls; a task that calls other tasks or several
// gates is an aggregate, never run itself, its gates run separately.

func TestAggregates(t *testing.T) {
	t.Run("a slot-named task covers the slot-named tasks it calls, so each linter runs once", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"pnpm lint:js && pnpm lint:css","lint:js":"eslint .","lint:css":"stylelint ."}}`+"\n")
		e.stub("npm", 0)
		r := e.run()
		r.Has(t, "PASS js: lint (lint)", "SKIP js: lint (lint:js) (covered by lint)", "SKIP js: lint (lint:css) (covered by lint)")
		e.callsEqual("npm", e.phys(".")+" run lint")
	})
	t.Run("a build-then-test test runs as itself, build and all", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"test":"npm run build && npm run jest","build":"tsc","jest":"jest"}}`+"\n")
		e.stub("npm", 0)
		r := e.run()
		r.Has(t, "PASS js: test (test)", "SKIP js: test (jest) (covered by test)")
		e.callsEqual("npm", e.phys(".")+" run test")
	})
	t.Run("npm: an aggregate's gates run separately and the aggregate never does", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"ci":"npm run lint && npm run types && vitest run","lint":"eslint .","types":"tsc --noEmit"}}`+"\n")
		e.write(".gitignore", "node_modules\n")
		vitest := filepath.Join(e.project, "node_modules/.bin/vitest")
		Stub(t, vitest, "echo \"vitest $*\" >>"+filepath.Join(e.stubs, "vitest.calls")+"\n")
		e.stub("npm", 0)
		r := e.run()
		r.Has(t, "PASS js: lint (lint)", "PASS js: typecheck (types)",
			"PASS js: test (ci: vitest)\n  bin: "+Physical(t, vitest)+" (local)\n")
		e.callsEqual("npm", e.phys(".")+" run types\n"+e.phys(".")+" run lint")
		e.callsEqual("vitest", "vitest run")
	})
	t.Run("make: prerequisites are the aggregate's parts", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n")
		e.write("Makefile", "ci: golint unit\n\ngolint:\n\tgolangci-lint run ./...\n\nunit:\n\tgo test ./...\n")
		e.stub("make", 0)
		e.stub("go", 0)
		r := e.run()
		r.Has(t, "PASS make: lint (golint)", "PASS make: test (unit)")
		if strings.Contains(e.calls("make"), " ci") {
			t.Errorf("the aggregate ran: %s", e.calls("make"))
		}
	})
	t.Run("just: dependencies and body gates are the aggregate's parts", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		e.write("justfile", "verify: fmtcheck\n    eslint .\n\nfmtcheck:\n    prettier --check .\n")
		e.write(".gitignore", "node_modules\n")
		Stub(t, filepath.Join(e.project, "node_modules/.bin/eslint"), "")
		r := e.run("--plan")
		r.Has(t, "RUN just: format-check (fmtcheck)", "RUN just: lint (verify: eslint)")
		r.Lacks(t, "(verify)\n")
	})
	t.Run("an inline gate carries the cd and exports before it", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"ci":"export CI=1 && cd web && vitest run && eslint ."}}`+"\n")
		e.write("web/.keep", "")
		e.write(".gitignore", "node_modules\n")
		vitest := filepath.Join(e.project, "web/node_modules/.bin/vitest")
		Stub(t, vitest, "")
		Stub(t, filepath.Join(e.project, "node_modules/.bin/eslint"), "")
		r := e.run("--plan")
		r.Has(t, "RUN js: test (ci: vitest)\n  cmd: env CI=1 "+Physical(t, vitest)+" run\n  dir: web\n")
	})
	t.Run("an inline gate after a stateful command is skipped with the reason", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"ci":"source .env && vitest run && eslint ."}}`+"\n")
		r := e.run("--plan")
		r.Has(t, "SKIP js: test (ci: vitest) (depends on `source .env` in ci)", "SKIP js: lint (ci: eslint) (depends on `source .env` in ci)")
	})
	t.Run("arguments an aggregate passes a task are kept", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"ci":"npm run unit -- --coverage && eslint .","unit":"vitest run"}}`+"\n")
		e.write(".gitignore", "node_modules\n")
		Stub(t, filepath.Join(e.project, "node_modules/.bin/eslint"), "")
		e.run("--plan").Has(t, "RUN js: test (unit)\n  cmd: npm run unit -- --coverage\n")
	})
	t.Run("an aggregate's setup steps are named, not run", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"ci":"npm run build && vitest run && eslint .","build":"tsc -b"}}`+"\n")
		e.write(".gitignore", "node_modules\n")
		Stub(t, filepath.Join(e.project, "node_modules/.bin/vitest"), "")
		Stub(t, filepath.Join(e.project, "node_modules/.bin/eslint"), "")
		e.run("--plan").Has(t, "  note: ci runs `npm run build` first; run-checks doesn't\n")
	})
	t.Run("a cross-provider cycle ends, and a missing reference is named", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"ci":"make ci && npm run nope && eslint ."}}`+"\n")
		e.write("Makefile", "ci:\n\tnpm run ci\n")
		e.write(".gitignore", "node_modules\n")
		Stub(t, filepath.Join(e.project, "node_modules/.bin/eslint"), "")
		r := e.run("--plan")
		r.Want(t, 0)
		r.Has(t, "SKIP js: ci (references missing task nope)", "RUN js: lint (ci: eslint)")
	})
	t.Run("an aggregate whose body runs only a script file runs nothing", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"verify":"node scripts/verify.js"}}`+"\n")
		e.stub("npm", 0)
		e.run()
		if e.called("npm") {
			t.Errorf("ran: %s", e.calls("npm"))
		}
	})
}
