package git_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// repo is a git repo on main with one commit holding a.txt, then on a
// feature branch.
func repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testutil.Put(t, dir, "a.txt", "one\n")
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "a.txt"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init"},
		{"checkout", "-q", "-b", "feature"},
	} {
		testutil.Git(t, dir, args...)
	}
	return dir
}

// A status with stale stat info would rewrite the index under index.lock,
// racing the user's own git add or commit.
func TestStatusLeavesTheIndexAlone(t *testing.T) {
	t.Parallel()
	dir := repo(t)
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "a.txt"), old, old); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(dir, ".git", "index")
	before, err := os.Stat(index)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git.Output(dir, "status", "--porcelain"); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(index)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Error("git status rewrote .git/index")
	}
}

func TestBaseFallsBackToMain(t *testing.T) {
	t.Parallel()
	if base, ok := git.Base(repo(t), ""); !ok || base != "main" {
		t.Errorf("base=%q ok=%v", base, ok)
	}
}

// stacked is a clone whose child branch is one commit past parent, and
// parent one past main, all pushed, with child checked out and tracking its
// own push target.
func stacked(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	origin, work := filepath.Join(tmp, "origin.git"), filepath.Join(tmp, "work")
	testutil.Git(t, repo(t), "clone", "-q", "--bare", ".", origin)
	testutil.Git(t, origin, "symbolic-ref", "HEAD", "refs/heads/main")
	testutil.Git(t, tmp, "clone", "-q", origin, work)
	for _, branch := range []string{"feat/parent", "feat/child"} {
		testutil.Git(t, work, "checkout", "-q", "-b", branch)
		testutil.GitID(t, work, "commit", "-q", "--allow-empty", "-m", branch)
		testutil.Git(t, work, "push", "-q", "-u", "origin", branch)
	}
	return work
}

// A stacked branch's base is the parent gh's own gh-merge-base setting
// names: a pushed child's upstream is only its push target, and origin/HEAD
// would count the parent's commits as its own.
func TestBaseOfAStackedBranch(t *testing.T) {
	t.Parallel()
	key := func(t *testing.T, dir, branch, value string) {
		testutil.Git(t, dir, "config", "branch."+branch+".gh-merge-base", value)
	}
	for _, c := range []struct {
		name, explicit, want string
		arrange              func(t *testing.T, dir string)
	}{
		{"with no key, origin/HEAD", "", "origin/main", nil},
		{"the parent the key names", "", "feat/parent", func(t *testing.T, dir string) { key(t, dir, "feat/child", "feat/parent") }},
		{"the key beats an upstream", "", "feat/parent", func(t *testing.T, dir string) {
			key(t, dir, "feat/child", "feat/parent")
			testutil.Git(t, dir, "branch", "-q", "--set-upstream-to=origin/main")
		}},
		{"a parent only on origin", "", "origin/feat/parent", func(t *testing.T, dir string) {
			key(t, dir, "feat/child", "feat/parent")
			testutil.Git(t, dir, "branch", "-q", "-D", "feat/parent")
		}},
		{"a key naming no branch", "", "origin/main", func(t *testing.T, dir string) { key(t, dir, "feat/child", "gone") }},
		{"a key naming a tag", "", "origin/main", func(t *testing.T, dir string) {
			testutil.Git(t, dir, "tag", "v1", "feat/parent")
			key(t, dir, "feat/child", "v1")
		}},
		{"a key naming the branch itself", "", "origin/main", func(t *testing.T, dir string) { key(t, dir, "feat/child", "feat/child") }},
		{"another branch's key", "", "origin/main", func(t *testing.T, dir string) { key(t, dir, "feat/parent", "feat/child") }},
		{"a detached HEAD", "", "origin/main", func(t *testing.T, dir string) {
			key(t, dir, "feat/child", "feat/parent")
			key(t, dir, "HEAD", "feat/parent")
			testutil.Git(t, dir, "checkout", "-q", "--detach")
		}},
		{"an explicit base wins", "main", "main", func(t *testing.T, dir string) { key(t, dir, "feat/child", "feat/parent") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := stacked(t)
			if c.arrange != nil {
				c.arrange(t, dir)
			}
			if base, ok := git.Base(dir, c.explicit); !ok || base != c.want {
				t.Errorf("base=%q ok=%v, want %q", base, ok, c.want)
			}
		})
	}
	t.Run("ForkPoint is the parent's tip", func(t *testing.T) {
		t.Parallel()
		dir := stacked(t)
		key(t, dir, "feat/child", "feat/parent")
		want, err := git.Line(dir, "rev-parse", "feat/parent")
		if sha, ok := git.ForkPoint(dir); err != nil || !ok || sha != want {
			t.Errorf("sha=%q ok=%v, want %q (%v)", sha, ok, want, err)
		}
	})
}

