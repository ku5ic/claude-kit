package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// formatEnv is one format-dispatch case. Formatter binaries are recording
// stubs: each appends "<name> <args>" to calls and changes nothing. The
// project's sit in its node_modules/.bin; on_edit's are on PATH.
type formatEnv struct {
	t                       *testing.T
	k                       *sandbox
	tmp, stubs, calls, repo string
}

// formatSetup sets PATH to the stubs, then a bash 4+ ahead of macOS
// /bin/bash, then git.
func formatSetup(t *testing.T) *formatEnv {
	t.Helper()
	tmp := testutil.Physical(t, t.TempDir())
	e := &formatEnv{t: t, k: newSandbox(t), tmp: tmp, stubs: filepath.Join(tmp, "stubs"), calls: filepath.Join(tmp, "calls"), repo: filepath.Join(tmp, "repo")}
	for _, name := range []string{"prettier", "mdformat"} {
		e.stub(filepath.Join(e.stubs, name), name)
	}
	for _, name := range []string{"prettier", "biome"} {
		e.stub(filepath.Join(e.repo, "node_modules/.bin", name), "project-"+name)
	}
	path := []string{e.stubs}
	for _, tool := range []string{"bash", "git"} {
		if p, err := exec.LookPath(tool); err == nil {
			path = append(path, filepath.Dir(p))
		}
	}
	t.Setenv("PATH", strings.Join(append(path, "/usr/bin", "/bin"), ":"))
	testutil.Git(t, e.repo, "init", "-q", "-b", "main")
	testutil.Write(t, filepath.Join(e.repo, ".gitignore"), "node_modules\n")
	return e
}

// stub writes an executable that records "<label> <args>".
func (e *formatEnv) stub(path, label string) {
	stub(e.t, path, `echo "`+label+` $*" >>"`+e.calls+"\"\n")
}

func (e *formatEnv) format(file string) result {
	e.t.Helper()
	testutil.Write(e.t, file, "content\n")
	return e.k.run("format-dispatch", map[string]any{"tool_input": map[string]any{"file_path": file}})
}

// callsWant checks the recorded calls, trailing newlines trimmed.
func (e *formatEnv) callsWant(want string) {
	e.t.Helper()
	if got := strings.TrimRight(read(e.t, e.calls), "\n"); got != want {
		e.t.Errorf("calls %q, want %q", got, want)
	}
}

func fixerProposal(command, evidence string, globs ...string) map[string]any {
	return map[string]any{"role": "fixer", "command": command, "file_form": command + " {files}", "globs": globs, "dir": ".", "evidence": evidence, "mutates": true}
}

