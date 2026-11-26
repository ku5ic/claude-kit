package e2e

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// Tests for `kit scratch-rotate`: one wiring case, then the safety cases
// (what it must never delete, what it refuses, dry runs), which stay on the
// binary on purpose. Retention is tested in internal/rotate.
//
// The command's home-fallback target dir is $HOME/.claude/scratch and its
// registry $HOME/.claude/logs/scratch-registry.txt, so every test fakes
// $HOME to keep it off the real ones. Ages are set with backdate rather
// than sleeping past a retention window.
func TestScratchRotate(t *testing.T) {
	t.Parallel()
	const day = 86400
	setup := func(t *testing.T) (k *Kit, scratch, registry string) {
		k = New(t)
		scratch = filepath.Join(k.Claude, "scratch")
		registry = filepath.Join(k.Claude, "logs/scratch-registry.txt")
		Mkdir(t, scratch)
		return k, scratch, registry
	}
	// age sets path's mtime to secs seconds ago.
	age := func(t *testing.T, path string, secs int) {
		t.Helper()
		testutil.Age(t, time.Duration(secs)*time.Second, path)
	}
	// touch creates an empty file whose mtime is secs seconds ago.
	touch := func(t *testing.T, path string, secs int) {
		t.Helper()
		Touch(t, path)
		age(t, path, secs)
	}
	gone := func(t *testing.T, path string) {
		t.Helper()
		if Exists(path) {
			t.Errorf("%s survived", path)
		}
	}
	kept := func(t *testing.T, path string) {
		t.Helper()
		if !Exists(path) {
			t.Errorf("%s was pruned", path)
		}
	}
	registryIs := func(t *testing.T, registry, want string) {
		t.Helper()
		if got := strings.TrimRight(Read(t, registry), "\n"); got != want {
			t.Errorf("registry %q, want %q", got, want)
		}
	}

	t.Run("prunes an .md artifact older than the retention window", func(t *testing.T) {
		t.Parallel()
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, "old.md"), 40*day)
		r := k.Run("", "scratch-rotate", "30")
		r.Want(t, 0)
		r.Has(t, "pruned 1 artifact(s)")
		gone(t, filepath.Join(scratch, "old.md"))
	})
	t.Run("0 empties scratch of fresh files too", func(t *testing.T) {
		t.Parallel()
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, "fresh.json"), 60)
		r := k.Run("", "scratch-rotate", "0")
		r.Want(t, 0)
		r.Has(t, "pruned 1 artifact(s) older than 0d")
		gone(t, filepath.Join(scratch, "fresh.json"))
	})
	t.Run("--dry-run reports what would go and deletes nothing", func(t *testing.T) {
		t.Parallel()
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, "old.md"), 40*day)
		r := k.Run("", "scratch-rotate", "--dry-run")
		r.Want(t, 0)
		r.Has(t, "would prune 1 artifact(s)")
		r.Lacks(t, "scratch-rotate: pruned")
		kept(t, filepath.Join(scratch, "old.md"))
	})
	// worktree adds a detached review worktree under a registered project
	// scratch dir, created secs seconds ago, holding a 40-day-old file.
	worktree := func(t *testing.T, k *Kit, registry, name string, secs int) (repo, wt string) {
		t.Helper()
		repo = k.Repo(filepath.Join(k.Home, "proj"))
		scratch := filepath.Join(repo, ".claude/scratch")
		Mkdir(t, scratch)
		Write(t, registry, scratch+"\n")
		wt = filepath.Join(scratch, name)
		k.Git(repo, "worktree", "add", "-q", "--detach", wt)
		touch(t, filepath.Join(wt, "old.txt"), 40*day)
		age(t, filepath.Join(wt, ".git"), secs)
		return repo, wt
	}
	t.Run("keeps a fresh review worktree and never prunes inside it", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		_, wt := worktree(t, k, registry, "review-pr-7", 3600)
		k.Run("", "scratch-rotate", "30").Want(t, 0)
		kept(t, filepath.Join(wt, "old.txt"))
		kept(t, filepath.Join(wt, ".git"))
	})
	t.Run("--dry-run keeps an old review worktree and says it would go", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		_, wt := worktree(t, k, registry, "review-pr-7", 3*day)
		_ = filepath.WalkDir(wt, func(path string, _ fs.DirEntry, _ error) error { age(t, path, 3*day); return nil })
		r := k.Run("", "scratch-rotate", "30", "--dry-run")
		r.Want(t, 0)
		r.Has(t, "would-delete checkout older than 1d "+wt)
		if log := Read(t, filepath.Join(k.Claude, "logs/scratch-rotate.log")); !strings.Contains(log, " would-delete "+wt+"\n") {
			t.Errorf("the rotate log doesn't say %s would go:\n%s", wt, log)
		}
		kept(t, filepath.Join(wt, "old.txt"))
	})
	t.Run("removes an old worktree holding uncommitted work or locked, review or not: scratch is throwaway", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"review-pr-7", "wt-7"} {
			for _, locked := range []bool{false, true} {
				k, _, registry := setup(t)
				repo, wt := worktree(t, k, registry, name, 3*day)
				Write(t, filepath.Join(wt, "NOTES.md"), "mine\n")
				if locked {
					k.Git(repo, "worktree", "lock", wt)
				}
				_ = filepath.WalkDir(wt, func(path string, _ fs.DirEntry, _ error) error { age(t, path, 40*day); return nil })
				k.Run("", "scratch-rotate", "30", "--dry-run").Want(t, 0)
				kept(t, filepath.Join(wt, "NOTES.md"))
				k.Run("", "scratch-rotate", "30").Want(t, 0)
				gone(t, wt)
				if list := k.Git(repo, "worktree", "list", "--porcelain"); strings.Contains(list, wt) {
					t.Errorf("%s locked=%v: the repo still lists %s", name, locked, wt)
				}
			}
		}
	})
	t.Run("removes an old worktree that isn't a review one, and its repo forgets it", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		repo, wt := worktree(t, k, registry, "kit-before", 3*day)
		k.Run("", "scratch-rotate", "0").Want(t, 0)
		if _, err := os.Lstat(wt); err == nil {
			t.Errorf("%s survived", wt)
		}
		if list := k.Git(repo, "worktree", "list", "--porcelain"); strings.Contains(list, wt) {
			t.Errorf("the repo still lists %s:\n%s", wt, list)
		}
	})
	t.Run("--dry-run keeps a stale registry entry and says it would drop it", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		proj := filepath.Join(Physical(t, t.TempDir()), "proj/scratch")
		Write(t, registry, proj+"\n")
		r := k.Run("", "scratch-rotate", "--dry-run")
		r.Want(t, 0)
		r.Has(t, "would drop stale registry entry "+proj)
		registryIs(t, registry, proj)
	})
	// A registry line is untrusted input, so anything outside $HOME is
	// refused rather than pruned.
	t.Run("refuses a registered dir outside HOME instead of pruning it", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		outside := filepath.Join(Physical(t, t.TempDir()), "outside/scratch")
		touch(t, filepath.Join(outside, "old.py"), 40*day)
		Write(t, registry, outside+"\n")
		r := k.Run("", "scratch-rotate", "30")
		r.Want(t, 0)
		r.Has(t, "REFUSING registry entry "+outside)
		kept(t, filepath.Join(outside, "old.py"))
		registryIs(t, registry, outside)
	})
}
