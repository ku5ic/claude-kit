package git_test

import (
	"slices"
	"testing"

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

func TestBaseFallsBackToMain(t *testing.T) {
	t.Parallel()
	if base, ok := git.Base(repo(t), ""); !ok || base != "main" {
		t.Errorf("base=%q ok=%v", base, ok)
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
