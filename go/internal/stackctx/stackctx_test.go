package stackctx

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestSuggestedSkipsGlobalAndDedupes(t *testing.T) {
	cfg := &config.Config{GlobalSkills: []string{"fix-sizing"}}
	got := suggested(suggested(nil, cfg.GlobalSkills), []string{"javascript-patterns", "fix-sizing"}, []string{"react-patterns", "javascript-patterns"})
	if want := []string{"javascript-patterns", "react-patterns"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The required block states where the rules are as a fact, and names the
// gate's denial only while the gate is on to deny.
func TestRequiredBlockStatesTheRulesAndTheGate(t *testing.T) {
	ctx := Context{Required: []string{"kit-rules", "slop-patterns"}}
	gated := "\n<required-skills>\nThis session's rules are in these skills: kit-rules, slop-patterns. The rules gate denies every other tool call until each loads through the Skill tool, or without it a Read of its SKILL.md.\n" + requiredReferences + "\n</required-skills>\n"
	if got := ctx.RequiredBlock(true); got != gated {
		t.Errorf("gated:\n%s\nwant\n%s", got, gated)
	}
	ungated := "\n<required-skills>\nThis session's rules are in these skills: kit-rules, slop-patterns. Each loads through the Skill tool.\n" + requiredReferences + "\n</required-skills>\n"
	if got := ctx.RequiredBlock(false); got != ungated {
		t.Errorf("ungated:\n%s\nwant\n%s", got, ungated)
	}
	if got := (Context{}).RequiredBlock(true); got != "" {
		t.Errorf("no required skills: %q", got)
	}
}

// Every subproject's declared dependencies count, Python names normalized,
// and the skills follow dependency_skills' order.
func TestDependencySkills(t *testing.T) {
	t.Parallel()
	root := fsx.PhysicalPath(t.TempDir())
	testutil.Git(t, root, "init", "-q")
	testutil.Put(t, root, "package.json", `{"name":"root","private":true,"devDependencies":{"vitest":"2.0.0"}}`)
	testutil.Put(t, root, "pnpm-workspace.yaml", "packages:\n  - \"packages/*\"\n")
	testutil.Put(t, root, "packages/a/package.json", `{"name":"a","dependencies":{"react":"19.0.0"}}`)
	testutil.Put(t, root, "services/api/pyproject.toml", "[project]\nname = \"api\"\ndependencies = [\"Django>=5\"]\n")
	testutil.Git(t, root, "add", "-A")
	cfg := testutil.KitConfig(t)
	cfg.DependencySkills = []config.DependencyRule{
		{Deps: []string{"django"}, Skills: []string{"django-patterns"}},
		{Deps: []string{"next"}, Skills: []string{"next-app-router-patterns"}},
		{Deps: []string{"react", "preact"}, Skills: []string{"react-patterns"}},
		{Deps: []string{"vitest", "jest"}, Skills: []string{"test-patterns"}},
	}
	if got, want := dependencySkills(cfg, root), []string{"django-patterns", "react-patterns", "test-patterns"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The cached skills follow a subproject's manifest, and a subproject
// added since, though no root manifest changed.
func TestCachedDependencySkillsFollowSubprojects(t *testing.T) {
	t.Parallel()
	root := filepath.Join(fsx.PhysicalPath(t.TempDir()), "My Projects")
	testutil.Put(t, root, "packages/a/package.json", `{"name":"a"}`)
	testutil.Put(t, root, "gems/b/Gemfile", "source 'https://rubygems.org'\n")
	testutil.Git(t, root, "init", "-q")
	testutil.Git(t, root, "add", "-A")
	cfg := testutil.KitConfig(t)
	cfg.DependencySkills = []config.DependencyRule{
		{Deps: []string{"react"}, Skills: []string{"react-patterns"}},
		{Deps: []string{"django"}, Skills: []string{"django-patterns"}},
		{Deps: []string{"rails"}, Skills: []string{"rails-patterns"}},
	}
	file := filepath.Join(t.TempDir(), "stack.txt")
	if got := cachedDependencySkills(cfg, root, file, true); len(got) != 0 {
		t.Fatalf("got %q, want none yet", got)
	}
	later := time.Now().Add(2 * time.Second)
	testutil.Put(t, root, "packages/a/package.json", `{"name":"a","dependencies":{"react":"19.0.0"}}`)
	if err := os.Chtimes(filepath.Join(root, "packages/a/package.json"), later, later); err != nil {
		t.Fatal(err)
	}
	if got := cachedDependencySkills(cfg, root, file, false); !slices.Equal(got, []string{"react-patterns"}) {
		t.Errorf("after a subproject's manifest changed: got %q", got)
	}
	// A lockfile the reader opens counts, though it's no manifest.
	testutil.Put(t, root, "gems/b/Gemfile.lock", "GEM\n  specs:\n    rails (7.1.0)\n")
	if err := os.Chtimes(filepath.Join(root, "gems/b/Gemfile.lock"), later.Add(time.Second), later.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := cachedDependencySkills(cfg, root, file, false); !slices.Equal(got, []string{"react-patterns", "rails-patterns"}) {
		t.Errorf("after a subproject's lockfile changed: got %q", got)
	}

	// A fresh repo, its cache left by an earlier hook process: Subprojects
	// is memoized per process, as each hook run is one.
	added := fsx.PhysicalPath(t.TempDir())
	testutil.Git(t, added, "init", "-q")
	testutil.Put(t, added, "services/api/pyproject.toml", "[project]\nname = \"api\"\ndependencies = [\"Django>=5\"]\n")
	testutil.Git(t, added, "add", "-A")
	file = filepath.Join(t.TempDir(), "stack.txt")
	testutil.Put(t, filepath.Dir(file), "stack.deps.txt", "\n\n")
	if err := os.Chtimes(filepath.Join(added, ".git/index"), later, later); err != nil {
		t.Fatal(err)
	}
	if got := cachedDependencySkills(cfg, added, file, false); !slices.Equal(got, []string{"django-patterns"}) {
		t.Errorf("after a subproject was added: got %q", got)
	}
}

// A gap-fill read touches its answers at most once a day, so it leaves the
// stack report fresh until then; the answers themselves are unchanged.
func TestAGapFillReadRebuildsTheStackReportAtMostDaily(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		age     time.Duration
		rebuilt bool
	}{
		{"read an hour after the last touch", time.Hour, false},
		{"read two days after the last touch", 48 * time.Hour, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := fsx.PhysicalPath(t.TempDir())
			testutil.Put(t, root, "go.mod", "module x\n")
			paths := config.Paths{ClaudeDir: t.TempDir()}
			cfg := testutil.KitConfig(t)
			answered := gapfill.CacheFile(paths.CacheDir(), root)
			testutil.Put(t, filepath.Dir(answered), filepath.Base(answered), "{}")
			touched := time.Now().Add(-tc.age)
			if err := os.Chtimes(answered, touched, touched); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), "stack.txt")
			refresh(paths, cfg, root, file)
			gapfill.Run(cfg, gapfill.Options{Root: root, CacheDir: paths.CacheDir()})
			if got := refresh(paths, cfg, root, file); got != tc.rebuilt {
				t.Errorf("rebuilt = %v, want %v", got, tc.rebuilt)
			}
		})
	}
}
