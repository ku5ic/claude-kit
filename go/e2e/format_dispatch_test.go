package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// format-dispatch's wiring: an edited file is formatted by the project's
// fixers, else by on_edit's chain, and on_edit's notes show their output.
// What enforce.Edit plans is the enforce package's tests.
//
// Formatter binaries are recording stubs: each appends "<name> <args>" to
// calls and changes nothing. The project's sit in its node_modules/.bin;
// on_edit's are on PATH. kit.yml's classifier is a stub whose answer each
// test sets, cached by kit enforce classify before the edit.
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
	for _, name := range []string{"prettier", "mdformat"} {
		e.stub(filepath.Join(e.stubs, name), name)
	}
	for _, name := range []string{"prettier", "biome"} {
		e.stub(filepath.Join(e.repo, "node_modules/.bin", name), "project-"+name)
	}
	// Stubs first, then a bash 4+ ahead of macOS /bin/bash, then git.
	path := []string{e.stubs}
	for _, tool := range []string{"bash", "git"} {
		if p, err := exec.LookPath(tool); err == nil {
			path = append(path, filepath.Dir(p))
		}
	}
	k.Setenv("PATH", strings.Join(append(path, "/usr/bin", "/bin"), ":"))
	k.Git(e.repo, "init", "-q", "-b", "main")
	Write(t, filepath.Join(e.repo, ".gitignore"), "node_modules\n")
	return e
}

// stub writes an executable that records "<label> <args>".
func (e *formatDispatchEnv) stub(path, label string) {
	Stub(e.t, path, `echo "`+label+` $*" >>"`+e.calls+"\"\n")
}

// classify caches the classifier's answer: verdicts for entries, and
// proposals.
func (e *formatDispatchEnv) classify(verdicts map[*sources.Entry]map[string]any, proposals ...map[string]any) {
	e.t.Helper()
	answers := []map[string]any{}
	for entry, v := range verdicts {
		a := map[string]any{"id": gapfill.Key(*entry), "mutates": false}
		for key, val := range v {
			a[key] = val
		}
		answers = append(answers, a)
	}
	if proposals == nil {
		proposals = []map[string]any{}
	}
	envelope, err := json.Marshal(map[string]any{"is_error": false, "subtype": "success", "structured_output": map[string]any{"entries": answers, "managers": []any{}, "proposals": proposals}})
	if err != nil {
		e.t.Fatal(err)
	}
	Write(e.t, filepath.Join(e.tmp, "answer.json"), string(envelope))
	classifier := filepath.Join(e.tmp, "classifier")
	Stub(e.t, classifier, "cat >/dev/null\ncat "+strconv.Quote(filepath.Join(e.tmp, "answer.json"))+"\n")
	e.Overlay("classifier: [" + strconv.Quote(classifier) + "]\n")
	dir := e.Dir
	e.Dir = e.repo
	defer func() { e.Dir = dir }()
	e.Run("", "enforce", "classify")
}

func (e *formatDispatchEnv) format(file string) Result {
	e.t.Helper()
	Write(e.t, file, "content\n")
	return e.Hook("format-dispatch", map[string]any{"tool_input": map[string]any{"file_path": file}})
}

// callsWant checks the recorded calls, as $(cat calls) would read them.
func (e *formatDispatchEnv) callsWant(want string) {
	e.t.Helper()
	got := ""
	if Exists(e.calls) {
		got = strings.TrimRight(Read(e.t, e.calls), "\n")
	}
	if got != want {
		e.t.Errorf("calls %q, want %q", got, want)
	}
}

func fixerProposal(command, evidence string, globs ...string) map[string]any {
	return map[string]any{"role": "fixer", "command": command, "file_form": command + " {files}", "globs": globs, "dir": ".", "evidence": evidence, "mutates": true}
}

