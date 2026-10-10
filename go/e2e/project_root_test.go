package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// `kit project-root` wiring; the root rules are tested in internal/project,
// --check and the $PWD cwd in cmd/kit.
func TestProjectRoot(t *testing.T) {
	t.Parallel()
	t.Run("prints the git toplevel from a nested subdirectory", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		tmp := t.TempDir()
		k.Git(tmp, "init", "-q", "-b", "main", filepath.Join(tmp, "repo"))
		root := Physical(t, filepath.Join(tmp, "repo"))
		k.Dir = filepath.Join(root, "src/app")
		Mkdir(t, k.Dir)
		k.Setenv("PWD", k.Dir)
		r := k.Run("", "project-root")
		r.Want(t, 0)
		if got := strings.TrimRight(r.Output, "\n"); got != root {
			t.Errorf("got %q, want %q", got, root)
		}
	})
}
