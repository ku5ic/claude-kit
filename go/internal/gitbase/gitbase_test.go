package gitbase

import (
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// repo makes a git repo with main and a feature branch, and chdirs into it.
func repo(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "init"},
		{"checkout", "-q", "-b", "feature"},
	} {
		testutil.Git(t, dir, args...)
	}
	t.Chdir(dir)
}

func TestParseModesFlagsAndPaths(t *testing.T) {
	repo(t)
	a, err := parse([]string{"--diff", "main", "--stat", "-n", "5", "--", "a.go", "-weird"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Mode != Diff || a.Explicit != "main" {
		t.Errorf("mode=%v explicit=%q", a.Mode, a.Explicit)
	}
	if !slices.Equal(a.Extra, []string{"--stat", "-n", "5"}) || !slices.Equal(a.Paths, []string{"a.go", "-weird"}) {
		t.Errorf("extra=%q paths=%q", a.Extra, a.Paths)
	}
}

func TestParseRejectsAWordThatIsNotARef(t *testing.T) {
	repo(t)
	_, err := parse([]string{"no-such-branch"})
	if err == nil || !strings.Contains(err.Error(), "'no-such-branch' is not a ref") {
		t.Errorf("err = %v", err)
	}
}

func TestParseBranchNamedDiffIsABase(t *testing.T) {
	repo(t)
	testutil.Git(t, ".", "branch", "diff")
	a, err := parse([]string{"diff"})
	if err != nil || a.Mode != Base || a.Explicit != "diff" {
		t.Errorf("a=%+v err=%v", a, err)
	}
}

func TestResolveFallsBackToMain(t *testing.T) {
	repo(t)
	if base, ok := Resolve("", ""); !ok || base != "main" {
		t.Errorf("base=%q ok=%v", base, ok)
	}
}

func TestResolveFailsWithNoCandidate(t *testing.T) {
	dir := t.TempDir()
	testutil.Git(t, dir, "init", "-q", "-b", "solo")
	testutil.Put(t, dir, "x", "")
	t.Chdir(dir)
	if base, ok := Resolve("", ""); ok {
		t.Errorf("resolved %q in a repo with no commits", base)
	}
}
