package stackctx

import (
	"slices"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestSuggestedSkipsGlobalAndDedupes(t *testing.T) {
	cfg := &config.Config{GlobalSkills: []string{"fix-sizing"}}
	got := suggested(required(cfg), []string{"javascript-patterns", "fix-sizing"}, []string{"react-patterns", "javascript-patterns"})
	if want := []string{"javascript-patterns", "react-patterns"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if (Context{Required: required(cfg)}).RequiredBlock() != "\n<required-skills>\nBLOCKING REQUIREMENT: invoke the Skill tool for each of these skills NOW, before any other action: fix-sizing\n</required-skills>\n" {
		t.Error("required block shape changed")
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
