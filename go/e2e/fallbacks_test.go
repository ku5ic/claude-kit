package e2e

import "testing"

// Test-slot fallbacks: with no task named test, the first fallback name
// present runs (test:unit, test-unit, test:ci, ...) and covers the others;
// with a test task, the fallback names are covered by it. Watch and e2e
// variants never run.

func TestTestFallbacks(t *testing.T) {
	t.Parallel()
	t.Run("js: the first fallback present runs and covers the rest", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"test:ci":"vitest run --ci","test:unit":"vitest run"}}`+"\n")
		e.stub("npm", 0)
		r := e.run()
		r.Has(t, "PASS js: test (test:unit)", "SKIP js: test (test:ci) (covered by test:unit)")
		e.callsEqual("npm", e.phys(".")+" run test:unit")
	})
	t.Run("js: a test task covers the fallback names", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"test":"vitest run","test:unit":"vitest run --dir unit"}}`+"\n")
		e.stub("npm", 0)
		r := e.run()
		r.Has(t, "PASS js: test (test)", "SKIP js: test (test:unit) (covered by test)")
		e.callsEqual("npm", e.phys(".")+" run test")
	})
	t.Run("js: watch and e2e variants never run", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"test:watch":"vitest","test:e2e":"playwright test"}}`+"\n")
		e.stub("npm", 0)
		e.run().Has(t, "SKIP js: test (no test task)")
		if e.called("npm") {
			t.Errorf("ran: %s", e.calls("npm"))
		}
	})
	t.Run("js: browser-bound test scripts (storybook, playwright) never run", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"test-storybook":"vitest --project=storybook","a11y-test-storybook":"vitest run --project=storybook","smoke":"vitest run smoke"}}`+"\n")
		e.stub("npm", 0)
		e.run().Has(t, "SKIP js: test (no test task)")
		if e.called("npm") {
			t.Errorf("ran: %s", e.calls("npm"))
		}
	})
	t.Run("python: a pdm test-unit script", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[tool.pdm.scripts]\ntest-unit = \"pytest tests/unit\"\n")
		e.stub("pdm", 0)
		e.run().Has(t, "PASS python: test (test-unit)")
	})
	t.Run("ruby: a rake test:unit task", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("Gemfile", "source 'https://rubygems.org'\n")
		e.write("Rakefile", "task \"test:unit\" do\nend\n")
		e.stub("bundle", 0)
		e.run().Has(t, "PASS ruby: test (test:unit)")
	})
	t.Run("go: a make test-unit target covers the toolchain test", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n")
		e.write("Makefile", "test-unit:\n\tgo test -short ./...\n")
		e.stub("make", 0)
		e.stub("go", 0)
		e.run().Has(t, "PASS make: test (test-unit)", "SKIP go: test (covered by make: test (test-unit))")
	})
	t.Run("rust: a cargo test-all alias covers the toolchain test", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("Cargo.toml", "[package]\nname = \"x\"\n")
		e.write(".cargo/config.toml", "[alias]\ntest-all = \"test --workspace\"\n")
		e.stub("cargo", 0)
		e.run().Has(t, "PASS rust: test (test-all)", "SKIP rust: test (covered by rust: test (test-all))")
	})
}
