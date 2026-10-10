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
