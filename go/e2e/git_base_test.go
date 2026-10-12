package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// `kit git-base` wiring; the detection order and the --diff and --log modes
// are tested in internal/gitbase.
func TestGitBase(t *testing.T) {
	t.Parallel()
	t.Run("with no remote, a local main is used", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		dir := filepath.Join(Physical(t, t.TempDir()), "local")
		Mkdir(t, dir)
		k.Git(dir, "init", "-q", "-b", "main")
		k.Git(dir, "commit", "-q", "--allow-empty", "-m", "init")
		k.Git(dir, "switch", "-q", "-c", "feat")
		k.Dir = dir
		r := k.Run("", "git-base")
		r.Want(t, 0)
		if got := strings.TrimRight(r.Output, "\n"); got != "main" {
			t.Errorf("got %q", got)
		}
	})
}
