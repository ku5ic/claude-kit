package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Tests for `kit scratch-rotate`.
//
// The command's home-fallback target dir is $HOME/.claude/scratch and its
// registry $HOME/.claude/logs/scratch-registry.txt, so every test fakes
// $HOME to keep it off the real ones. Ages are set with backdate rather
// than sleeping past a retention window.
func TestScratchRotate(t *testing.T) {
	const day = 86400
	setup := func(t *testing.T) (k *Kit, scratch, registry string) {
		k = New(t)
		scratch = filepath.Join(k.Claude, "scratch")
		registry = filepath.Join(k.Claude, "logs/scratch-registry.txt")
		Mkdir(t, scratch)
		return k, scratch, registry
	}
	// touch creates an empty file whose mtime is secs seconds ago.
	touch := func(t *testing.T, path string, secs int) {
		t.Helper()
		Touch(t, path)
		ts := time.Now().Add(-time.Duration(secs) * time.Second)
		if err := os.Chtimes(path, ts, ts); err != nil {
			t.Fatal(err)
		}
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
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, "old.md"), 40*day)
		r := k.Run("", "scratch-rotate", "30")
		r.Want(t, 0)
		r.Has(t, "pruned 1 artifact(s)")
		gone(t, filepath.Join(scratch, "old.md"))
	})
	t.Run("--dry-run reports what would go and deletes nothing", func(t *testing.T) {
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, "old.md"), 40*day)
		r := k.Run("", "scratch-rotate", "--dry-run")
		r.Want(t, 0)
		r.Has(t, "would prune 1 artifact(s)")
		r.Lacks(t, "scratch-rotate: pruned")
		kept(t, filepath.Join(scratch, "old.md"))
	})
	t.Run("keeps an .md artifact newer than the retention window", func(t *testing.T) {
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, "fresh.md"), 3600)
		r := k.Run("", "scratch-rotate", "30")
		r.Want(t, 0)
		r.Has(t, "pruned 0 artifact(s)")
		kept(t, filepath.Join(scratch, "fresh.md"))
	})
	t.Run("prunes a .injected- session marker older than 1 day", func(t *testing.T) {
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, ".injected-old"), 2*day)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "pruned 1 session marker(s) older than 1d")
		gone(t, filepath.Join(scratch, ".injected-old"))
	})
	t.Run("keeps a .injected- session marker younger than 1 day", func(t *testing.T) {
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
		k, scratch, _ := setup(t)
		touch(t, filepath.Join(scratch, "sub/.injected-nested"), 3*day)
		k.Run("", "scratch-rotate").Want(t, 0)
		kept(t, filepath.Join(scratch, "sub/.injected-nested"))
	})
	t.Run("both sweeps run together and report independent counts", func(t *testing.T) {
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
		k, scratch, _ := setup(t)
		if err := os.RemoveAll(scratch); err != nil {
			t.Fatal(err)
		}
		k.Run("", "scratch-rotate").Want(t, 0)
	})
	t.Run("prunes an old file in a registered project scratch dir", func(t *testing.T) {
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
	t.Run("keeps a fresh file in a registered project scratch dir", func(t *testing.T) {
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
		k, _, registry := setup(t)
		proj := filepath.Join(Physical(t, t.TempDir()), "proj/scratch")
		Write(t, registry, proj+"\n")
		r := k.Run("", "scratch-rotate", "--dry-run")
		r.Want(t, 0)
		r.Has(t, "would drop stale registry entry "+proj)
		registryIs(t, registry, proj)
	})
	t.Run("prunes multiple registered project dirs and keeps existing entries", func(t *testing.T) {
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
		k, _, registry := setup(t)
		if err := os.Remove(registry); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		k.Run("", "scratch-rotate").Want(t, 0)
	})

	// guard-skills' per-skill marker cache
	// ($HOME/.claude/cache/skills-loaded/<session>-<skill>)

	t.Run("prunes a skills-loaded marker older than 1 day", func(t *testing.T) {
		k, _, _ := setup(t)
		cache := filepath.Join(k.Claude, "cache/skills-loaded")
		touch(t, filepath.Join(cache, "s1-bash-patterns"), 2*day)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "pruned 1 skill-loaded marker(s) older than 1d")
		gone(t, filepath.Join(cache, "s1-bash-patterns"))
	})
	t.Run("prunes a file-skills cache older than 1 day and keeps a fresh one", func(t *testing.T) {
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
	t.Run("keeps a skills-loaded marker younger than 1 day", func(t *testing.T) {
		k, _, _ := setup(t)
		cache := filepath.Join(k.Claude, "cache/skills-loaded")
		touch(t, filepath.Join(cache, "s1-bash-patterns"), 3600)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Has(t, "pruned 0 skill-loaded marker(s) older than 1d")
		kept(t, filepath.Join(cache, "s1-bash-patterns"))
	})
	t.Run("prunes only the stale markers, keeping fresh ones in the same cache dir", func(t *testing.T) {
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
		k, _, _ := setup(t)
		r := k.Run("", "scratch-rotate")
		r.Want(t, 0)
		r.Lacks(t, "skill-loaded marker")
	})
	t.Run("trims every JSONL log to log_max_lines from the overlay", func(t *testing.T) {
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