// ForkPoint is where HEAD left its base, and HEAD itself only on another
// branch: on main with no remote, main is HEAD, and holds every commit
// made on it.
func TestForkPointIsHeadOnlyOffTheBaseBranch(t *testing.T) {
	t.Parallel()
	commit := []string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "next"}
	dir := repo(t)
	mainSHA, err := git.Line(dir, "rev-parse", "main")
	if err != nil {
		t.Fatal(err)
	}
	if sha, ok := git.ForkPoint(dir); !ok || sha != mainSHA {
		t.Errorf("on a branch with no commits: sha=%q ok=%v, want main's %q", sha, ok, mainSHA)
	}
	testutil.Git(t, dir, commit...)
	if sha, ok := git.ForkPoint(dir); !ok || sha != mainSHA {
		t.Errorf("on a branch: sha=%q ok=%v, want main's %q", sha, ok, mainSHA)
	}
	testutil.Git(t, dir, "checkout", "-q", "main")
	testutil.Git(t, dir, commit...)
	if sha, ok := git.ForkPoint(dir); ok {
		t.Errorf("on main with no remote, got %q, want none", sha)
	}
}

func TestBaseFailsWithNoCandidate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.Git(t, dir, "init", "-q", "-b", "solo")
	testutil.Put(t, dir, "x", "")
	if base, ok := git.Base(dir, ""); ok {
		t.Errorf("resolved %q in a repo with no commits", base)
	}
}

func TestStatusParsesRenamesAndOddNames(t *testing.T) {
	t.Parallel()
	dir := repo(t)
	testutil.Git(t, dir, "mv", "a.txt", "b c.txt")
	testutil.Put(t, dir, "new/one.txt", "")
	testutil.Put(t, dir, "new/two.txt", "")
	got, err := git.Status(dir, false)
	want := []git.Change{{"R ", "b c.txt"}, {"??", "new/"}}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("Status = %q, %v; want %q", got, err, want)
	}
	all, _ := git.Status(dir, true)
	if want := []git.Change{{"R ", "b c.txt"}, {"??", "new/one.txt"}, {"??", "new/two.txt"}}; !slices.Equal(all, want) {
		t.Errorf("Status all = %q, want %q", all, want)
	}
}

// A same-size rewrite that keeps the stat data of its index entry is
// rehashed only while the entry is racy: no older than the index file. A
// copy of the index made later must keep the index's mtime, or the entry
// stops being racy and the snapshot keeps the old content.
func TestSnapshotSeesARacyRewrite(t *testing.T) {
	t.Parallel()
	dir := repo(t)
	// The rewrite can't keep its ctime; this makes mtime and size the match.
	testutil.Git(t, dir, "config", "core.trustctime", "false")
	path := filepath.Join(dir, "a.txt")
	past := time.Now().Add(-time.Hour).Truncate(time.Second)
	touch := func(path string) {
		if err := os.Chtimes(path, past, past); err != nil {
			t.Fatal(err)
		}
	}
	touch(path)
	testutil.Git(t, dir, "update-index", "--refresh")
	touch(filepath.Join(dir, ".git", "index"))
	testutil.Put(t, dir, "a.txt", "ONE\n")
	touch(path)
	head, _ := git.Line(dir, "rev-parse", "HEAD^{tree}")
	if s, _ := git.Snapshot(dir); s == head {
		t.Error("the snapshot kept the committed content of a rewritten file")
	}
}

func TestSnapshotSeesContentNotIndex(t *testing.T) {
	t.Parallel()
	dir := repo(t)
	clean, err := git.Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Put(t, dir, "a.txt", "two\n")
	modified, _ := git.Snapshot(dir)
	testutil.Put(t, dir, "a.txt", "three\n")
	rewritten, _ := git.Snapshot(dir)
	testutil.Put(t, dir, "untracked.txt", "x")
	untracked, _ := git.Snapshot(dir)
	if again, _ := git.Snapshot(dir); again != untracked {
		t.Errorf("an unchanged tree snapshots differently: %s, %s", untracked, again)
	}
	for _, s := range []string{modified, rewritten, untracked} {
		if s == "" || s == clean {
			t.Fatalf("snapshots %s %s %s %s: each change must differ", clean, modified, rewritten, untracked)
		}
	}
	if modified == rewritten {
		t.Error("rewriting an already-modified file left the snapshot unchanged")
	}
	if staged, _ := git.Lines(dir, "diff", "--cached", "--name-only"); len(staged) != 0 {
		t.Errorf("Snapshot touched the real index: %q", staged)
	}
}

// A hook's deadline cuts every git call short of it, so git never outlives
// the hook's own timeout. Not parallel: the deadline is the process's.
func TestADeadlineCutsAGitCall(t *testing.T) {
	dir := t.TempDir()
	git.SetDeadline(time.Now().Add(-time.Second))
	_, err := git.Output(dir, "--version")
	git.SetDeadline(time.Time{})
	if err == nil {
		t.Error("a git call past the deadline ran")
	}
	if _, err := git.Output(dir, "--version"); err != nil {
		t.Errorf("with no deadline: %v", err)
	}
}
