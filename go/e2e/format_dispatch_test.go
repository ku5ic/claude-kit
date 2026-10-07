package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Each file is formatted by the one formatter its project opts into, per
// kit.yml's formatters table.
//
// Formatter binaries are recording stubs: each appends "<name> <args>" to
// calls and changes nothing. The stub prettier answers --find-config-path
// with $FAKE_PRETTIER_CONFIG, so tests choose where "its" config lives.
// Tools the project would install sit in the repo's node_modules/.bin or
// .venv/bin; the standalone ones (path_fallback) and prettier, for the
// Markdown fallback, are on PATH too.
type formatDispatchEnv struct {
	*Kit
	tmp, stubs, calls, repo string
}

func formatDispatchSetup(t *testing.T) *formatDispatchEnv {
	t.Helper()
	k := New(t)
	tmp := Physical(t, t.TempDir())
	e := &formatDispatchEnv{Kit: k, tmp: tmp, stubs: filepath.Join(tmp, "stubs"), calls: filepath.Join(tmp, "calls")}
	e.repo = filepath.Join(tmp, "repo")
	for _, name := range []string{"biome", "dprint", "ruff", "black", "shfmt", "stylua", "gofmt", "taplo"} {
		e.stub(filepath.Join(e.stubs, name), name)
	}
	for _, name := range []string{"biome", "dprint", "taplo"} {
		e.stub(filepath.Join(e.repo, "node_modules/.bin", name), name)
	}
	for _, name := range []string{"ruff", "black"} {
		e.stub(filepath.Join(e.repo, ".venv/bin", name), name)
	}
	e.prettier(filepath.Join(e.stubs, "prettier"))
	e.prettier(filepath.Join(e.repo, "node_modules/.bin/prettier"))
	// Stubs first, then a bash 4+ ahead of macOS /bin/bash, then jq and yq.
	path := []string{e.stubs}
	for _, tool := range []string{"bash", "jq", "yq"} {
		if p, err := exec.LookPath(tool); err == nil {
			path = append(path, filepath.Dir(p))
		}
	}
	k.Setenv("PATH", strings.Join(append(path, "/usr/bin", "/bin"), ":"))
	k.Git(e.repo, "init", "-q", "-b", "main")
	return e
}

// stub writes an executable that records "<label> <args>".
func (e *formatDispatchEnv) stub(path, label string) {
	Stub(e.t, path, `echo "`+label+` $*" >>"`+e.calls+"\"\n")
}

// prettier writes the recording prettier stub that also answers
// --find-config-path.
func (e *formatDispatchEnv) prettier(path string) {
	Stub(e.t, path, `if [[ "$1" == --find-config-path ]]; then
  [[ -n "${FAKE_PRETTIER_CONFIG:-}" ]] || exit 1
  echo "$FAKE_PRETTIER_CONFIG"
  exit 0
fi
echo "prettier $*" >>"`+e.calls+"\"\n")
}

// remove deletes the given files, failing the test on any error.
func (e *formatDispatchEnv) remove(paths ...string) {
	for _, p := range paths {
		if err := os.Remove(p); err != nil {
			e.t.Fatal(err)
		}
	}
}

func (e *formatDispatchEnv) format(file string) Result {
	e.t.Helper()
	Write(e.t, file, "content\n")
	return e.Hook("format-dispatch", map[string]any{"tool_input": map[string]any{"file_path": file}})
}

// callsWant checks the recorded calls, as $(cat calls) would read them.
func (e *formatDispatchEnv) callsWant(want string) {
	e.t.Helper()
	if got := strings.TrimRight(Read(e.t, e.calls), "\n"); got != want {
		e.t.Errorf("calls %q, want %q", got, want)
	}
}

