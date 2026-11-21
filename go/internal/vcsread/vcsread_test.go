package vcsread

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// setup makes a repo with one committed file and a tag, chdirs into it, and
// puts fake gh and glab first on PATH, recording their runs in the returned
// file.
func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testutil.GitID(t, dir, "init", "-q", "-b", "main")
	testutil.Put(t, dir, "a.txt", "one\n")
	testutil.GitID(t, dir, "add", "a.txt")
	testutil.GitID(t, dir, "commit", "-q", "-m", "init")
	testutil.GitID(t, dir, "tag", "v1")
	t.Chdir(dir)
	bin, calls := t.TempDir(), filepath.Join(t.TempDir(), "calls")
	for _, tool := range []string{"gh", "glab"} {
		testutil.FakeTool(t, filepath.Join(bin, tool), calls, "")
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return calls
}

func TestRunRunsEachAllowedVerb(t *testing.T) {
	calls := setup(t)
	for _, argv := range [][]string{
		{"git", "log", "-1", "--oneline"},
		{"git", "show", "HEAD:a.txt"},
		{"git", "blame", "a.txt"},
		{"git", "tag", "-l"},
		{"git", "tag", "--list", "v*"},
		{"git", "rev-parse", "HEAD"},
		{"git", "ls-files"},
		{"gh", "pr", "view", "12"},
		{"gh", "pr", "list", "--state", "merged"},
		{"gh", "issue", "view", "3", "--comments"},
		{"gh", "release", "list"},
		{"gh", "repo", "view"},
		{"gh", "run", "view", "9", "--log"},
		{"gh", "api", "repos/o/r/pulls/12/comments", "--paginate"},
		{"glab", "mr", "view", "4"},
		{"glab", "issue", "list"},
		{"glab", "api", "projects/1/merge_requests"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Errorf("%q exited %d: %s", argv, code, stderr.String())
		}
		if argv[0] == "git" && stdout.Len() == 0 {
			t.Errorf("%q printed nothing", argv)
		}
	}
	got := testutil.Calls(t, calls)
	if len(got) != 10 {
		t.Fatalf("gh and glab ran %d times, want 10: %q", len(got), got)
	}
	if _, args, _ := strings.Cut(got[0], "|"); args != "pr view 12" {
		t.Errorf("gh ran with %q, want the args as given", args)
	}
}

func TestRunRefusesAnyCallThatCouldWrite(t *testing.T) {
	calls := setup(t)
	for _, argv := range [][]string{
		nil,
		{"rm", "-rf", "x"},
		{"git"},
		{"git", "grep", "-O", "x"},
		{"git", "diff", "--output=out"},
		{"git", "log", "--output=out"},
		{"git", "show", "-Ovim", "HEAD"},
		{"git", "commit", "-m", "x"},
		{"git", "-c", "core.pager=sh", "log"},
		{"git", "tag", "v2"},
		{"git", "tag", "-l", "--no-list", "-a", "v2", "-m", "x"},
		{"gh"},
		{"gh", "api", "-X", "POST", "repos/o/r/issues"},
		{"gh", "api", "-XPOST", "repos/o/r/issues"},
		{"gh", "api", "--method=DELETE", "repos/o/r"},
		{"gh", "api", "-f", "title=x", "repos/o/r/issues"},
		{"gh", "api", "-F", "n=1", "repos/o/r/issues"},
		{"gh", "api", "--raw-field", "a=b", "repos/o/r/issues"},
		{"gh", "api", "--input", "body.json", "repos/o/r/issues"},
		{"gh", "api", "-iX", "POST", "repos/o/r/issues"},
		{"gh", "pr", "merge", "12"},
		{"gh", "pr", "view", "12", "--web"},
		{"gh", "pr", "view", "12", "-w"},
		{"gh", "myext", "view"},
		{"glab", "api", "--method", "POST", "projects/1/issues"},
		{"glab", "mr", "approve", "4"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(argv, &stdout, &stderr); code != 2 {
			t.Errorf("%q exited %d, want 2", argv, code)
		}
		if !strings.HasPrefix(stderr.String(), "kit read: ") {
			t.Errorf("%q gave no reason: %q", argv, stderr.String())
		}
	}
	if got := testutil.Calls(t, calls); got != nil {
		t.Errorf("a refused call ran gh or glab: %q", got)
	}
	if _, err := os.Stat("out"); err == nil {
		t.Error("a refused git call wrote its --output file")
	}
	if out, err := git.Output("", "tag", "-l", "v2"); err != nil || out != "" {
		t.Errorf("a refused git tag created v2: %q, %v", out, err)
	}
}
