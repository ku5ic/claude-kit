package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// Tests for `kit scratch-rotate`.
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
	t.Run("keeps an .md artifact newer than the retention window", func(t *testing.T) {
		t.Parallel()
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, "fresh.md"), 3600)
		r := k.Run("", "scratch-rotate", "30")
		r.Want(t, 0)
		r.Has(t, "pruned 0 artifact(s)")
		kept(t, filepath.Join(scratch, "fresh.md"))
	})
	t.Run("prunes a .injected- session marker older than 1 day", func(t *testing.T) {
		t.Parallel()
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, ".injected-old"), 2*day)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "pruned 1 session marker(s) older than 1d")
		gone(t, filepath.Join(scratch, ".injected-old"))
	})
	t.Run("keeps a .injected- session marker younger than 1 day", func(t *testing.T) {
		t.Parallel()
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, ".injected-fresh"), 3600)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "pruned 0 session marker(s) older than 1d")
		kept(t, filepath.Join(scratch, ".injected-fresh"))
	})
	t.Run("marker retention is fixed at 1 day, independent of the .md days argument", func(t *testing.T) {
		// A 2-day-old marker is pruned even when the .md retention window passed
		// as an argument is generous (100 days) - the two sweeps use unrelated
		// cutoffs.
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, ".injected-old"), 2*day)
		k.Run("", "scratch-rotate", "100").Want(t, 0)
		gone(t, filepath.Join(scratch, ".injected-old"))
	})
	t.Run("marker sweep does not descend into subdirectories (maxdepth 1)", func(t *testing.T) {
		t.Parallel()
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, "sub/.injected-nested"), 3*day)
		k.Run("", "scratch-rotate").Want(t, 0)
		kept(t, filepath.Join(scratch, "sub/.injected-nested"))
	})
	t.Run("both sweeps run together and report independent counts", func(t *testing.T) {
		t.Parallel()
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, "old.md"), 40*day)
		touch(t, filepath.Join(scratch, "fresh.md"), 3600)
		touch(t, filepath.Join(scratch, ".injected-old"), 2*day)
		touch(t, filepath.Join(scratch, ".injected-fresh"), 3600)
		r := k.Run("", "scratch-rotate", "30")
		r.Want(t, 0)
		r.Has(t, "pruned 1 artifact(s) older than 30d", "pruned 1 session marker(s) older than 1d")
		gone(t, filepath.Join(scratch, "old.md"))
		kept(t, filepath.Join(scratch, "fresh.md"))
		gone(t, filepath.Join(scratch, ".injected-old"))
		kept(t, filepath.Join(scratch, ".injected-fresh"))
	})
	t.Run("no scratch dir: exits 0 without error", func(t *testing.T) {
		t.Parallel()
		k, scratch, _ := setup(t)
		if err := os.RemoveAll(scratch); err != nil {
			t.Fatal(err)
		}
		k.Run("", "scratch-rotate").Want(t, 0)
	})
	t.Run("prunes an old file in a registered project scratch dir", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		proj := filepath.Join(k.Home, "proj/scratch")
		touch(t, filepath.Join(proj, "poc.py"), 40*day)
		Write(t, registry, proj+"\n")
		r := k.Run("", "scratch-rotate", "30")
		r.Want(t, 0)
		r.Has(t, "pruned 1 artifact(s) older than 30d from "+proj)
		gone(t, filepath.Join(proj, "poc.py"))
		registryIs(t, registry, proj)
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
	t.Run("removes a review worktree older than 1 day through git", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		repo, wt := worktree(t, k, registry, "review-pr-7", 3*day)
		r := k.Run("", "scratch-rotate", "30")
		r.Want(t, 0)
		r.Has(t, "deleted review worktree older than 1d "+wt)
		gone(t, wt)
		if list := k.Git(repo, "worktree", "list"); strings.Contains(list, wt) {
			t.Errorf("git still lists %s:\n%s", wt, list)
		}
	})
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
	t.Run("removes a review worktree 30 hours old", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		_, wt := worktree(t, k, registry, "review-pr-7", 30*3600)
		k.Run("", "scratch-rotate", "30").Has(t, "deleted review worktree older than 1d "+wt)
		gone(t, wt)
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
	t.Run("keeps a fresh file in a registered project scratch dir", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		proj := filepath.Join(k.Home, "proj/scratch")
		touch(t, filepath.Join(proj, "poc.py"), 3600)
		Write(t, registry, proj+"\n")
		r := k.Run("", "scratch-rotate", "30")
		r.Want(t, 0)
		r.Has(t, "pruned 0 artifact(s) older than 30d from "+proj)
		kept(t, filepath.Join(proj, "poc.py"))
	})
	t.Run("drops a registry entry whose project scratch dir no longer exists", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		proj := filepath.Join(Physical(t, t.TempDir()), "proj/scratch")
		Write(t, registry, proj+"\n")
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "dropping stale registry entry "+proj)
		if Read(t, registry) != "" {
			t.Errorf("registry not emptied: %q", Read(t, registry))
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
	t.Run("prunes multiple registered project dirs and keeps existing entries", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		projA := filepath.Join(k.Home, "proj-a/scratch")
		projB := filepath.Join(k.Home, "proj-b/scratch")
		touch(t, filepath.Join(projA, "old.py"), 40*day)
		touch(t, filepath.Join(projB, "fresh.py"), 3600)
		Write(t, registry, projA+"\n"+projB+"\n")
		k.Run("", "scratch-rotate", "30").Want(t, 0)
		gone(t, filepath.Join(projA, "old.py"))
		kept(t, filepath.Join(projB, "fresh.py"))
		registryIs(t, registry, projA+"\n"+projB)
	})
	// The guard that makes every project dir above live under $HOME: a
	// registry line is untrusted input, so anything outside $HOME is refused
	// rather than pruned.
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
	t.Run("no registry file: exits 0 without error", func(t *testing.T) {
		t.Parallel()
		k, _, registry := setup(t)
		if err := os.Remove(registry); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		k.Run("", "scratch-rotate").Want(t, 0)
	})

	// guard-skills' per-skill marker cache
	// ($HOME/.claude/cache/skills-loaded/<session>-<skill>)

	t.Run("prunes a skills-loaded marker older than 1 day", func(t *testing.T) {
		t.Parallel()
		k, _, _ := setup(t)
		cache := filepath.Join(k.Claude, "cache/skills-loaded")
		touch(t, filepath.Join(cache, "s1-bash-patterns"), 2*day)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "pruned 1 skill-loaded marker(s) older than 1d")
		gone(t, filepath.Join(cache, "s1-bash-patterns"))
	})
	t.Run("prunes a file-skills cache older than 1 day and keeps a fresh one", func(t *testing.T) {
		t.Parallel()
		k, _, _ := setup(t)
		cache := filepath.Join(k.Claude, "cache/file-skills")
		touch(t, filepath.Join(cache, "s1-0123abcd"), 2*day)
		touch(t, filepath.Join(cache, "s2-0123abcd"), 3600)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "pruned 1 file-skills cache(s) older than 1d")
		gone(t, filepath.Join(cache, "s1-0123abcd"))
		kept(t, filepath.Join(cache, "s2-0123abcd"))
	})
	t.Run("prunes a plan-active marker older than 1 day", func(t *testing.T) {
		t.Parallel()
		k, _, _ := setup(t)
		cache := filepath.Join(k.Claude, "cache/plan-active")
		touch(t, filepath.Join(cache, "s1"), 2*day)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "pruned 1 plan-active marker(s) older than 1d")
		gone(t, filepath.Join(cache, "s1"))
	})
	t.Run("prunes a statusline git cache older than 1 day", func(t *testing.T) {
		t.Parallel()
		k, _, _ := setup(t)
		cache := filepath.Join(k.Claude, "cache/statusline")
		touch(t, filepath.Join(cache, "git-s1"), 2*day)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "pruned 1 statusline cache(s) older than 1d")
		gone(t, filepath.Join(cache, "git-s1"))
	})
	t.Run("keeps a skills-loaded marker younger than 1 day", func(t *testing.T) {
		t.Parallel()
		k, _, _ := setup(t)
		cache := filepath.Join(k.Claude, "cache/skills-loaded")
		touch(t, filepath.Join(cache, "s1-bash-patterns"), 3600)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "pruned 0 skill-loaded marker(s) older than 1d")
		kept(t, filepath.Join(cache, "s1-bash-patterns"))
	})
	t.Run("prunes only the stale markers, keeping fresh ones in the same cache dir", func(t *testing.T) {
		t.Parallel()
		k, _, _ := setup(t)
		cache := filepath.Join(k.Claude, "cache/skills-loaded")
		touch(t, filepath.Join(cache, "s1-bash-patterns"), 2*day)
		touch(t, filepath.Join(cache, "s1-typescript-patterns"), 3600)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "pruned 1 skill-loaded marker(s) older than 1d")
		gone(t, filepath.Join(cache, "s1-bash-patterns"))
		kept(t, filepath.Join(cache, "s1-typescript-patterns"))
	})
	t.Run("no skills-loaded cache dir: exits 0 without error and skips that line", func(t *testing.T) {
		t.Parallel()
		k, _, _ := setup(t)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Lacks(t, "skill-loaded marker")
	})
	t.Run("trims every JSONL log to log_max_lines from the overlay", func(t *testing.T) {
		t.Parallel()
		k, _, _ := setup(t)
		logs := filepath.Join(k.Claude, "logs")
		k.Overlay("log_max_lines: 2\n")
		Write(t, filepath.Join(logs, "skills.jsonl"), "{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n{\"n\":4}\n")
		Write(t, filepath.Join(logs, "guards.jsonl"), "{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n")
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "trimmed skills.jsonl from 4 to 2 lines", "trimmed guards.jsonl from 3 to 2 lines")
		if got := strings.TrimRight(Read(t, filepath.Join(logs, "guards.jsonl")), "\n"); got != "{\"n\":2}\n{\"n\":3}" {
			t.Errorf("guards.jsonl %q", got)
		}
	})
}
