package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// The tool_resolution matrix (kit.yml), through run-checks: for each stack
// and each rung, which copy of a tool a toolchain check runs, or why it
// skips. Every copy is a recorder appending its label to <stubs>/rec.calls,
// so a test asserts which one ran. Shared-resolver cases for the Stop hook
// and format-dispatch live with those tests.

// probe adds a toolchain check "probe" for stack on bin, plus extra overlay.
func (e *runChecksEnv) probe(stack, bin, extra string) {
	e.k.Overlay(fmt.Sprintf("toolchain_checks:\n  - {stack: %s, name: probe, cmd: \"{bin} --probe\", bin: [%s]}\n%s", stack, bin, extra))
	e.write(".gitignore", "node_modules\n.venv\nenvs\npenv\n")
}

// recorder writes an executable at path that records label.
func (e *runChecksEnv) recorder(path, label string) {
	Stub(e.t, path, fmt.Sprintf("echo %q >>%q\n", label, filepath.Join(e.stubs, "rec.calls")))
}

// installed puts a recording fakefmt at version under dir/node_modules.
func (e *runChecksEnv) installed(dir, version, label string) string {
	e.write(filepath.Join(dir, "node_modules/fakefmt/package.json"), `{"name":"fakefmt","version":"`+version+`"}`)
	bin := filepath.Join(e.project, dir, "node_modules/.bin/fakefmt")
	e.recorder(bin, label)
	return Physical(e.t, bin)
}

func (e *runChecksEnv) phys(rel string) string { return Physical(e.t, filepath.Join(e.project, rel)) }

