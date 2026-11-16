package rotate

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

const day = 24 * time.Hour

// physicalTemp is a fresh temp dir with symlinks resolved, as git prints
// paths.
func physicalTemp(t *testing.T) string {
	t.Helper()
	return testutil.Physical(t, t.TempDir())
}

// kitHome is a fake Claude config dir under dir: its scratch/ exists,
// nothing else.
func kitHome(t *testing.T, dir string) config.Paths {
	t.Helper()
	paths := config.Paths{ClaudeDir: filepath.Join(dir, ".claude")}
	if err := os.MkdirAll(paths.ScratchHome(), 0o755); err != nil {
		t.Fatal(err)
	}
	return paths
}

// touch creates an empty file at base/rel, last modified age ago.
func touch(t *testing.T, base, rel string, age time.Duration) {
	t.Helper()
	testutil.Put(t, base, rel, "")
	testutil.Age(t, age, filepath.Join(base, rel))
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// rotate runs a 30-day scratch-rotate and fails the test unless it exits 0
// with nothing on stderr; it returns stdout.
func rotate(t *testing.T, paths config.Paths, home string) string {
	t.Helper()
	var stdout, stderr strings.Builder
	if code := Run(paths, home, 30, false, &stdout, &stderr); code != 0 || stderr.Len() > 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	return stdout.String()
}

// wantFiles checks every path, relative to base, is gone or kept.
func wantFiles(t *testing.T, base string, gone, kept []string) {
	t.Helper()
	for _, p := range gone {
		if exists(filepath.Join(base, p)) {
			t.Errorf("%s survived", p)
		}
	}
	for _, p := range kept {
		if !exists(filepath.Join(base, p)) {
			t.Errorf("%s was pruned", p)
		}
	}
}

func wantOutput(t *testing.T, stdout string, has, lacks []string) {
	t.Helper()
	for _, s := range has {
		if !strings.Contains(stdout, s) {
			t.Errorf("stdout lacks %q:\n%s", s, stdout)
		}
	}
	for _, s := range lacks {
		if strings.Contains(stdout, s) {
			t.Errorf("stdout has %q:\n%s", s, stdout)
		}
	}
}

// The home tier: .md artifacts by the retention window, the state caches
// by their own. Paths are relative to the kit home.
func TestRunPrunesTheHomeTierByAge(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]time.Duration
		noScratch bool
		gone      []string
		kept      []string
		has       []string
		lacks     []string
	}{
		{
			name:  "prunes an .md artifact older than the retention window",
			files: map[string]time.Duration{"scratch/old.md": 40 * day},
			gone:  []string{"scratch/old.md"},
			has:   []string{"pruned 1 artifact(s)"},
		},
		{
			name:  "keeps an .md artifact newer than the retention window",
			files: map[string]time.Duration{"scratch/fresh.md": time.Hour},
			kept:  []string{"scratch/fresh.md"},
			has:   []string{"pruned 0 artifact(s)"},
		},
		{
			name:      "no scratch dir: exits 0 without error",
			noScratch: true,
		},
		{
			name: "no registry file: exits 0 without error",
		},
		{
			name:  "prunes a skills-loaded marker older than 1 day",
			files: map[string]time.Duration{"cache/skills-loaded/s1-bash-patterns": 2 * day},
			gone:  []string{"cache/skills-loaded/s1-bash-patterns"},
			has:   []string{"pruned 1 skill-loaded marker(s) older than 1d"},
		},
		{
			name: "prunes a file-skills cache older than 1 day and keeps a fresh one",
			files: map[string]time.Duration{
				"cache/file-skills/s1-0123abcd": 2 * day,
				"cache/file-skills/s2-0123abcd": time.Hour,
			},
			gone: []string{"cache/file-skills/s1-0123abcd"},
			kept: []string{"cache/file-skills/s2-0123abcd"},
			has:  []string{"pruned 1 file-skills cache(s) older than 1d"},
		},
		{
			name:  "prunes a plan-active marker older than 1 day",
			files: map[string]time.Duration{"cache/plan-active/s1": 2 * day},
			gone:  []string{"cache/plan-active/s1"},
			has:   []string{"pruned 1 plan-active marker(s) older than 1d"},
		},
		{
			name:  "prunes a statusline git cache older than 1 day",
			files: map[string]time.Duration{"cache/statusline/git-s1": 2 * day},
			gone:  []string{"cache/statusline/git-s1"},
			has:   []string{"pruned 1 statusline cache(s) older than 1d"},
		},
		{
			name:  "keeps a skills-loaded marker younger than 1 day",
			files: map[string]time.Duration{"cache/skills-loaded/s1-bash-patterns": time.Hour},
			kept:  []string{"cache/skills-loaded/s1-bash-patterns"},
			has:   []string{"pruned 0 skill-loaded marker(s) older than 1d"},
		},
		{
			name: "prunes only the stale skills-loaded markers, keeping fresh ones in the same cache dir",
			files: map[string]time.Duration{
				"cache/skills-loaded/s1-bash-patterns":       2 * day,
				"cache/skills-loaded/s1-typescript-patterns": time.Hour,
			},
			gone: []string{"cache/skills-loaded/s1-bash-patterns"},
			kept: []string{"cache/skills-loaded/s1-typescript-patterns"},
			has:  []string{"pruned 1 skill-loaded marker(s) older than 1d"},
		},
		{
			name:  "no skills-loaded cache dir: skips that line",
			lacks: []string{"skill-loaded marker"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			paths := kitHome(t, t.TempDir())
			if tt.noScratch {
				if err := os.Remove(paths.ScratchHome()); err != nil {
					t.Fatal(err)
				}
			}
			for rel, age := range tt.files {
				touch(t, paths.ClaudeDir, rel, age)
			}
			wantOutput(t, rotate(t, paths, ""), tt.has, tt.lacks)
			wantFiles(t, paths.ClaudeDir, tt.gone, tt.kept)
		})
	}
}

