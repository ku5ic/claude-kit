package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `kit scratch-dir` wiring; the tier rules are tested in internal/project,
// the arguments and CLAUDE_CONFIG_DIR in cmd/kit.
func TestScratchDir(t *testing.T) {
	t.Parallel()
	t.Run("kit scratch-dir inside a repo prints and creates the project tier", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		tmp := Physical(t, t.TempDir())
		repo := filepath.Join(tmp, "repo")
		k.Git(tmp, "init", "-q", "-b", "main", repo)
		k.Dir = filepath.Join(repo, "src")
		Mkdir(t, k.Dir)
		k.Setenv("PWD", k.Dir)
		r := k.Run("", "scratch-dir")
		r.Want(t, 0)
		want := filepath.Join(repo, ".claude/scratch")
		if got := strings.TrimRight(r.Output, "\n"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if fi, err := os.Stat(want); err != nil || !fi.IsDir() {
			t.Errorf("%s not created", want)
		}
	})
}
