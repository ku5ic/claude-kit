package project

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

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
	cfg := &config.Config{}
	home := tmp(t)
	paths := config.Paths{Home: home}
	outside := filepath.Join(tmp(t), "plain")

	dir, err := Dir(cfg, paths, outside, "scratch", false)
	if err != nil || dir != filepath.Join(home, "scratch") {
		t.Errorf("unanchored scratch = %q, %v", dir, err)
	}
	if _, err := Dir(cfg, paths, outside, "bogus", false); err == nil {
		t.Error("unknown kind accepted")
	}

	repo := tmp(t)
	git(t, repo, "init", "-q", "-b", "main")
	for range 2 {
		if _, err := Dir(cfg, paths, repo, "scratch", true); err != nil {
			t.Fatal(err)
		}
	}
	lines := regexp.MustCompile(`\n`).Split(readFile(t, filepath.Join(home, "logs", "scratch-registry.txt")), -1)
	if len(lines) != 2 || lines[0] != filepath.Join(repo, ".claude", "scratch") {
		t.Errorf("registry = %q, want the project tier once", lines)
	}
}
