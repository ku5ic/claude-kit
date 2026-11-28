package resolve

import (
	"os"
	"os/exec"
	"path/filepath"
)

// BashToolShell is the shell Claude Code runs Bash tool commands in, found
// as it finds it (code.claude.com/docs/en/env-vars.md, CLAUDE_CODE_SHELL):
// that variable when it's a working bash or zsh, else $SHELL when it is,
// else the first zsh, then bash, on PATH; "" when there's none.
func BashToolShell() string {
	for _, shell := range []string{os.Getenv("CLAUDE_CODE_SHELL"), os.Getenv("SHELL")} {
		if name := filepath.Base(shell); name == "bash" || name == "zsh" {
			if _, err := exec.LookPath(shell); err == nil {
				return shell
			}
		}
	}
	for _, name := range []string{"zsh", "bash"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}
