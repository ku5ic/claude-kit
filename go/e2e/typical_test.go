package e2e

import (
	"slices"
	"strings"
	"testing"
)

// One typical project per stack, empty overlay: run-checks finds every
// gate such a project declares and runs each once. The assertion is the
// whole set of verdict lines, so a gate run twice or a stray SKIP fails it.

// verdicts is r's PASS, FAIL, and SKIP lines, in order.
func verdicts(r Result) []string {
	var out []string
	for _, l := range Lines(r.Output) {
		if strings.HasPrefix(l, "PASS ") || strings.HasPrefix(l, "FAIL ") || strings.HasPrefix(l, "SKIP ") {
			out = append(out, l)
		}
	}
	return out
}

func wantVerdicts(t *testing.T, r Result, want ...string) {
	t.Helper()
	if got := verdicts(r); !slices.Equal(got, want) {
		t.Errorf("verdicts:\n  %s\nwant:\n  %s\noutput:\n%s", strings.Join(got, "\n  "), strings.Join(want, "\n  "), r.Output)
	}
}

func TestTypicalProjects(t *testing.T) {
	t.Run("js: scripts, an aggregate ci script, and a CI workflow", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"dev":"vite","build":"vite build","lint":"eslint .","typecheck":"tsc --noEmit",`+
			`"format":"prettier --write .","format:check":"prettier --check .","test":"vitest run","test:watch":"vitest",`+
			`"knip":"knip","ci":"pnpm lint && pnpm typecheck && pnpm test"}}`+"\n")
		e.write("pnpm-lock.yaml", "")
		e.workflow("jobs:\n  ci:\n    steps:\n      - uses: actions/checkout@v4\n      - run: pnpm install --frozen-lockfile\n      - run: pnpm run ci\n" +
			"  release:\n    permissions:\n      id-token: write\n    steps:\n      - run: pnpm publish\n")
		e.branchOff()
		e.write("src/app.ts", "export const a = 1\n")
		e.stub("pnpm", 0)
		wantVerdicts(t, e.run(),
			"PASS js: typecheck (typecheck)",
			"PASS js: lint (lint)",
			"PASS js: format-check (format:check)",
			"PASS js: test (test)",
			"PASS js: deadcode (knip)",
			"SKIP js: deadcode (covered by js: deadcode (knip))")
	})
	t.Run("python: pdm scripts, uv-free", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[project]\nname = \"x\"\n\n[tool.pdm.scripts]\nlint = \"ruff check .\"\ntypecheck = \"mypy src\"\n"+
			"format-check = \"ruff format --check .\"\ntest = \"pytest\"\ndeadcode = \"vulture src\"\n")
		e.write("pdm.lock", "")
		e.branchOff()
		e.write("src/app.py", "x = 1\n")
		e.stub("pdm", 0)
		wantVerdicts(t, e.run(),
			"PASS python: typecheck (typecheck)",
			"PASS python: lint (lint)",
			"PASS python: format-check (format-check)",
			"PASS python: test (test)",
			"PASS python: deadcode (deadcode)",
			"SKIP python: deadcode (covered by python: deadcode (deadcode))")
	})
	t.Run("ruby: rake lint and test", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("Gemfile", "source 'https://rubygems.org'\n")
		e.write("Rakefile", "task :lint do\nend\n\ntask :test do\nend\n")
		e.stub("bundle", 0)
		wantVerdicts(t, e.run(),
			"SKIP ruby: typecheck (no typecheck task)",
			"PASS ruby: lint (lint)",
			"SKIP ruby: format-check (no format-check task)",
			"PASS ruby: test (test)",
			"SKIP ruby: deadcode (no deadcode task)")
	})
	t.Run("go: a Makefile with lint, fmt-check, and test", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n")
		e.write("Makefile", "lint:\n\tgolangci-lint run ./...\n\nfmt-check:\n\ttest -z \"$$(gofmt -l .)\"\n\ntest:\n\tgo test ./...\n")
		e.stub("make", 0)
		e.stub("go", 0)
		wantVerdicts(t, e.run(),
			"SKIP make: typecheck (no typecheck task)",
			"PASS make: lint (lint)",
			"PASS make: format-check (fmt-check)",
			"PASS make: test (test)",
			"SKIP make: deadcode (no deadcode task)",
			"PASS go: vet",
			"SKIP go: test (covered by make: test (test))",
			"SKIP go: deadcode (deadcode not installed)")
	})
	t.Run("rust: Cargo.toml alone gets cargo's own checks", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("Cargo.toml", "[package]\nname = \"x\"\n")
		e.stub("cargo", 0)
		wantVerdicts(t, e.run(),
			"PASS rust: check",
			"PASS rust: clippy",
			"PASS rust: fmt",
			"PASS rust: test")
	})
	t.Run("opentofu: fmt runs, validate waits for init", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write(".terraform.lock.hcl", "")
		e.write("main.tf", "terraform {}\n")
		e.stub("tofu", 0)
		wantVerdicts(t, e.run(),
			"PASS opentofu: fmt",
			"SKIP opentofu: validate (no .terraform/ yet)")
	})
	t.Run("docker: a Dockerfile alone has no gates", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("Dockerfile", "FROM alpine\n")
		r := e.run()
		wantVerdicts(t, r)
		r.Has(t, "checks: 0 passed, 0 failed, 0 skipped")
	})
}
