package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Characterization tests for `kit scratch-dir` and `kit plans-dir`: the
// project tier inside a git repo, the home fallback outside one.
func TestScratchDir(t *testing.T) {
	t.Parallel()
	output := func(r Result) string { return strings.TrimRight(r.Output, "\n") }
	isDir := func(path string) bool {
		fi, err := os.Stat(path)
		return err == nil && fi.IsDir()
	}
	matches := func(t *testing.T, r Result, pattern string) {
		t.Helper()
		if !regexp.MustCompile(pattern).MatchString(output(r)) {
			t.Errorf("output %q does not match %s", output(r), pattern)
		}
	}
	// A fresh sandbox, a repo with src/, and a dir outside any repo with no
	// sentinel within three levels.
	setup := func(t *testing.T) (k *Kit, repo, outside string) {
		k = New(t)
		tmp := Physical(t, t.TempDir())
		k.Git(tmp, "init", "-q", "-b", "main", filepath.Join(tmp, "repo"))
		repo = filepath.Join(tmp, "repo")
		Mkdir(t, filepath.Join(repo, "src"))
		outside = filepath.Join(tmp, "plain/a/b")
		Mkdir(t, outside)
		return k, repo, outside
	}
	cd := func(k *Kit, dir string) {
		k.Dir = dir
		k.Setenv("PWD", dir)
	}

	t.Run("kit scratch-dir inside a repo prints and creates the project tier", func(t *testing.T) {
		t.Parallel()
		k, repo, _ := setup(t)
		cd(k, filepath.Join(repo, "src"))
		r := k.Run("", "scratch-dir")
		r.Want(t, 0)
		if got, want := output(r), filepath.Join(repo, ".claude/scratch"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if !isDir(filepath.Join(repo, ".claude/scratch")) {
			t.Error("project scratch not created")
		}
	})
	t.Run("kit scratch-dir <kind> <slug> prints a timestamped report path", func(t *testing.T) {
		t.Parallel()
		k, repo, _ := setup(t)
		cd(k, repo)
		r := k.Run("", "scratch-dir", "perf", "checkout-page")
		r.Want(t, 0)
		matches(t, r, `^`+regexp.QuoteMeta(repo)+`/\.claude/scratch/perf-checkout-page-[0-9]{8}-[0-9]{4}\.md$`)
	})
	t.Run("kit scratch-dir <kind> alone leaves the slug out", func(t *testing.T) {
		t.Parallel()
		k, repo, _ := setup(t)
		cd(k, repo)
		matches(t, k.Run("", "scratch-dir", "deps"), `/deps-[0-9]{8}-[0-9]{4}\.md$`)
	})
	t.Run("kit scratch-dir makes the slug filename-safe", func(t *testing.T) {
		t.Parallel()
		k, repo, _ := setup(t)
		cd(k, repo)
		matches(t, k.Run("", "scratch-dir", "review", "feat/login page"), `/review-feat-login-page-[0-9]{8}-[0-9]{4}\.md$`)
	})
	t.Run("kit scratch-dir registers the project tier once", func(t *testing.T) {
		t.Parallel()
		k, repo, _ := setup(t)
		cd(k, repo)
		k.Run("", "scratch-dir")
		k.Run("", "scratch-dir")
		got := strings.TrimRight(Read(t, filepath.Join(k.Claude, "logs/scratch-registry.txt")), "\n")
		if want := filepath.Join(repo, ".claude/scratch"); got != want {
			t.Errorf("registry %q, want %q", got, want)
		}
	})
	t.Run("kit scratch-dir outside a project falls back to home", func(t *testing.T) {
		t.Parallel()
		k, _, outside := setup(t)
		cd(k, outside)
		r := k.Run("", "scratch-dir")
		r.Want(t, 0)
		if got, want := output(r), filepath.Join(k.Home, ".claude/scratch"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if !isDir(filepath.Join(k.Home, ".claude/scratch")) {
			t.Error("home scratch not created")
		}
		if Exists(filepath.Join(k.Claude, "logs/scratch-registry.txt")) {
			t.Error("home tier was registered")
		}
	})
	t.Run("kit plans-dir inside a repo prints and creates the project tier", func(t *testing.T) {
		t.Parallel()
		k, repo, _ := setup(t)
		cd(k, filepath.Join(repo, "src"))
		r := k.Run("", "plans-dir")
		r.Want(t, 0)
		if got, want := output(r), filepath.Join(repo, ".claude/plans"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if !isDir(filepath.Join(repo, ".claude/plans")) {
			t.Error("project plans not created")
		}
	})
	t.Run("kit plans-dir registers nothing", func(t *testing.T) {
		t.Parallel()
		k, repo, _ := setup(t)
		cd(k, repo)
		k.Run("", "plans-dir")
		if Exists(filepath.Join(k.Claude, "logs/scratch-registry.txt")) {
			t.Error("plans-dir registered something")
		}
	})
	t.Run("CLAUDE_CONFIG_DIR relocates the fallback and the registry", func(t *testing.T) {
		t.Parallel()
		k, repo, outside := setup(t)
		config := filepath.Join(Physical(t, t.TempDir()), "config")
		k.Setenv("CLAUDE_CONFIG_DIR", config)
		cd(k, outside)
		if got, want := output(k.Run("", "scratch-dir")), filepath.Join(config, "scratch"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}

		cd(k, repo)
		k.Run("", "scratch-dir")
		got := strings.TrimRight(Read(t, filepath.Join(config, "logs/scratch-registry.txt")), "\n")
		if want := filepath.Join(repo, ".claude/scratch"); got != want {
			t.Errorf("registry %q, want %q", got, want)
		}
		if Exists(filepath.Join(k.Claude, "logs/scratch-registry.txt")) {
			t.Error("registered under HOME despite CLAUDE_CONFIG_DIR")
		}
	})
	t.Run("kit plans-dir outside a project falls back to home", func(t *testing.T) {
		t.Parallel()
		k, _, outside := setup(t)
		cd(k, outside)
		r := k.Run("", "plans-dir")
		r.Want(t, 0)
		if got, want := output(r), filepath.Join(k.Home, ".claude/plans"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
