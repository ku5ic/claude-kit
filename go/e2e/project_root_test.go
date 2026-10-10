package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// Characterization tests for `kit project-root`: git toplevel, the
// sentinel walk, the bare-$PWD fallback, and --check.
func TestProjectRoot(t *testing.T) {
	t.Parallel()
	output := func(r Result) string { return strings.TrimRight(r.Output, "\n") }
	// cd is bats' cd: the run's cwd, and $PWD as the logical path.
	cd := func(k *Kit, dir string) {
		k.Dir = dir
		k.Setenv("PWD", dir)
	}

	t.Run("prints the git toplevel from a nested subdirectory", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		tmp := t.TempDir()
		k.Git(tmp, "init", "-q", "-b", "main", filepath.Join(tmp, "repo"))
		root := Physical(t, filepath.Join(tmp, "repo"))
		Mkdir(t, filepath.Join(root, "src/app"))
		cd(k, filepath.Join(root, "src/app"))
		r := k.Run("", "project-root")
		r.Want(t, 0)
		if got := output(r); got != root {
			t.Errorf("got %q, want %q", got, root)
		}
	})
	t.Run("--check inside a repo exits 0 and prints nothing", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		tmp := t.TempDir()
		k.Git(tmp, "init", "-q", "-b", "main", filepath.Join(tmp, "repo"))
		Mkdir(t, filepath.Join(tmp, "repo/src"))
		cd(k, filepath.Join(tmp, "repo/src"))
		r := k.Run("", "project-root", "--check")
		r.Want(t, 0)
		r.Empty(t)
	})
	t.Run("outside a repo, an anchor sentinel two levels up is the root", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		tmp := t.TempDir()
		Mkdir(t, filepath.Join(tmp, "proj/a/b"))
		Touch(t, filepath.Join(tmp, "proj/package.json"))
		cd(k, filepath.Join(tmp, "proj/a/b"))
		r := k.Run("", "project-root")
		r.Want(t, 0)
		if got, want := output(r), filepath.Join(tmp, "proj"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		k.Run("", "project-root", "--check").Want(t, 0)
	})
	t.Run("outside a repo with no sentinel, prints $PWD and --check exits 1", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		dir := filepath.Join(t.TempDir(), "plain/a/b")
		Mkdir(t, dir)
		cd(k, dir)
		r := k.Run("", "project-root")
		r.Want(t, 0)
		if got := output(r); got != dir {
			t.Errorf("got %q, want %q", got, dir)
		}
		r = k.Run("", "project-root", "--check")
		r.Want(t, 1)
		r.Empty(t)
	})
}