func TestResolutionMatrix(t *testing.T) {
	// A project-local or package-manager copy beats one on PATH.
	t.Run("python: a .venv copy beats PATH", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[project]\nname = \"x\"\n")
		e.probe("python", "fakepy", "")
		e.recorder(filepath.Join(e.project, ".venv/bin/fakepy"), "local")
		e.recorder(filepath.Join(e.stubs, "fakepy"), "path")
		e.run().Has(t, "PASS python: probe\n  bin: "+e.phys(".venv/bin/fakepy")+" (local)\n")
		e.callsEqual("rec", "local")
	})
	t.Run("python: a poetry env beats PATH", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[tool.poetry]\nname = \"x\"\n")
		e.write("poetry.lock", "")
		e.probe("python", "fakepy", "")
		Stub(t, filepath.Join(e.stubs, "poetry"), fmt.Sprintf("[ \"$*\" = \"env info -p\" ] && echo %q\n", filepath.Join(e.project, "penv")))
		e.recorder(filepath.Join(e.project, "penv/bin/fakepy"), "poetry")
		e.recorder(filepath.Join(e.stubs, "fakepy"), "path")
		e.run().Has(t, "(package-manager env)")
		e.callsEqual("rec", "poetry")
	})
	t.Run("python: an activated virtualenv inside the repo beats PATH", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[project]\nname = \"x\"\n")
		e.probe("python", "fakepy", "")
		e.k.Setenv("VIRTUAL_ENV", filepath.Join(e.project, "envs/dev"))
		e.recorder(filepath.Join(e.project, "envs/dev/bin/fakepy"), "venv")
		e.recorder(filepath.Join(e.stubs, "fakepy"), "path")
		e.run().Has(t, "PASS python: probe")
		e.callsEqual("rec", "venv")
	})
	t.Run("ruby: a bundled gem runs through bundle exec, not PATH", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("Gemfile", "source 'https://rubygems.org'\n")
		e.write("Gemfile.lock", "GEM\n  specs:\n    fakerb (1.0)\n")
		e.probe("ruby", "fakerb", "")
		Stub(t, filepath.Join(e.stubs, "bundle"), fmt.Sprintf("case \"$1\" in\ninfo) exit 0 ;;\nexec) echo \"bundle $2\" >>%q ;;\nesac\n", filepath.Join(e.stubs, "rec.calls")))
		e.recorder(filepath.Join(e.stubs, "fakerb"), "path")
		e.run().Has(t, "PASS ruby: probe\n  bin: bundle exec fakerb (package-manager env)\n")
		e.callsEqual("rec", "bundle fakerb")
	})
	t.Run("go: a go.mod tool beats PATH", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("go.mod", "module example.com/x\n\ntool example.com/cmd/fakego\n")
		e.probe("go", "fakego", "")
		built := filepath.Join(e.stubs, "built/fakego")
		Stub(t, filepath.Join(e.stubs, "go"), fmt.Sprintf("[ \"$1 $2 $3\" = \"tool -n fakego\" ] && echo %q\nexit 0\n", built))
		e.recorder(built, "gotool")
		e.recorder(filepath.Join(e.stubs, "fakego"), "path")
		e.run().Has(t, "PASS go: probe\n  bin: "+built+" (package-manager env)\n")
		e.callsEqual("rec", "gotool")
	})

	t.Run("a nested subproject without its own copy uses the root's before PATH", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		e.write("packages/b/package.json", "{}\n")
		e.probe("js", "fakefmt", "")
		e.recorder(filepath.Join(e.project, "node_modules/.bin/fakefmt"), "root")
		e.recorder(filepath.Join(e.stubs, "fakefmt"), "path")
		e.run().Has(t, "PASS js: probe [packages/b]\n  bin: "+e.phys("node_modules/.bin/fakefmt")+" (local)\n")
		e.callsEqual("rec", "root\nroot")
	})

	// JS workspace priority: the declaring package's range decides.
	workspace := func(t *testing.T) *runChecksEnv {
		e := runChecksSetup(t)
		e.write("package.json", "{\"private\":true}\n")
		e.write("packages/a/package.json", `{"devDependencies":{"fakefmt":"~1.0.0"}}`+"\n")
		e.probe("js", "fakefmt", "")
		return e
	}
	t.Run("workspace: the package's own copy beats a different root copy", func(t *testing.T) {
		e := workspace(t)
		e.installed(".", "2.0.0", "root")
		own := e.installed("packages/a", "1.0.5", "own")
		e.run().Has(t, "PASS js: probe [packages/a]\n  bin: "+own+" (local)\n")
	})
	t.Run("workspace: a hoisted copy outside the package's range is skipped, not run", func(t *testing.T) {
		e := workspace(t)
		e.installed(".", "2.0.0", "root")
		r := e.run()
		r.Has(t, "SKIP js: probe [packages/a] (packages/a/package.json declares fakefmt ~1.0.0, installed is 2.0.0 at .; run npm install)")
		e.callsEqual("rec", "root")
	})
	t.Run("workspace: a hoisted copy inside the package's range runs", func(t *testing.T) {
		e := workspace(t)
		root := e.installed(".", "1.0.3", "root")
		e.run().Has(t, "PASS js: probe [packages/a]\n  bin: "+root+" (local)\n")
	})
	t.Run("workspace: Yarn PnP is asked from the declaring package", func(t *testing.T) {
		e := workspace(t)
		e.write(".pnp.cjs", "")
		where := filepath.Join(e.stubs, "where")
		Stub(t, filepath.Join(e.stubs, "yarn"), fmt.Sprintf("case \"$1\" in\nbin) pwd >>%q ;;\nrun) echo \"yarn $2\" >>%q ;;\nesac\n", where, filepath.Join(e.stubs, "rec.calls")))
		e.run().Has(t, "PASS js: probe [packages/a]\n  bin: yarn run fakefmt (package-manager env)\n")
		if got := Read(t, where); !strings.Contains(got, e.phys("packages/a")+"\n") {
			t.Errorf("yarn bin ran in:\n%s", got)
		}
	})

	// Declared but not installed: skipped with the install command, even
	// with a copy on PATH.
	t.Run("js: declared but not installed skips with pnpm's install", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", `{"devDependencies":{"fakefmt":"1.0.0"}}`+"\n")
		e.write("pnpm-lock.yaml", "")
		e.probe("js", "fakefmt", "")
		e.recorder(filepath.Join(e.stubs, "fakefmt"), "path")
		e.run().Has(t, "SKIP js: probe (fakefmt declared in package.json but not installed; run pnpm install)")
		if e.called("rec") {
			t.Errorf("ran: %s", e.calls("rec"))
		}
	})
	t.Run("python: declared but not installed skips with uv's install", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("pyproject.toml", "[dependency-groups]\ndev = [\"fakepy>=1\"]\n")
		e.write("uv.lock", "")
		e.probe("python", "fakepy", "")
		e.recorder(filepath.Join(e.stubs, "fakepy"), "path")
		e.run().Has(t, "SKIP python: probe (fakepy declared in pyproject.toml but not installed; run uv sync)")
		if e.called("rec") {
			t.Errorf("ran: %s", e.calls("rec"))
		}
	})
	t.Run("ruby: a locked gem bundler can't find skips with bundle install", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("Gemfile", "source 'https://rubygems.org'\n")
		e.write("Gemfile.lock", "GEM\n  specs:\n    fakerb (1.0)\n")
		e.probe("ruby", "fakerb", "")
		Stub(t, filepath.Join(e.stubs, "bundle"), "exit 1\n")
		e.recorder(filepath.Join(e.stubs, "fakerb"), "path")
		e.run().Has(t, "SKIP ruby: probe (fakerb declared in Gemfile.lock but not installed; run bundle install)")
	})

	// Version-manager pins: PATH is fine only when the copy is the manager's.
	t.Run("pin: an asdf shim for a .tool-versions tool runs", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		e.write(".tool-versions", "fakefmt 1.0.0\n")
		e.probe("js", "fakefmt", "")
		asdf := filepath.Join(filepath.Dir(e.stubs), "asdf")
		e.k.Setenv("ASDF_DATA_DIR", asdf)
		e.recorder(filepath.Join(asdf, "shims/fakefmt"), "shim")
		e.k.PrependPath(filepath.Join(asdf, "shims"))
		e.run().Has(t, "PASS js: probe\n  bin: "+filepath.Join(asdf, "shims/fakefmt")+" (version manager)\n")
		e.callsEqual("rec", "shim")
	})
	t.Run("pin: a mise install dir on PATH runs", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		e.write("mise.toml", "[tools]\nfakefmt = \"1.0.0\"\n")
		e.probe("js", "fakefmt", "")
		mise := filepath.Join(filepath.Dir(e.stubs), "mise")
		e.k.Setenv("MISE_DATA_DIR", mise)
		bin := filepath.Join(mise, "installs/fakefmt/1.0.0/bin")
		e.recorder(filepath.Join(bin, "fakefmt"), "mise")
		e.k.PrependPath(bin)
		e.run().Has(t, "(version manager)")
		e.callsEqual("rec", "mise")
	})
	t.Run("pin: a pinned tool's non-manager PATH copy is skipped", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		e.write(".tool-versions", "fakefmt 1.0.0\n")
		e.probe("js", "fakefmt", "")
		e.recorder(filepath.Join(e.stubs, "fakefmt"), "path")
		e.run().Has(t, "SKIP js: probe (fakefmt pinned in .tool-versions, but PATH has "+filepath.Join(e.stubs, "fakefmt")+")")
	})

	// Standalone toolchains run from PATH; rust and opentofu have no
	// project-local install, so this row covers them.
	t.Run("standalone: cargo and tofu run from PATH", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("Cargo.toml", "[package]\nname = \"x\"\n")
		e.write(".terraform.lock.hcl", "")
		e.stub("cargo", 0)
		e.stub("tofu", 0)
		e.run().Has(t,
			"PASS rust: check\n  bin: "+filepath.Join(e.stubs, "cargo")+" (PATH)\n",
			"PASS opentofu: fmt\n  bin: "+filepath.Join(e.stubs, "tofu")+" (PATH)\n")
	})

	t.Run("a toolchain check with no {bin} gets no bin line", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		e.k.Overlay("toolchain_checks:\n  - {stack: js, name: plain, cmd: \"fakeplain --probe\"}\n")
		e.stub("fakeplain", 0)
		r := e.run()
		r.Has(t, "PASS js: plain\n")
		r.Lacks(t, "PASS js: plain\n  bin:")
	})

	t.Run("ambiguous: an undeclared, unpinned PATH-only tool is skipped with the reason", func(t *testing.T) {
		e := runChecksSetup(t)
		e.write("package.json", "{}\n")
		e.probe("js", "fakefmt", "")
		e.recorder(filepath.Join(e.stubs, "fakefmt"), "path")
		e.run().Has(t, "SKIP js: probe (fakefmt only on PATH ("+filepath.Join(e.stubs, "fakefmt")+"); nothing in the project declares or pins it.")
		if e.called("rec") {
			t.Errorf("ran: %s", e.calls("rec"))
		}
	})
}
