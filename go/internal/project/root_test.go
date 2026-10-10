package project

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// Callers drop their own symlink resolving on the strength of this; outside
// git, Root keeps the path as given (kit project-root prints $PWD).
func TestToplevelIsPhysicalThroughASymlink(t *testing.T) {
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real, link := filepath.Join(tmp, "real"), filepath.Join(tmp, "link")
	testutil.Put(t, real, "sub/x.go", "package x\n")
	testutil.Git(t, real, "init", "-q")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if top := git.Toplevel(filepath.Join(link, "sub")); top != real {
		t.Errorf("Toplevel = %q, want %q", top, real)
	}
}

func TestName(t *testing.T) {
	t.Setenv("HOME", "/Users/someone")
	cases := map[string]string{
		"/Users/someone":           "home",
		"/":                        "root",
		"/src/My Project":          "my-project",
		"/src/.dotfiles":           "dotfiles",
		"/src/--weird__Name!!":     "weird-name",
		"/src/...":                 "unknown",
		"/src/claude-kit":          "claude-kit",
		"/src/Spacelift.Front_end": "spacelift-front-end",
	}
	for root, want := range cases {
		if got := Name(root); got != want {
			t.Errorf("Name(%q) = %q, want %q", root, got, want)
		}
	}
}

func TestReportPath(t *testing.T) {
	got := ReportPath("/s", "review", "feat/login page", "20260928-1500")
	if got != "/s/review-feat-login-page-20260928-1500.md" {
		t.Errorf("got %q", got)
	}
	if got := ReportPath("/s", "deps", "", "20260928-1500"); got != "/s/deps-20260928-1500.md" {
		t.Errorf("no slug: got %q", got)
	}
}

func TestDirHomeFallbackAndRegistry(t *testing.T) {
	home := tmp(t)
	paths := config.Paths{Home: home}
	outside := filepath.Join(tmp(t), "plain")

	dir, err := Dir(paths, outside, "scratch", false)
	if err != nil || dir != filepath.Join(home, "scratch") {
		t.Errorf("unanchored scratch = %q, %v", dir, err)
	}
	if _, err := Dir(paths, outside, "bogus", false); err == nil {
		t.Error("unknown kind accepted")
	}

	repo := tmp(t)
	testutil.Git(t, repo, "init", "-q", "-b", "main")
	for range 2 {
		if _, err := Dir(paths, repo, "scratch", true); err != nil {
			t.Fatal(err)
		}
	}
	lines := regexp.MustCompile(`\n`).Split(testutil.Read(t, filepath.Join(home, "logs", "scratch-registry.txt")), -1)
	if len(lines) != 2 || lines[0] != filepath.Join(repo, ".claude", "scratch") {
		t.Errorf("registry = %q, want the project tier once", lines)
	}
}

// Only a project scratch dir is registered: the home fallback isn't one for
// scratch-rotate to prune.
func TestDirScratchOutsideAProjectFallsBackToTheKitHomeUnregistered(t *testing.T) {
	t.Parallel()
	home := tmp(t)
	cwd := filepath.Join(tmp(t), "plain", "a", "b")
	testutil.Put(t, cwd, ".keep", "")
	dir, err := Dir(config.Paths{Home: home}, cwd, "scratch", true)
	if want := filepath.Join(home, "scratch"); err != nil || dir != want {
		t.Fatalf("Dir = %q, %v; want %q", dir, err, want)
	}
	if !fsx.IsDir(dir) {
		t.Errorf("%s not created", dir)
	}
	if _, err := os.Stat(filepath.Join(home, "logs", "scratch-registry.txt")); err == nil {
		t.Error("registered the home tier")
	}
}

func TestRoot(t *testing.T) {
	tests := []struct {
		name string
		// setup lays out a temp dir and returns the cwd and the root it wants.
		setup    func(t *testing.T, dir string) (cwd, root string)
		anchored bool
	}{
		{
			name: "a repo's git toplevel, from a nested subdirectory",
			setup: func(t *testing.T, dir string) (string, string) {
				testutil.Git(t, dir, "init", "-q", "-b", "main")
				testutil.Put(t, dir, "src/app/.keep", "")
				return filepath.Join(dir, "src/app"), dir
			},
			anchored: true,
		},
		{
			name: "outside a repo, a language manifest two levels up",
			setup: func(t *testing.T, dir string) (string, string) {
				testutil.Put(t, dir, "proj/package.json", "")
				testutil.Put(t, dir, "proj/a/b/.keep", "")
				return filepath.Join(dir, "proj/a/b"), filepath.Join(dir, "proj")
			},
			anchored: true,
		},
		{
			name: "outside a repo with no manifest, the cwd itself",
			setup: func(t *testing.T, dir string) (string, string) {
				testutil.Put(t, dir, "plain/a/b/.keep", "")
				return filepath.Join(dir, "plain/a/b"), filepath.Join(dir, "plain/a/b")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cwd, want := tt.setup(t, tmp(t))
			if root, anchored := Root(cwd); root != want || anchored != tt.anchored {
				t.Errorf("Root = %q, %v; want %q, %v", root, anchored, want, tt.anchored)
			}
		})
	}
}
