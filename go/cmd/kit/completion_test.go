package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestZshQuote(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"explain:why a guard decides": `'explain:why a guard decides'`,
		"x:a subagent's context":      `'x:a subagent'\''s context'`,
		"x:path: a file":              `'x:path\: a file'`,
		"git-base":                    `'git-base'`,
	} {
		if got := zshQuote(in); got != want {
			t.Errorf("zshQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

// Each case sources the bash script and calls _kit as readline would, with
// the cursor on the last word, from a dir holding sub/ and file.txt.
func TestBashCompletion(t *testing.T) {
	t.Parallel()
	var script, stderr strings.Builder
	if code := cmdCompletion([]string{"bash"}, &script, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	scriptDir := t.TempDir()
	testutil.Put(t, scriptDir, "kit.bash", script.String())
	scriptPath := filepath.Join(scriptDir, "kit.bash")
	dir := t.TempDir()
	testutil.Put(t, dir, "file.txt", "")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		words []string
		want  string
	}{
		{"a command prefix completes to the commands it starts", []string{"sc"}, "scratch-dir scratch-rotate"},
		{"hook completes hook names", []string{"hook", "guard-"}, "guard-bash guard-commit guard-dispatch guard-edit guard-skills"},
		{"a command's flags complete after it", []string{"scratch-rotate", "--"}, "--dry-run"},
		{"nothing completes past a command's first argument", []string{"hook", "guard-bash", "x"}, ""},
		{"an unmatched hook prefix offers no filenames", []string{"hook", "gx"}, ""},
		{"a command without arguments offers no filenames", []string{"version", ""}, ""},
		{"a directory-only argument offers only directories", []string{"tasks", ""}, "sub"},
		{"a path argument offers files", []string{"blast-radius", "f"}, "file.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"-c", `source "$1"; shift; COMP_WORDS=(kit "$@"); COMP_CWORD=$#; COMPREPLY=(); _kit; echo "${COMPREPLY[*]}"`, "bash", scriptPath}, tt.words...)
			cmd := exec.Command("bash", args...)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			if got := strings.TrimSuffix(string(out), "\n"); got != tt.want {
				t.Errorf("COMPREPLY %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompletionMissingOrUnknownShellIsAUsageError(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"fish"}} {
		var stdout, stderr strings.Builder
		if code := cmdCompletion(args, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "usage: kit completion bash|zsh") {
			t.Errorf("args %q: exit %d, stderr %q", args, code, stderr.String())
		}
	}
}
