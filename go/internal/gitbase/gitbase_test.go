package gitbase

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	t.Chdir(dir)
}

func TestParseModesFlagsAndPaths(t *testing.T) {
	repo(t)
	a, err := Parse([]string{"--diff", "main", "--stat", "-n", "5", "--", "a.go", "-weird"})
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
	_, err := Parse([]string{"no-such-branch"})
	if err == nil || !strings.Contains(err.Error(), "'no-such-branch' is not a ref") {
		t.Errorf("err = %v", err)
	}
}

func TestParseBranchNamedDiffIsABase(t *testing.T) {
	repo(t)
	cmd := exec.Command("git", "branch", "diff")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	a, err := Parse([]string{"diff"})
	if err != nil || a.Mode != Base || a.Explicit != "diff" {
		t.Errorf("a=%+v err=%v", a, err)
	}
}

func TestResolveFallsBackToMain(t *testing.T) {
	repo(t)
	if base, ok := Resolve(""); !ok || base != "main" {
		t.Errorf("base=%q ok=%v", base, ok)
	}
}

func TestResolveFailsWithNoCandidate(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q", "-b", "solo").CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "x"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if base, ok := Resolve(""); ok {
		t.Errorf("resolved %q in a repo with no commits", base)
	}
}