// Not parallel: each case sets PATH.
func TestFormatDispatch(t *testing.T) {
	t.Run("a file nothing claims is left untouched", func(t *testing.T) {
		e := formatSetup(t)
		e.k.classify(e.repo, envelope(t, nil, nil, nil))
		e.format(filepath.Join(e.repo, "a.ts")).Want(t, 0)
		e.callsWant("")
	})
	t.Run("the project's lint-staged fixer formats the edited file, a path with spaces as one argument", func(t *testing.T) {
		e := formatSetup(t)
		testutil.Write(t, filepath.Join(e.repo, "package.json"), `{"lint-staged":{"*.ts":"prettier --write"}}`)
		entry := sources.Entry{Source: "lint-staged", File: "package.json", Name: "*.ts", Dir: ".", Body: sources.Body{Text: "prettier --write"}, Files: []string{"*.ts"}, PassFiles: true, Stage: "pre-commit"}
		e.k.classify(e.repo, envelope(t, []map[string]any{verdict(entry, map[string]any{"role": "fixer", "mutates": true})}, nil, nil))
		e.format(filepath.Join(e.repo, "src", "my file.ts")).Want(t, 0)
		e.callsWant("project-prettier --write src/my file.ts")
	})
	t.Run("a Prettier config and a [tool.ruff] table each format by evidence", func(t *testing.T) {
		e := formatSetup(t)
		testutil.Write(t, filepath.Join(e.repo, ".prettierrc"), "{}\n")
		testutil.Write(t, filepath.Join(e.repo, "pyproject.toml"), "[tool.ruff]\n")
		e.stub(filepath.Join(e.repo, ".venv/bin/ruff"), "ruff")
		testutil.Write(t, filepath.Join(e.repo, ".gitignore"), "node_modules\n.venv\n")
		e.k.classify(e.repo, envelope(t, nil, nil, []map[string]any{fixerProposal("prettier --write", ".prettierrc", "*.ts"), fixerProposal("ruff format", "pyproject.toml", "*.py")}))
		e.format(filepath.Join(e.repo, "a.ts")).Want(t, 0)
		e.format(filepath.Join(e.repo, "b.py")).Want(t, 0)
		e.callsWant("project-prettier --write a.ts\nruff format b.py")
	})
	t.Run("two formatters claiming a file leave it alone", func(t *testing.T) {
		e := formatSetup(t)
		testutil.Write(t, filepath.Join(e.repo, ".prettierrc"), "{}\n")
		testutil.Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		e.k.classify(e.repo, envelope(t, nil, nil, []map[string]any{fixerProposal("prettier --write", ".prettierrc", "*.ts"), fixerProposal("biome format --write", "biome.json", "*.ts")}))
		e.format(filepath.Join(e.repo, "a.ts")).Want(t, 0)
		e.callsWant("")
	})
	t.Run("prettierd's stdout replaces the file through a symlink, which stays one; a failure leaves it", func(t *testing.T) {
		e := formatSetup(t)
		stub(t, filepath.Join(e.stubs, "prettierd"), "[[ -e "+strconv.Quote(filepath.Join(e.tmp, "fail"))+" ]] && { echo partial; exit 1; }\necho formatted\n")
		e.k.classify(e.repo, envelope(t, nil, nil, nil))
		testutil.Write(t, filepath.Join(e.repo, "AGENTS.md"), "x\n")
		link := filepath.Join(e.repo, "CLAUDE.md")
		if err := os.Symlink("AGENTS.md", link); err != nil {
			t.Fatal(err)
		}
		e.k.run("format-dispatch", map[string]any{"tool_input": map[string]any{"file_path": link}}).Want(t, 0)
		if got := read(t, filepath.Join(e.repo, "AGENTS.md")); got != "formatted\n" {
			t.Errorf("AGENTS.md = %q", got)
		}
		if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Error("CLAUDE.md is no longer a symlink")
		}
		testutil.Touch(t, filepath.Join(e.tmp, "fail"))
		e.format(filepath.Join(e.repo, "b.md")).Want(t, 0)
		if got := read(t, filepath.Join(e.repo, "b.md")); got != "content\n" {
			t.Errorf("b.md = %q after a failed run", got)
		}
	})
	t.Run("a project fixer that can't run stops on_edit, and says why", func(t *testing.T) {
		e := formatSetup(t)
		testutil.Write(t, filepath.Join(e.repo, ".prettierrc"), "{}\n")
		testutil.Write(t, filepath.Join(e.repo, "package.json"), `{"devDependencies":{"dprint":"1"}}`)
		e.k.classify(e.repo, envelope(t, nil, nil, []map[string]any{fixerProposal("dprint fmt", ".prettierrc", "*.md")}))
		e.format(filepath.Join(e.repo, "a.md")).Want(t, 0)
		e.callsWant("")
		// What kit explain stop prints as the last per-edit run.
		if report := read(t, cache.EditReport(e.k.paths.CacheDir(), e.repo)); !strings.HasPrefix(report, "SKIP fix (evidence .prettierrc) (dprint declared in package.json but not installed") {
			t.Errorf("per-edit report:\n%s", report)
		}
	})
	t.Run("an overlay's on_edit rule for .toml runs", func(t *testing.T) {
		e := formatSetup(t)
		e.stub(filepath.Join(e.stubs, "taplo"), "taplo")
		e.k.classify(e.repo, envelope(t, nil, nil, nil))
		e.k.overlay(read(t, e.k.paths.Overlay) + "on_edit:\n  - {globs: [\"*.toml\"], run: [{cmd: \"taplo fmt {file}\"}]}\n")
		e.format(filepath.Join(e.repo, "a.toml")).Want(t, 0)
		e.callsWant("taplo fmt " + filepath.Join(e.repo, "a.toml"))
	})
	t.Run("a shell file's shellcheck note shows its output on stderr", func(t *testing.T) {
		e := formatSetup(t)
		stub(t, filepath.Join(e.stubs, "shellcheck"), "echo \"SC2086 in $1\" >&2\nexit 1\n")
		e.k.classify(e.repo, envelope(t, nil, nil, nil))
		r := e.format(filepath.Join(e.repo, "run.sh"))
		r.Want(t, 0)
		if want := "SC2086 in " + filepath.Join(e.repo, "run.sh"); !strings.Contains(r.Stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.Stderr)
		}
	})
}