func TestFormatDispatch(t *testing.T) {
	t.Parallel()
	t.Run("a file nothing claims is left untouched", func(t *testing.T) {
		t.Parallel()
		e := formatDispatchSetup(t)
		e.classify(nil)
		e.format(filepath.Join(e.repo, "a.ts")).Want(t, 0)
		e.callsWant("")
	})
	t.Run("the project's lint-staged fixer formats the edited file, a path with spaces as one argument", func(t *testing.T) {
		t.Parallel()
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, "package.json"), `{"lint-staged":{"*.ts":"prettier --write"}}`)
		entry := sources.Entry{Source: "lint-staged", File: "package.json", Name: "*.ts", Dir: ".", Body: sources.Body{Text: "prettier --write"}, Files: []string{"*.ts"}, PassFiles: true, Stage: "pre-commit"}
		e.classify(map[*sources.Entry]map[string]any{&entry: {"role": "fixer", "mutates": true}})
		e.format(filepath.Join(e.repo, "src", "my file.ts")).Want(t, 0)
		e.callsWant("project-prettier --write src/my file.ts")
	})
	t.Run("a Prettier config and a [tool.ruff] table each format by evidence", func(t *testing.T) {
		t.Parallel()
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, ".prettierrc"), "{}\n")
		Write(t, filepath.Join(e.repo, "pyproject.toml"), "[tool.ruff]\n")
		e.stub(filepath.Join(e.repo, ".venv/bin/ruff"), "ruff")
		Write(t, filepath.Join(e.repo, ".gitignore"), "node_modules\n.venv\n")
		e.classify(nil, fixerProposal("prettier --write", ".prettierrc", "*.ts"), fixerProposal("ruff format", "pyproject.toml", "*.py"))
		e.format(filepath.Join(e.repo, "a.ts")).Want(t, 0)
		e.format(filepath.Join(e.repo, "b.py")).Want(t, 0)
		e.callsWant("project-prettier --write a.ts\nruff format b.py")
	})
	t.Run("two formatters claiming a file leave it alone", func(t *testing.T) {
		t.Parallel()
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, ".prettierrc"), "{}\n")
		Write(t, filepath.Join(e.repo, "biome.json"), "{}\n")
		e.classify(nil, fixerProposal("prettier --write", ".prettierrc", "*.ts"), fixerProposal("biome format --write", "biome.json", "*.ts"))
		e.format(filepath.Join(e.repo, "a.ts")).Want(t, 0)
		e.callsWant("")
	})
	t.Run("unclaimed Markdown takes on_edit's first formatter on PATH", func(t *testing.T) {
		t.Parallel()
		e := formatDispatchSetup(t)
		e.classify(nil)
		e.format(filepath.Join(e.repo, "a.md")).Want(t, 0)
		e.callsWant("prettier --write " + filepath.Join(e.repo, "a.md"))
	})
	t.Run("prettierd's stdout replaces the file through a symlink, which stays one; a failure leaves it", func(t *testing.T) {
		t.Parallel()
		e := formatDispatchSetup(t)
		Stub(t, filepath.Join(e.stubs, "prettierd"), "[[ -e "+strconv.Quote(filepath.Join(e.tmp, "fail"))+" ]] && { echo partial; exit 1; }\necho formatted\n")
		e.classify(nil)
		Write(t, filepath.Join(e.repo, "AGENTS.md"), "x\n")
		if err := os.Symlink("AGENTS.md", filepath.Join(e.repo, "CLAUDE.md")); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(e.repo, "CLAUDE.md")
		e.Hook("format-dispatch", map[string]any{"tool_input": map[string]any{"file_path": link}}).Want(t, 0)
		if got := Read(t, filepath.Join(e.repo, "AGENTS.md")); got != "formatted\n" {
			t.Errorf("AGENTS.md = %q", got)
		}
		if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Error("CLAUDE.md is no longer a symlink")
		}
		Touch(t, filepath.Join(e.tmp, "fail"))
		e.format(filepath.Join(e.repo, "b.md")).Want(t, 0)
		if got := Read(t, filepath.Join(e.repo, "b.md")); got != "content\n" {
			t.Errorf("b.md = %q after a failed run", got)
		}
	})
	t.Run("a project fixer that can't run stops on_edit, and says why", func(t *testing.T) {
		t.Parallel()
		e := formatDispatchSetup(t)
		Write(t, filepath.Join(e.repo, ".prettierrc"), "{}\n")
		Write(t, filepath.Join(e.repo, "package.json"), `{"devDependencies":{"dprint":"1"}}`)
		e.classify(nil, fixerProposal("dprint fmt", ".prettierrc", "*.md"))
		e.format(filepath.Join(e.repo, "a.md")).Want(t, 0)
		e.callsWant("")
		e.Dir = e.repo
		e.Run("", "explain", "stop").Has(t, "last per-edit run:\nSKIP fix (evidence .prettierrc) (dprint declared in package.json but not installed")
	})
	t.Run("an overlay's on_edit rule for .toml runs", func(t *testing.T) {
		t.Parallel()
		e := formatDispatchSetup(t)
		e.stub(filepath.Join(e.stubs, "taplo"), "taplo")
		e.classify(nil)
		overlay := Read(t, filepath.Join(e.Claude, "claude-kit.local.yml"))
		e.Overlay(overlay + "on_edit:\n  - {globs: [\"*.toml\"], run: [{cmd: \"taplo fmt {file}\"}]}\n")
		e.format(filepath.Join(e.repo, "a.toml")).Want(t, 0)
		e.callsWant("taplo fmt " + filepath.Join(e.repo, "a.toml"))
	})
	t.Run("a shell file's shellcheck note shows its output on stderr", func(t *testing.T) {
		t.Parallel()
		e := formatDispatchSetup(t)
		Stub(t, filepath.Join(e.stubs, "shellcheck"), "echo \"SC2086 in $1\" >&2\nexit 1\n")
		e.classify(nil)
		r := e.format(filepath.Join(e.repo, "run.sh"))
		r.Want(t, 0)
		r.Has(t, "SC2086 in "+filepath.Join(e.repo, "run.sh"))
	})
}
