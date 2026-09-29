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
type formatDispatchEnv struct {
	*Kit
	tmp, stubs, calls, repo string
}

func formatDispatchSetup(t *testing.T) *formatDispatchEnv {
	t.Helper()
	k := New(t)
	tmp := Physical(t, t.TempDir())
	e := &formatDispatchEnv{Kit: k, tmp: tmp, stubs: filepath.Join(tmp, "stubs"), calls: filepath.Join(tmp, "calls")}
	for _, name := range []string{"biome", "dprint", "ruff", "black", "shfmt", "stylua", "gofmt", "taplo"} {
		e.stub(filepath.Join(e.stubs, name), name)
	}
	Stub(t, filepath.Join(e.stubs, "prettier"), `if [[ "$1" == --find-config-path ]]; then
  [[ -n "${FAKE_PRETTIER_CONFIG:-}" ]] || exit 1
  echo "$FAKE_PRETTIER_CONFIG"
  exit 0
fi
echo "prettier $*" >>"`+e.calls+"\"\n")
	// Stubs first, then a bash 4+ ahead of macOS /bin/bash, then jq and yq.
	path := []string{e.stubs}
	for _, tool := range []string{"bash", "jq", "yq"} {
		if p, err := exec.LookPath(tool); err == nil {
			path = append(path, filepath.Dir(p))
		}
	}
	k.Setenv("PATH", strings.Join(append(path, "/usr/bin", "/bin"), ":"))
	e.repo = filepath.Join(tmp, "repo")
	Mkdir(t, e.repo)
	k.Git(e.repo, "init", "-q", "-b", "main")
	return e
}

// stub writes an executable that records "<label> <args>".
func (e *formatDispatchEnv) stub(path, label string) {
	Stub(e.t, path, `echo "`+label+` $*" >>"`+e.calls+"\"\n")
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
		r := e.format(filepath.Join(e.repo, "README.md"))
		r.Want(t, 0)
		e.callsWant("")
		if got := Read(t, filepath.Join(e.repo, "README.md")); got != "content\n" {
			t.Errorf("README.md is %q", got)
		}
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
		e.format(filepath.Join(e.repo, "notes.md"))
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

	t.Run("a configured formatter that isn't installed leaves the file alone", func(t *testing.T) {
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		if err := os.Remove(filepath.Join(e.stubs, "biome")); err != nil {
			t.Fatal(err)
		}
		r := e.format(filepath.Join(e.repo, "app.ts"))
		r.Want(t, 0)
		r.Has(t, "biome is configured here but not installed")
	})
}