// The registry guard checks a project dir is under $HOME, so the project
// tier's tests set HOME and can't run in parallel. Their paths are
// relative to that fake HOME.

func TestRunPrunesRegisteredProjectDirs(t *testing.T) {
	tests := []struct {
		name         string
		files        map[string]time.Duration
		registry     []string
		gone         []string
		kept         []string
		has          []string // $HOME stands for the fake home
		wantRegistry []string
	}{
		{
			name:         "prunes an old file in a registered project scratch dir",
			files:        map[string]time.Duration{"proj/scratch/poc.py": 40 * day},
			registry:     []string{"proj/scratch"},
			gone:         []string{"proj/scratch/poc.py"},
			has:          []string{"pruned 1 artifact(s) older than 30d from $HOME/proj/scratch"},
			wantRegistry: []string{"proj/scratch"},
		},
		{
			name:         "keeps a fresh file in a registered project scratch dir",
			files:        map[string]time.Duration{"proj/scratch/poc.py": time.Hour},
			registry:     []string{"proj/scratch"},
			kept:         []string{"proj/scratch/poc.py"},
			has:          []string{"pruned 0 artifact(s) older than 30d from $HOME/proj/scratch"},
			wantRegistry: []string{"proj/scratch"},
		},
		{
			name:     "drops a registry entry whose project scratch dir no longer exists",
			registry: []string{"gone/scratch"},
			has:      []string{"dropping stale registry entry $HOME/gone/scratch"},
		},
		{
			name: "prunes multiple registered project dirs and keeps existing entries",
			files: map[string]time.Duration{
				"proj-a/scratch/old.py":   40 * day,
				"proj-b/scratch/fresh.py": time.Hour,
			},
			registry:     []string{"proj-a/scratch", "proj-b/scratch"},
			gone:         []string{"proj-a/scratch/old.py"},
			kept:         []string{"proj-b/scratch/fresh.py"},
			wantRegistry: []string{"proj-a/scratch", "proj-b/scratch"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := physicalTemp(t)
			t.Setenv("HOME", home)
			paths := kitHome(t, home)
			for rel, age := range tt.files {
				touch(t, home, rel, age)
			}
			abs := func(rels []string) string {
				var lines strings.Builder
				for _, rel := range rels {
					lines.WriteString(filepath.Join(home, rel) + "\n")
				}
				return lines.String()
			}
			testutil.Put(t, paths.LogDir(), "scratch-registry.txt", abs(tt.registry))

			stdout := rotate(t, paths, home)
			wantOutput(t, strings.ReplaceAll(stdout, home, "$HOME"), tt.has, nil)
			wantFiles(t, home, tt.gone, tt.kept)
			if got, want := testutil.Read(t, paths.ScratchRegistry()), abs(tt.wantRegistry); got != want {
				t.Errorf("registry %q, want %q", got, want)
			}
		})
	}
}