func TestFormatDispatch(t *testing.T) {
	t.Run("no formatter signal leaves the file untouched", func(t *testing.T) {
		e := formatDispatchSetup(t)
		r := e.format(filepath.Join(e.repo, "app.ts"))
		r.Want(t, 0)
		e.callsWant("")
		if got := Read(t, filepath.Join(e.repo, "app.ts")); got != "content\n" {
			t.Errorf("app.ts is %q", got)
		}
	})

	// Markdown is the one opinionated exception: with no formatter claiming
	// it, the first fallback that resolves formats it, PATH allowed.
	t.Run("unclaimed Markdown is formatted by the fallback, from PATH", func(t *testing.T) {
		e := formatDispatchSetup(t)
		e.remove(filepath.Join(e.repo, "node_modules/.bin/prettier"))
		r := e.format(filepath.Join(e.repo, "README.md"))
		r.Want(t, 0)
		e.callsWant("prettier --write " + e.repo + "/README.md")
	})
	t.Run("prettierd comes first, and its stdout replaces the file", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Stub(t, filepath.Join(e.stubs, "prettierd"), `cat >/dev/null
echo "prettierd $*" >>"`+e.calls+`"
echo formatted
`)
		e.format(filepath.Join(e.repo, "README.md"))
		e.callsWant("prettierd " + e.repo + "/README.md")
		if got := Read(t, filepath.Join(e.repo, "README.md")); got != "formatted\n" {
			t.Errorf("README.md is %q", got)
		}
	})
	t.Run("prettierd's output goes through a symlink, which stays a symlink", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Stub(t, filepath.Join(e.stubs, "prettierd"), "cat >/dev/null\necho formatted\n")
		Write(t, filepath.Join(e.repo, "AGENTS.md"), "content\n")
		if err := os.Symlink("AGENTS.md", filepath.Join(e.repo, "CLAUDE.md")); err != nil {
			t.Fatal(err)
		}
		e.Hook("format-dispatch", map[string]any{"tool_input": map[string]any{"file_path": filepath.Join(e.repo, "CLAUDE.md")}})
		if link, err := os.Readlink(filepath.Join(e.repo, "CLAUDE.md")); err != nil || link != "AGENTS.md" {
			t.Errorf("CLAUDE.md is no longer the symlink: %q %v", link, err)
		}
		if got := Read(t, filepath.Join(e.repo, "AGENTS.md")); got != "formatted\n" {
			t.Errorf("AGENTS.md is %q", got)
		}
	})
	t.Run("a declared but uninstalled prettier with a config blocks the Markdown fallback", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "package.json"), `{"devDependencies":{"prettier":"^3"}}`+"\n")
		Touch(t, filepath.Join(e.repo, ".prettierrc"))
		e.remove(filepath.Join(e.repo, "node_modules/.bin/prettier"))
		Stub(t, filepath.Join(e.stubs, "prettierd"), `echo "prettierd $*" >>"`+e.calls+"\"\n")
		r := e.format(filepath.Join(e.repo, "README.md"))
		r.Has(t, "prettier is configured here but can't run: prettier declared in package.json but not installed")
		e.callsWant("")
	})
	t.Run("a configured Biome that can't run also names a declared prettier that can't", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "package.json"), `{"devDependencies":{"prettier":"^3"}}`+"\n")
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		e.remove(filepath.Join(e.repo, "node_modules/.bin/prettier"))
		e.remove(filepath.Join(e.repo, "node_modules/.bin/biome"))
		r := e.format(filepath.Join(e.repo, "app.ts"))
		r.Has(t, "biome is configured here but can't run: ", "(prettier is also declared here but can't run: prettier declared in package.json but not installed")
		e.callsWant("")
	})
	t.Run("a stale prettier dependency doesn't stop a configured Biome", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "package.json"), `{"devDependencies":{"prettier":"^3"}}`+"\n")
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		e.remove(filepath.Join(e.repo, "node_modules/.bin/prettier"))
		e.format(filepath.Join(e.repo, "app.ts"))
		e.callsWant("biome format --write " + e.repo + "/app.ts")
	})
	t.Run("a prettierd failure leaves the file as it was", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Stub(t, filepath.Join(e.stubs, "prettierd"), "cat >/dev/null\necho partial\nexit 2\n")
		e.format(filepath.Join(e.repo, "README.md"))
		if got := Read(t, filepath.Join(e.repo, "README.md")); got != "content\n" {
			t.Errorf("README.md is %q", got)
		}
	})
	t.Run("a project formatter claiming Markdown beats the fallback", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "dprint.json"), "{}\n")
		e.format(filepath.Join(e.repo, "README.md"))
		e.callsWant("dprint fmt " + e.repo + "/README.md")
	})
	t.Run("a declared but uninstalled prettier stops the fallback chain with its reason", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "package.json"), `{"devDependencies":{"prettier":"^3.6.0"}}`+"\n")
		e.remove(filepath.Join(e.repo, "node_modules/.bin/prettier"))
		e.stub(filepath.Join(e.stubs, "deno"), "deno")
		r := e.format(filepath.Join(e.repo, "README.md"))
		r.Want(t, 0)
		r.Has(t, "prettier declared in package.json but not installed; run npm install")
		e.callsWant("")
	})
	t.Run("Markdown is left alone when no fallback is installed", func(t *testing.T) {
		e := formatDispatchSetup(t)
		e.remove(filepath.Join(e.repo, "node_modules/.bin/prettier"), filepath.Join(e.stubs, "prettier"))
		r := e.format(filepath.Join(e.repo, "README.md"))
		r.Want(t, 0)
		e.callsWant("")
	})

	t.Run("a Biome-only repo runs Biome, not Prettier", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		r := e.format(filepath.Join(e.repo, "src", "app.ts"))
		r.Want(t, 0)
		e.callsWant("biome format --write " + e.repo + "/src/app.ts")
	})

	t.Run("Biome and Prettier both configured: untouched, with a notice", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		Touch(t, filepath.Join(e.repo, ".prettierrc"))
		e.Setenv("FAKE_PRETTIER_CONFIG", filepath.Join(e.repo, ".prettierrc"))
		r := e.format(filepath.Join(e.repo, "app.ts"))
		r.Want(t, 0)
		e.callsWant("")
		r.Has(t, "biome prettier are all configured")
	})

	t.Run("a Prettier config in the project formats with Prettier", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Touch(t, filepath.Join(e.repo, ".prettierrc"))
		e.Setenv("FAKE_PRETTIER_CONFIG", ".prettierrc")
		e.format(filepath.Join(e.repo, "notes.md"))
		e.callsWant("prettier --write " + e.repo + "/notes.md")
	})

	t.Run("only a ~/.prettierrc leaves the file untouched", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Touch(t, filepath.Join(e.Home, ".prettierrc"))
		e.Setenv("FAKE_PRETTIER_CONFIG", filepath.Join(e.Home, ".prettierrc"))
		e.format(filepath.Join(e.repo, "app.ts"))
		e.callsWant("")
	})

	t.Run("a project-local binary wins over the one on PATH", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		e.stub(filepath.Join(e.repo, "node_modules", ".bin", "biome"), "local-biome")
		e.format(filepath.Join(e.repo, "app.ts"))
		e.callsWant("local-biome format --write " + e.repo + "/app.ts")
	})

	t.Run("a bare [tool.ruff] table picks Ruff over Black", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "pyproject.toml"), "[tool.ruff]\n")
		e.format(filepath.Join(e.repo, "app.py"))
		e.callsWant("ruff format " + e.repo + "/app.py")
	})

	t.Run("shfmt runs with no indent flag, so .editorconfig decides", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, ".editorconfig"), "root = true\n")
		e.format(filepath.Join(e.repo, "run.sh"))
		e.callsWant("shfmt -w " + e.repo + "/run.sh")
	})

	t.Run("a signal above the project root is ignored", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.tmp, ".editorconfig"), "root = true\n")
		e.format(filepath.Join(e.repo, "run.sh"))
		e.callsWant("")
	})

	t.Run("a path with spaces stays one argument", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		e.format(filepath.Join(e.repo, "my dir", "app.ts"))
		e.callsWant("biome format --write " + e.repo + "/my dir/app.ts")
	})

	t.Run("disabled_formatters in the overlay turns one off", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		e.Overlay("disabled_formatters: [biome]\n")
		e.format(filepath.Join(e.repo, "app.ts"))
		e.callsWant("")
	})

	t.Run("an overlay-only formatter for .toml runs", func(t *testing.T) {
		e := formatDispatchSetup(t)
		e.Overlay(`formatters:
  - name: taplo
    ext: [toml]
    signal_files: [taplo.toml]
    bin: taplo
    cmd: "{bin} format {file}"
`)
		Touch(t, filepath.Join(e.repo, "taplo.toml"))
		e.format(filepath.Join(e.repo, "Cargo.toml"))
		e.callsWant("taplo format " + e.repo + "/Cargo.toml")
	})

	t.Run("a configured formatter only on PATH, undeclared and unpinned, can't run", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		e.remove(filepath.Join(e.repo, "node_modules/.bin/biome"))
		r := e.format(filepath.Join(e.repo, "app.ts"))
		r.Want(t, 0)
		r.Has(t, "biome is configured here but can't run: biome only on PATH (")
		e.callsWant("")
	})

	t.Run("a pinned formatter runs from the version manager's shims", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		Write(t, filepath.Join(e.repo, ".tool-versions"), "biome 2.0.0\n")
		e.remove(filepath.Join(e.repo, "node_modules/.bin/biome"))
		asdf := filepath.Join(e.tmp, "asdf")
		e.stub(filepath.Join(asdf, "shims/biome"), "shim-biome")
		e.Setenv("ASDF_DATA_DIR", asdf)
		e.PrependPath(filepath.Join(asdf, "shims"))
		e.format(filepath.Join(e.repo, "app.ts"))
		e.callsWant("shim-biome format --write " + e.repo + "/app.ts")
	})

	t.Run("a configured formatter that isn't installed leaves the file alone", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		e.remove(filepath.Join(e.stubs, "biome"), filepath.Join(e.repo, "node_modules/.bin/biome"))
		r := e.format(filepath.Join(e.repo, "app.ts"))
		r.Want(t, 0)
		r.Has(t, "biome is configured here but not installed")
	})
}
