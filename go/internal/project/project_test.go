package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// kit_tasks and kit_subprojects against the real kit.yml.

func tmp(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func taskLines(tasks []Task) string {
	var lines []string
	for _, task := range tasks {
		stack := task.Stack
		if stack == "" {
			stack = "-"
		}
		lines = append(lines, strings.Join([]string{task.Provider, stack, task.Name, task.Cmd}, "\t"))
	}
	return strings.Join(lines, "\n")
}

func TestTasksResolvePMFromLockfile(t *testing.T) {
	dir := tmp(t)
	testutil.Git(t, dir, "init", "-q", "-b", "main")
	testutil.Put(t, dir, "package.json", `{"scripts":{"test":"vitest"}}`)
	testutil.Put(t, dir, "pnpm-lock.yaml", "")
	if got := taskLines(Tasks(testutil.KitConfig(t), dir)); got != "package-scripts\tjs\ttest\tpnpm run test" {
		t.Errorf("got %q", got)
	}
}

func TestTasksDefaultPMIsNpm(t *testing.T) {
	dir := tmp(t)
	testutil.Put(t, dir, "package.json", `{"scripts":{"lint":"eslint ."}}`)
	if got := taskLines(Tasks(testutil.KitConfig(t), dir)); got != "package-scripts\tjs\tlint\tnpm run lint" {
		t.Errorf("got %q", got)
	}
}

func TestTasksRunByPMForPoeUnderPoetry(t *testing.T) {
	dir := tmp(t)
	testutil.Put(t, dir, "pyproject.toml", "[tool.poe.tasks]\ntest = \"pytest\"\n")
	testutil.Put(t, dir, "poetry.lock", "")
	if got := taskLines(Tasks(testutil.KitConfig(t), dir)); got != "poe\tpython\ttest\tpoetry run poe test" {
		t.Errorf("got %q", got)
	}
}

func TestTasksSeveralProviders(t *testing.T) {
	dir := tmp(t)
	testutil.Put(t, dir, "Makefile", "test:\n\tgo test\n")
	testutil.Put(t, dir, "pyproject.toml", "[tool.pdm.scripts]\nlint = \"ruff\"\n")
	got := taskLines(Tasks(testutil.KitConfig(t), dir))
	for _, want := range []string{"make\t-\ttest\tmake test", "pdm\tpython\tlint\tpdm run lint"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

// pnpm root with packages/a, a uv project at services/api, and an untracked
// node_modules manifest that must not count.
func monorepo(t *testing.T) string {
	dir := tmp(t)
	testutil.Git(t, dir, "init", "-q", "-b", "main")
	testutil.Put(t, dir, "package.json", `{"name":"root","private":true}`)
	testutil.Put(t, dir, "pnpm-workspace.yaml", "packages:\n  - \"packages/*\"\n")
	testutil.Put(t, dir, "pnpm-lock.yaml", "")
	testutil.Put(t, dir, "packages/a/package.json", `{"name":"a","scripts":{"test":"vitest"}}`)
	testutil.Put(t, dir, "services/api/pyproject.toml", "[project]\nname = \"api\"\n")
	testutil.Put(t, dir, "services/api/uv.lock", "")
	testutil.Put(t, dir, "node_modules/dep/package.json", `{"name":"dep"}`)
	testutil.Git(t, dir, "add", "package.json", "pnpm-workspace.yaml", "pnpm-lock.yaml", "packages", "services")
	return dir
}

func TestSubprojectsRootWorkspaceAndNested(t *testing.T) {
	dir := monorepo(t)
	if got := strings.Join(Subprojects(testutil.KitConfig(t), dir), "\n"); got != ".\npackages/a\nservices/api" {
		t.Errorf("got %q", got)
	}
}

func TestSubprojectsRespectMaxDepth(t *testing.T) {
	dir := tmp(t)
	testutil.Git(t, dir, "init", "-q", "-b", "main")
	testutil.Put(t, dir, "a/b/c/d/package.json", "{}")
	testutil.Put(t, dir, "a/b/c/d/e/package.json", "{}")
	testutil.Git(t, dir, "add", "a")
	if got := strings.Join(Subprojects(testutil.KitConfig(t), dir), "\n"); got != ".\na/b/c/d" {
		t.Errorf("got %q", got)
	}
}

func TestSubprojectsGoWorkAndCargoMembers(t *testing.T) {
	dir := tmp(t)
	testutil.Git(t, dir, "init", "-q", "-b", "main")
	for _, d := range []string{"svc", "tools", "crates/x"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	testutil.Put(t, dir, "go.work", "go 1.22\n\nuse (\n\t./svc\n)\nuse ./tools\n")
	testutil.Put(t, dir, "Cargo.toml", "[workspace]\nmembers = [\"crates/*\"]\n")
	if got := strings.Join(Subprojects(testutil.KitConfig(t), dir), "\n"); got != ".\ncrates/x\nsvc\ntools" {
		t.Errorf("got %q", got)
	}
}

func TestGlobDirsGlobstarSkipsDotDirs(t *testing.T) {
	dir := tmp(t)
	for _, d := range []string{"apps/web", "apps/.hidden", "apps/deep/nested"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	testutil.Put(t, dir, "apps/file", "")
	got := strings.Join(globDirs(dir, "apps/*"), ",")
	if got != "apps/deep,apps/web" {
		t.Errorf("apps/* = %q", got)
	}
	got = strings.Join(globDirs(dir, "apps/**"), ",")
	if got != "apps,apps/deep,apps/deep/nested,apps/web" {
		t.Errorf("apps/** = %q", got)
	}
}

func TestFindUpStopsAtStop(t *testing.T) {
	dir := tmp(t)
	testutil.Put(t, dir, "marker", "")
	testutil.Put(t, dir, "repo/sub/x", "")
	if got := FindUp(filepath.Join(dir, "repo/sub"), filepath.Join(dir, "repo"), "marker"); got != "" {
		t.Errorf("found above stop: %q", got)
	}
	if got := FindUp(filepath.Join(dir, "repo/sub"), dir, "marker"); got != filepath.Join(dir, "marker") {
		t.Errorf("got %q", got)
	}
}

func TestIsScratch(t *testing.T) {
	for path, want := range map[string]bool{
		"/repo/.claude/scratch":            true,
		"/repo/.claude/scratch/":           true,
		"/repo/.claude/scratch/a/b.md":     true,
		"/home/u/.claude/scratch/x.log":    true,
		"/repo/src/scratch/x.go":           false,
		"/repo/scratch/x.go":               false,
		"/repo/.claude/scratchpad/x":       false,
		"/repo/.claude/scratch/../src/x":   false,
		"/repo/.claude/plans/scratch.md":   false,
		"/home/u/.config/claude/scratch":   true,
		"/home/u/.config/claude/scratch/r": true,
		"/home/u/.config/claude/scratchy":  false,
	} {
		if got := IsScratch(config.Paths{Home: "/home/u/.config/claude"}, path); got != want {
			t.Errorf("IsScratch(%q) = %v, want %v", path, got, want)
		}
	}
}
