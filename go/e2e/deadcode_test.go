package e2e

import (
	"path/filepath"
	"strconv"
	"testing"
)

// The deadcode slot runs whole-program but only fails on findings on lines
// changed since the git base, judged from its output, never its exit code.
// The task runner stub prints the dead-code tool's output, in the format
// each tool printed in the step 7 sandbox runs.

// branchOff commits everything on main and checks out a feature branch, so
// main is the git base and later writes are the change.
func (e *runChecksEnv) branchOff() {
	e.k.Git(e.project, "add", "-A")
	e.k.Git(e.project, "commit", "-q", "-m", "base")
	e.k.Git(e.project, "checkout", "-q", "-b", "feat")
}

// prints makes runner name print out and exit with code.
func (e *runChecksEnv) prints(name, out string, code int) {
	Stub(e.t, filepath.Join(e.stubs, name), "cat <<'EOF'\n"+out+"\nEOF\nexit "+strconv.Itoa(code)+"\n")
}

func TestDeadcode(t *testing.T) {
	knip := func(t *testing.T) *runChecksEnv {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"knip":"knip"}}`+"\n")
		e.write("src/old.ts", "export const old = 1\n")
		e.branchOff()
		e.write("src/new.ts", "export const fresh = 2\n")
		return e
	}
	t.Run("js: a knip finding in a changed file fails, one in an unchanged file doesn't", func(t *testing.T) {
		e := knip(t)
		e.prints("npm", "Unused exports (2)\nold  src/old.ts:1:14\nfresh  src/new.ts:1:14", 1)
		r := e.run()
		r.Has(t, "FAIL js: deadcode (knip)", "fresh  src/new.ts:1:14")
		r.Lacks(t, "old  src/old.ts:1:14")
	})
	t.Run("js: knip findings only in unchanged files pass, whatever the exit code", func(t *testing.T) {
		e := knip(t)
		e.prints("npm", "Unused exports (1)\nold  src/old.ts:1:14", 1)
		e.run().Has(t, "PASS js: deadcode (knip) (1 finding on unchanged lines)")
	})
	t.Run("js: knip's unused-files list, bare paths, is scoped like any finding", func(t *testing.T) {
		e := knip(t)
		e.prints("npm", "> knip\n\nUnused files (1)\nsrc/old.ts", 1)
		e.run().Has(t, "PASS js: deadcode (knip) (1 finding on unchanged lines)")
		e.prints("npm", "Unused files (1)\nsrc/new.ts\nUnused exports (1)\nold  src/old.ts:1:14", 1)
		e.run().Has(t, "FAIL js: deadcode (knip)", "src/new.ts")
	})
	t.Run("js: knip output with nothing parsed and a non-zero exit is a tool error", func(t *testing.T) {
		e := knip(t)
		e.prints("npm", "> knip\nError: Cannot read knip.json", 2)
		e.run().Has(t, "FAIL js: deadcode (knip)", "Error: Cannot read knip.json")
	})
	t.Run("touching a file doesn't inherit its old finding; a new line's finding fails", func(t *testing.T) {
		e := knip(t)
		e.write("src/old.ts", "export const old = 1\n// touched\nexport const added = 3\n")
		e.prints("npm", "Unused exports (2)\nold  src/old.ts:1:14\nadded  src/old.ts:3:14", 1)
		r := e.run()
		r.Has(t, "FAIL js: deadcode (knip)", "added  src/old.ts:3:14")
		r.Lacks(t, "old  src/old.ts:1:14")
		e.prints("npm", "Unused exports (1)\nold  src/old.ts:1:14", 1)
		e.run().Has(t, "PASS js: deadcode (knip) (1 finding on unchanged lines)")
	})
	t.Run("a new line's finding fails in a tracked spaced path, whatever the diff prefix config", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"knip":"knip"}}`+"\n")
		e.write("src/my old.ts", "export const old = 1\n")
		e.branchOff()
		e.k.Git(e.project, "config", "diff.mnemonicPrefix", "true")
		e.write("src/my old.ts", "export const old = 1\nexport const added = 2\n")
		e.prints("npm", "Unused exports (1)\nadded  src/my old.ts:2:14", 1)
		e.run().Has(t, "FAIL js: deadcode (knip)", "added  src/my old.ts:2:14")
	})
	t.Run("added or removed lines that look like diff headers stay content", func(t *testing.T) {
		e := knip(t)
		// In the file with findings, so its diff reaches the parser: a
		// removed "-- x" and an added "++ b/src/x.ts" print as --- and +++.
		e.write("src/old.ts", "export const old = 1\n-- x\n")
		e.k.Git(e.project, "commit", "-q", "-am", "x")
		e.write("src/old.ts", "export const old = 1\n++ b/src/x.ts\nexport const added = 2\n")
		e.prints("npm", "Unused exports (2)\nold  src/old.ts:1:14\nadded  src/old.ts:3:14", 1)
		r := e.run()
		r.Has(t, "FAIL js: deadcode (knip)", "added  src/old.ts:3:14")
		r.Lacks(t, "old  src/old.ts:1:14")
	})
	t.Run("a finding in a changed file whose path has a space still fails", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"knip":"knip"}}`+"\n")
		e.branchOff()
		e.write("src/my module.ts", "export const fresh = 2\n")
		e.prints("npm", "Unused exports (1)\nfresh  src/my module.ts:1:14", 1)
		e.run().Has(t, "FAIL js: deadcode (knip)")
	})
	t.Run("python: vulture's exit 3 with only unchanged findings passes", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[tool.pdm.scripts]\ndeadcode = \"vulture src\"\n")
		e.write("src/old.py", "def old(): pass\n")
		e.branchOff()
		e.write("src/new.py", "def fresh(): pass\n")
		e.prints("pdm", "src/old.py:1: unused function 'old' (60% confidence)", 3)
		e.run().Has(t, "PASS python: deadcode (deadcode) (1 finding on unchanged lines)")
	})
	t.Run("go: deadcode's exit 0 with a finding in a changed file fails", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n")
		e.write("Makefile", "deadcode:\n\tdeadcode ./...\n")
		e.branchOff()
		e.write("new.go", "package main\n")
		e.stub("go", 0)
		e.prints("make", "new.go:1:1: unreachable func: fresh", 0)
		e.run().Has(t, "FAIL make: deadcode (deadcode)", "new.go:1:1: unreachable func: fresh")
	})
	t.Run("opentofu: tflint limited to unused declarations is dead code, scoped", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write(".terraform.lock.hcl", "")
		e.write("Makefile", "unused-vars:\n\ttflint --only=terraform_unused_declarations\n")
		e.write("old.tf", "variable \"old\" {}\n")
		e.branchOff()
		e.write("new.tf", "output \"o\" { value = 1 }\n")
		e.stub("tofu", 0)
		e.prints("make", "old.tf:1:1: Warning - variable \"old\" is declared but not used (terraform_unused_declarations)", 2)
		e.run().Has(t, "PASS make: deadcode (unused-vars) (1 finding on unchanged lines)")
	})
	t.Run("ruby: debride is advisory, so even a changed file's finding passes", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("Gemfile", "source 'https://rubygems.org'\n")
		e.write("Makefile", "deadcode:\n\tdebride lib\n")
		e.branchOff()
		e.write("lib/fx.rb", "class Fx; def unused; end; end\n")
		e.prints("make", "These methods MIGHT not be called:\n\nFx\n  unused   lib/fx.rb:1-1 (1)", 0)
		e.run().Has(t, "PASS make: deadcode (deadcode) (advisory: output not mapped to files)")
	})
	t.Run("a deadcode task whose body can't be read is advisory", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"deadcode":"node scripts/dead.js"}}`+"\n")
		e.branchOff()
		e.write("a.ts", "x\n")
		e.prints("npm", "anything", 1)
		e.run().Has(t, "PASS js: deadcode (deadcode) (advisory: output not mapped to files)")
	})
	t.Run("with no git base, dead code is skipped", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"knip":"knip"}}`+"\n")
		e.stub("npm", 0)
		e.run().Has(t, "SKIP js: deadcode (knip) (no git base)")
		if e.called("npm") {
			t.Error("knip ran with no base")
		}
	})
	t.Run("with nothing changed since the base, dead code passes without running", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"knip":"knip"}}`+"\n")
		e.branchOff()
		e.stub("npm", 0)
		e.run().Has(t, "PASS js: deadcode (knip) (nothing changed since main)")
		if e.called("npm") {
			t.Error("knip ran with nothing changed")
		}
	})
	t.Run("--plan says how dead code is judged", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"knip":"knip","dead":"node x.js"}}`+"\n")
		r := e.run("--plan")
		r.Has(t, "RUN js: deadcode (knip)\n  cmd: npm run knip\n  scope: only findings on lines changed since the git base fail it\n")
	})
}
