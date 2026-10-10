package e2e

import (
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
		// Ignored, so it isn't uncommitted work that keeps the worktree.
		Write(t, filepath.Join(repo, ".git/info/exclude"), "old.txt\n")
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
		r := k.Run("", "scratch-rotate", "30", "--dry-run")
		r.Has(t, "would-delete review worktree older than 1d "+wt)
		kept(t, filepath.Join(wt, "old.txt"))
	})
	t.Run("keeps an old review worktree holding uncommitted work, and dry-run says so", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		_, wt := worktree(t, k, registry, "review-pr-7", 3*day)
		Write(t, filepath.Join(wt, "NOTES.md"), "mine\n")
		for _, args := range [][]string{{"30", "--dry-run"}, {"30"}} {
			r := k.Run("", append([]string{"scratch-rotate"}, args...)...)
			r.Want(t, 0)
			r.Has(t, "kept review worktree with uncommitted changes "+wt)
		}
		kept(t, filepath.Join(wt, "NOTES.md"))
	})
	t.Run("keeps a locked review worktree, and dry-run says so", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		repo, wt := worktree(t, k, registry, "review-pr-7", 3*day)
		k.Git(repo, "worktree", "lock", wt)
		for _, args := range [][]string{{"30", "--dry-run"}, {"30"}} {
			r := k.Run("", append([]string{"scratch-rotate"}, args...)...)
			r.Want(t, 0)
			r.Has(t, "kept locked review worktree "+wt)
		}
		kept(t, filepath.Join(wt, ".git"))
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
