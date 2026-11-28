package resolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// The Bash tool's shell is found the way Claude Code finds it
// (code.claude.com/docs/en/env-vars.md, CLAUDE_CODE_SHELL): that variable
// when it's a working bash or zsh, else $SHELL when it is, else the first
// zsh, then bash, on PATH.
func TestBashToolShellFollowsClaudeCode(t *testing.T) {
	exe := func(dir, name string) string {
		path := filepath.Join(dir, name)
		testutil.Write(t, path, "#!/bin/sh\n")
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	both, bashOnly, none := t.TempDir(), t.TempDir(), t.TempDir()
	zsh, bash, fish := exe(both, "zsh"), exe(both, "bash"), exe(both, "fish")
	onlyBash := exe(bashOnly, "bash")
	cases := []struct{ name, override, shell, path, want string }{
		{"the override wins", bash, zsh, both, bash},
		{"a missing override falls back to $SHELL", filepath.Join(none, "zsh"), bash, both, bash},
		{"an override that isn't bash or zsh is ignored", fish, bash, both, bash},
		{"a fish $SHELL falls back to zsh on PATH", "", fish, both, zsh},
		{"with no zsh on PATH, bash", "", fish, bashOnly, onlyBash},
		{"with neither on PATH, none", "", fish, none, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_SHELL", c.override)
			t.Setenv("SHELL", c.shell)
			t.Setenv("PATH", c.path)
			if got := BashToolShell(); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
