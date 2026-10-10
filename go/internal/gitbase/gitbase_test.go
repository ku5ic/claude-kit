package gitbase

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// gitID is testutil.Git with a fixed identity and no signing, so commits
// work whatever the developer's config.
func gitID(t *testing.T, dir string, args ...string) {
	t.Helper()
	testutil.Git(t, dir, append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
}

// repo makes a git repo with main and a feature branch, and chdirs into it.
func repo(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	gitID(t, dir, "init", "-q", "-b", "main")
	gitID(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	gitID(t, dir, "checkout", "-q", "-b", "feature")
	t.Chdir(dir)
}

// local makes a repo on branch with one empty commit and returns it.
func local(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	gitID(t, dir, "init", "-q", "-b", branch)
	gitID(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

// clone makes a bare origin with one commit on main and returns a clone
// of it, which has origin/HEAD set.
func clone(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	seed, origin, work := local(t, "main"), filepath.Join(tmp, "origin.git"), filepath.Join(tmp, "work")
	gitID(t, tmp, "clone", "-q", "--bare", seed, origin)
	gitID(t, tmp, "clone", "-q", origin, work)
	return work
}

// branched is a local main with one commit and feat, checked out, two
// commits ahead: "add a" (a.txt) then "empty".
func branched(t *testing.T) string {
	t.Helper()
	dir := local(t, "main")
	gitID(t, dir, "switch", "-q", "-c", "feat")
	testutil.Put(t, dir, "a.txt", "one\n")
	gitID(t, dir, "add", "a.txt")
	gitID(t, dir, "commit", "-q", "-m", "add a")
	gitID(t, dir, "commit", "-q", "--allow-empty", "-m", "empty")
	return dir
}

// run is kit git-base with args, from dir.
func run(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	t.Chdir(dir)
	var out, errOut strings.Builder
	code = Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// subjects drops the hashes from --log output.
func subjects(log string) string {
	var out []string
	for line := range strings.Lines(log) {
		_, subject, _ := strings.Cut(strings.TrimSuffix(line, "\n"), " ")
		out = append(out, subject)
	}
	return strings.Join(out, "\n")
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

// Run resolves from the working directory, so none of these is parallel.

func TestRunPrintsTheBase(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		args  []string
		want  string
	}{
		{
			name: "an upstream other than this branch's own name wins",
			setup: func(t *testing.T) string {
				dir := clone(t)
				gitID(t, dir, "switch", "-q", "-c", "feat", "--track", "origin/main")
				return dir
			},
			want: "origin/main\n",
		},
		{
			name: "with no upstream, origin/HEAD is used",
			setup: func(t *testing.T) string {
				dir := clone(t)
				gitID(t, dir, "switch", "-q", "-c", "feat", "--no-track")
				return dir
			},
			want: "origin/main\n",
		},
		{
			name: "with no remote, a local main is used",
			setup: func(t *testing.T) string {
				dir := local(t, "main")
				gitID(t, dir, "switch", "-q", "-c", "feat")
				return dir
			},
			want: "main\n",
		},
		{
			name: "an explicit ref that resolves wins over everything",
			setup: func(t *testing.T) string {
				dir := clone(t)
				gitID(t, dir, "switch", "-q", "-c", "feat", "--track", "origin/main")
				gitID(t, dir, "branch", "-q", "other")
				return dir
			},
			args: []string{"other"},
			want: "other\n",
		},
		{
			name: "a branch named log is still usable as the base",
			setup: func(t *testing.T) string {
				dir := branched(t)
				gitID(t, dir, "branch", "-q", "log", "main")
				return dir
			},
			args: []string{"log"},
			want: "log\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(t, tt.setup(t), tt.args...)
			if code != 0 || stdout != tt.want {
				t.Errorf("exit %d, stdout %q, want %q; stderr %q", code, stdout, tt.want, stderr)
			}
		})
	}
}

func TestRunWithNothingResolvableExits1Silently(t *testing.T) {
	code, stdout, stderr := run(t, local(t, "feat"))
	if code != 1 || stdout != "" || stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestRunLogListsTheBranchCommits(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		args  []string
		want  string
	}{
		{name: "against the base", args: []string{"--log"}, want: "empty\nadd a"},
		{name: "passing flags through to git log", args: []string{"--log", "-1"}, want: "empty"},
		{name: "taking -n 1's 1 as the flag's value, not the base", args: []string{"--log", "-n", "1"}, want: "empty"},
		{
			name:  "against a branch named log",
			setup: func(t *testing.T, dir string) { gitID(t, dir, "branch", "-q", "log", "main") },
			args:  []string{"--log", "log"},
			want:  "empty\nadd a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := branched(t)
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			code, stdout, stderr := run(t, dir, tt.args...)
			if got := subjects(stdout); code != 0 || got != tt.want {
				t.Errorf("exit %d, subjects %q, want %q; stderr %q", code, got, tt.want, stderr)
			}
		})
	}
}

func TestRunDiffIsTheThreeDotDiffAgainstTheBase(t *testing.T) {
	code, stdout, _ := run(t, branched(t), "--diff", "--stat")
	if code != 0 || !strings.Contains(stdout, "a.txt | 1 +") {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
}

func TestRunDiffPathsAfterDashDashLimitTheDiff(t *testing.T) {
	dir := branched(t)
	testutil.Put(t, dir, "b.txt", "two\n")
	gitID(t, dir, "add", "b.txt")
	gitID(t, dir, "commit", "-q", "-m", "add b")
	code, stdout, _ := run(t, dir, "--diff", "main", "--stat", "--", "a.txt")
	if code != 0 || !strings.Contains(stdout, "a.txt") || strings.Contains(stdout, "b.txt") {
		t.Errorf("exit %d, stdout %q, want a.txt only", code, stdout)
	}
}

func TestRunABaseThatDoesNotResolveExits1InEveryMode(t *testing.T) {
	for name, mode := range map[string][]string{"base": nil, "--diff": {"--diff"}, "--log": {"--log"}} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := run(t, branched(t), append(mode, "notaref")...)
			if code != 1 || !strings.Contains(stderr, "'notaref' is not a ref") {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
		})
	}
}
