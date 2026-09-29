package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// Characterization tests for `kit git-base`: pins which base it picks at
// each step of its detection order.
func TestGitBase(t *testing.T) {
	// output is bats' $output: trailing newlines dropped.
	output := func(r Result) string { return strings.TrimRight(r.Output, "\n") }
	// subjects is `cut -d' ' -f2-` over --log output: the hashes dropped.
	subjects := func(r Result) string {
		var out []string
		for _, l := range Lines(r.Output) {
			_, rest, _ := strings.Cut(l, " ")
			out = append(out, rest)
		}
		return strings.Join(out, "\n")
	}
	// Bare origin with one commit on main, cloned to work, which becomes the
	// cwd. The clone sets origin/HEAD.
	clone := func(t *testing.T) (*Kit, string) {
		k := New(t)
		tmp := Physical(t, t.TempDir())
		seed, origin, work := filepath.Join(tmp, "seed"), filepath.Join(tmp, "origin.git"), filepath.Join(tmp, "work")
		k.Git(tmp, "init", "-q", "-b", "main", seed)
		k.Git(seed, "commit", "-q", "--allow-empty", "-m", "init")
		k.Git(tmp, "clone", "-q", "--bare", seed, origin)
		k.Git(tmp, "clone", "-q", origin, work)
		k.Dir = work
		return k, work
	}
	// A local repo on branch, with one empty commit, as the cwd.
	local := func(t *testing.T, branch string) (*Kit, string) {
		k := New(t)
		dir := filepath.Join(Physical(t, t.TempDir()), "local")
		Mkdir(t, dir)
		k.Git(dir, "init", "-q", "-b", branch)
		k.Git(dir, "commit", "-q", "--allow-empty", "-m", "init")
		k.Dir = dir
		return k, dir
	}

	t.Run("an upstream other than this branch's own name wins", func(t *testing.T) {
		k, work := clone(t)
		k.Git(work, "switch", "-q", "-c", "feat", "--track", "origin/main")
		r := k.Run("", "git-base")
		r.Want(t, 0)
		if got := output(r); got != "origin/main" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("with no upstream, origin/HEAD is used", func(t *testing.T) {
		k, work := clone(t)
		k.Git(work, "switch", "-q", "-c", "feat", "--no-track")
		r := k.Run("", "git-base")
		r.Want(t, 0)
		if got := output(r); got != "origin/main" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("with no remote, a local main is used", func(t *testing.T) {
		k, dir := local(t, "main")
		k.Git(dir, "switch", "-q", "-c", "feat")
		r := k.Run("", "git-base")
		r.Want(t, 0)
		if got := output(r); got != "main" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("an explicit ref that resolves wins over everything", func(t *testing.T) {
		k, work := clone(t)
		k.Git(work, "switch", "-q", "-c", "feat", "--track", "origin/main")
		k.Git(work, "branch", "-q", "other")
		r := k.Run("", "git-base", "other")
		r.Want(t, 0)
		if got := output(r); got != "other" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("nothing resolvable exits 1", func(t *testing.T) {
		k, _ := local(t, "feat")
		r := k.Run("", "git-base")
		r.Want(t, 1)
		r.Empty(t)
	})

	// --diff and --log: the old git-diff-from-base.sh and git-log-from-base.sh.

	// A local repo: main with one commit, feat with two more, one a
	// merge-free change to a.txt.
	branch := func(t *testing.T) (*Kit, string) {
		k, dir := local(t, "main")
		k.Git(dir, "switch", "-q", "-c", "feat")
		Write(t, filepath.Join(dir, "a.txt"), "one\n")
		k.Git(dir, "add", "a.txt")
		k.Git(dir, "commit", "-q", "-m", "add a")
		k.Git(dir, "commit", "-q", "--allow-empty", "-m", "empty")
		return k, dir
	}

	t.Run("--log lists this branch's commits against the base", func(t *testing.T) {
		k, _ := branch(t)
		r := k.Run("", "git-base", "--log")
		r.Want(t, 0)
		if got := subjects(r); got != "empty\nadd a" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("--log passes flags through to git log", func(t *testing.T) {
		k, _ := branch(t)
		r := k.Run("", "git-base", "--log", "-1")
		if got := subjects(r); got != "empty" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("--diff prints the three-dot diff against the base", func(t *testing.T) {
		k, _ := branch(t)
		r := k.Run("", "git-base", "--diff", "--stat")
		r.Want(t, 0)
		r.Has(t, "a.txt | 1 +")
	})
	t.Run("--log -n 5 takes 5 as the flag's value, not the base", func(t *testing.T) {
		k, _ := branch(t)
		r := k.Run("", "git-base", "--log", "-n", "1")
		r.Want(t, 0)
		if got := subjects(r); got != "empty" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("--diff base -- path limits the diff to the path, after the range", func(t *testing.T) {
		k, dir := branch(t)
		Write(t, filepath.Join(dir, "b.txt"), "two\n")
		k.Git(dir, "add", "b.txt")
		k.Git(dir, "commit", "-q", "-m", "add b")
		r := k.Run("", "git-base", "--diff", "main", "--stat", "--", "a.txt")
		r.Want(t, 0)
		r.Has(t, "a.txt")
		r.Lacks(t, "b.txt")
	})
	t.Run("a base that doesn't resolve exits 1 in every mode", func(t *testing.T) {
		k, _ := branch(t)
		for _, mode := range []string{"", "--diff", "--log"} {
			args := []string{"git-base"}
			if mode != "" {
				args = append(args, mode)
			}
			r := k.Run("", append(args, "notaref")...)
			if r.Status != 1 {
				t.Fatalf("mode '%s' exited %d", mode, r.Status)
			}
			r.Has(t, "'notaref' is not a ref")
		}
	})
	t.Run("a branch named log is still usable as the base", func(t *testing.T) {
		k, dir := branch(t)
		k.Git(dir, "branch", "-q", "log", "main")
		r := k.Run("", "git-base", "--log", "log")
		if n := len(Lines(r.Output)); n != 2 {
			t.Errorf("got %d lines:\n%s", n, r.Output)
		}
		r = k.Run("", "git-base", "log")
		if got := output(r); got != "log" {
			t.Errorf("got %q", got)
		}
	})
}