// A review worktree past a day goes through git worktree remove; age is
// how long ago its .git file was written.
func TestRunRemovesAReviewWorktreeOlderThanADay(t *testing.T) {
	for name, age := range map[string]time.Duration{"3 days": 3 * day, "30 hours": 30 * time.Hour} {
		t.Run(name, func(t *testing.T) {
			home := physicalTemp(t)
			t.Setenv("HOME", home)
			paths := kitHome(t, home)
			repo := filepath.Join(home, "proj")
			testutil.Git(t, home, "init", "-q", "-b", "main", repo)
			testutil.GitID(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
			scratch := filepath.Join(repo, ".claude/scratch")
			testutil.Put(t, paths.LogDir(), "scratch-registry.txt", scratch+"\n")
			wt := filepath.Join(scratch, "review-pr-7")
			testutil.Git(t, repo, "worktree", "add", "-q", "--detach", wt)
			testutil.Age(t, age, filepath.Join(wt, ".git"))

			wantOutput(t, rotate(t, paths, home), []string{"deleted review worktree older than 1d " + wt}, nil)
			if exists(wt) {
				t.Errorf("%s survived", wt)
			}
			out, err := git.Output(repo, "worktree", "list")
			if err != nil || strings.Contains(out, wt) {
				t.Errorf("git worktree list (%v) still has %s:\n%s", err, wt, out)
			}
		})
	}
}

// A project scratch dir loses the folders the run left empty, deepest
// first, when they're past the window by their mtime before the run: never
// the scratch dir itself, a fresh folder, or a nested checkout.
func TestRunRemovesEmptyProjectFolders(t *testing.T) {
	tests := []struct {
		name   string
		files  map[string]time.Duration // under the scratch dir
		dirs   map[string]time.Duration // aged after the files
		dryRun bool
		gone   []string
		kept   []string
		logged []string // rotate log lines, the scratch dir as $S
	}{
		{name: "nested empty folders go bottom-up, and the scratch dir stays",
			dirs: map[string]time.Duration{"a": 40 * day, "a/b": 40 * day, "a/b/c": 40 * day},
			gone: []string{"a"}, kept: []string{"."},
			logged: []string{"deleted $S/a/b/c\n", "deleted $S/a/b\n", "deleted $S/a\n"}},
		{name: "a folder whose only file this run prunes goes in the same run",
			files: map[string]time.Duration{"d/old.py": 40 * day},
			dirs:  map[string]time.Duration{"d": 40 * day},
			gone:  []string{"d"}},
		{name: "a dry run removes nothing and logs what it would",
			files:  map[string]time.Duration{"d/old.py": 40 * day},
			dirs:   map[string]time.Duration{"d": 40 * day},
			dryRun: true,
			kept:   []string{"d/old.py"},
			logged: []string{"would-delete $S/d/old.py\n", "would-delete $S/d\n"}},
		{name: "a folder holding a fresh file stays",
			files: map[string]time.Duration{"e/new.py": time.Hour},
			dirs:  map[string]time.Duration{"e": 40 * day},
			kept:  []string{"e/new.py"}},
		{name: "a fresh empty folder stays",
			dirs: map[string]time.Duration{"f": time.Hour},
			kept: []string{"f"}},
		{name: "a nested checkout is never entered or removed",
			files: map[string]time.Duration{"g/.git": 40 * day},
			dirs:  map[string]time.Duration{"g": 40 * day},
			kept:  []string{"g/.git"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := physicalTemp(t)
			t.Setenv("HOME", home)
			paths := kitHome(t, home)
			scratch := filepath.Join(home, "proj", "scratch")
			testutil.Mkdir(t, scratch)
			for rel := range tt.dirs {
				testutil.Mkdir(t, filepath.Join(scratch, rel))
			}
			for rel, age := range tt.files {
				touch(t, scratch, rel, age)
			}
			for rel, age := range tt.dirs {
				testutil.Age(t, age, filepath.Join(scratch, rel))
			}
			testutil.Put(t, paths.LogDir(), "scratch-registry.txt", scratch+"\n")

			var stdout, stderr strings.Builder
			if code := Run(paths, home, 30, tt.dryRun, &stdout, &stderr); code != 0 || stderr.Len() > 0 {
				t.Fatalf("exit %d, stderr %q", code, stderr.String())
			}
			wantFiles(t, scratch, tt.gone, tt.kept)
			data, _ := os.ReadFile(filepath.Join(paths.LogDir(), "scratch-rotate.log"))
			testutil.Has(t, strings.ReplaceAll(string(data), scratch, "$S"), tt.logged...)
		})
	}
}

func TestDailyRunsOnceADay(t *testing.T) {
	t.Parallel()
	dir := physicalTemp(t)
	paths := kitHome(t, dir)
	touch(t, paths.ScratchHome(), "old.md", 40*day)
	now := time.Now()
	Daily(paths, dir, now)
	if exists(filepath.Join(paths.ScratchHome(), "old.md")) {
		t.Fatal("the first run of the day didn't prune")
	}
	touch(t, paths.ScratchHome(), "old.md", 40*day)
	Daily(paths, dir, now.Add(time.Hour))
	if !exists(filepath.Join(paths.ScratchHome(), "old.md")) {
		t.Error("a second run within a day pruned")
	}
	Daily(paths, dir, now.Add(25*time.Hour))
	if exists(filepath.Join(paths.ScratchHome(), "old.md")) {
		t.Error("a run a day later didn't prune")
	}
}

func TestCleanRemovesOnlyTheKitsOwnState(t *testing.T) {
	t.Parallel()
	dir := physicalTemp(t)
	paths := kitHome(t, dir)
	kit := []string{
		paths.LogFile("guards"), paths.LogFile("claims"), paths.LogFile("skills"), paths.ScratchRegistry(),
		rotateLog(paths), filepath.Join(paths.CacheDir(), "stack", "x"),
		filepath.Join(paths.CacheDir(), "config.gob"), filepath.Join(paths.ScratchHome(), "report.md"),
	}
	others := []string{filepath.Join(paths.CacheDir(), "changelog.md"), filepath.Join(paths.LogDir(), "other.log"), filepath.Join(paths.PlansHome(), "p.md")}
	for _, p := range append(slices.Clone(kit), others...) {
		testutil.Write(t, p, "x")
	}
	var out strings.Builder
	if code := Clean(paths, false, &out); code != 0 {
		t.Fatalf("dry run exit %d", code)
	}
	for _, p := range kit {
		if !exists(p) {
			t.Errorf("the dry run removed %s", p)
		}
	}
	if !strings.Contains(out.String(), "would remove") || !strings.Contains(out.String(), "--yes") {
		t.Errorf("dry run output:\n%s", out.String())
	}
	if code := Clean(paths, true, &out); code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, p := range kit {
		if exists(p) {
			t.Errorf("%s survived", p)
		}
	}
	for _, p := range others {
		if !exists(p) {
			t.Errorf("%s isn't the kit's, and went", p)
		}
	}
}

func TestCleanSaysWhatItLeaves(t *testing.T) {
	t.Parallel()
	dir := physicalTemp(t)
	paths := kitHome(t, dir)
	var out strings.Builder
	Clean(paths, true, io.Discard)
	Clean(paths, true, &out)
	if !strings.Contains(out.String(), "nothing to remove") {
		t.Errorf("an already-clean home printed:\n%q", out.String())
	}
	project := filepath.Join(dir, "proj", ".claude", "scratch")
	testutil.Write(t, paths.ScratchRegistry(), project+"\n")
	out.Reset()
	Clean(paths, true, &out)
	if !strings.Contains(out.String(), "delete it yourself: "+project) {
		t.Errorf("--yes forgets the project scratch without saying so:\n%s", out.String())
	}
}
