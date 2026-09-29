package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/dotfiles/claude-kit/go/internal/config"
)

// Ports providers.bats' kit_tasks and kit_subprojects tests against the real
// kit.yml.

func realConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, _, err := config.Load(config.Paths{Base: "../../../kit.yml", Overlay: filepath.Join(t.TempDir(), "none.yml")})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func tmp(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func put(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
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
	git(t, dir, "init", "-q", "-b", "main")
	put(t, dir, "package.json", `{"scripts":{"test":"vitest"}}`)
	put(t, dir, "pnpm-lock.yaml", "")
	if got := taskLines(Tasks(realConfig(t), dir)); got != "package-scripts\tjs\ttest\tpnpm run test" {
		t.Errorf("got %q", got)
	}
}

func TestTasksDefaultPMIsNpm(t *testing.T) {
	dir := tmp(t)
	put(t, dir, "package.json", `{"scripts":{"lint":"eslint ."}}`)
	if got := taskLines(Tasks(realConfig(t), dir)); got != "package-scripts\tjs\tlint\tnpm run lint" {
		t.Errorf("got %q", got)
	}
}

func TestTasksRunByPMForPoeUnderPoetry(t *testing.T) {
	dir := tmp(t)
	put(t, dir, "pyproject.toml", "[tool.poe.tasks]\ntest = \"pytest\"\n")
	put(t, dir, "poetry.lock", "")
	if got := taskLines(Tasks(realConfig(t), dir)); got != "poe\tpython\ttest\tpoetry run poe test" {
		t.Errorf("got %q", got)
	}
}

func TestTasksSeveralProviders(t *testing.T) {
	dir := tmp(t)
	put(t, dir, "Makefile", "test:\n\tgo test\n")
	put(t, dir, "pyproject.toml", "[tool.pdm.scripts]\nlint = \"ruff\"\n")
	got := taskLines(Tasks(realConfig(t), dir))
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
	git(t, dir, "init", "-q", "-b", "main")
	put(t, dir, "package.json", `{"name":"root","private":true}`)
	put(t, dir, "pnpm-workspace.yaml", "packages:\n  - \"packages/*\"\n")
	put(t, dir, "pnpm-lock.yaml", "")
	put(t, dir, "packages/a/package.json", `{"name":"a","scripts":{"test":"vitest"}}`)
	put(t, dir, "services/api/pyproject.toml", "[project]\nname = \"api\"\n")
	put(t, dir, "services/api/uv.lock", "")
	put(t, dir, "node_modules/dep/package.json", `{"name":"dep"}`)
	git(t, dir, "add", "package.json", "pnpm-workspace.yaml", "pnpm-lock.yaml", "packages", "services")
	return dir
}

func TestSubprojectsRootWorkspaceAndNested(t *testing.T) {
	dir := monorepo(t)
	if got := strings.Join(Subprojects(realConfig(t), dir), "\n"); got != ".\npackages/a\nservices/api" {
		t.Errorf("got %q", got)
	}
}

func TestSubprojectsRespectMaxDepth(t *testing.T) {
	dir := tmp(t)
	git(t, dir, "init", "-q", "-b", "main")
	put(t, dir, "a/b/c/d/package.json", "{}")
	put(t, dir, "a/b/c/d/e/package.json", "{}")
	git(t, dir, "add", "a")
	if got := strings.Join(Subprojects(realConfig(t), dir), "\n"); got != ".\na/b/c/d" {
		t.Errorf("got %q", got)
	}
}

func TestSubprojectsGoWorkAndCargoMembers(t *testing.T) {
	dir := tmp(t)
	git(t, dir, "init", "-q", "-b", "main")
	for _, d := range []string{"svc", "tools", "crates/x"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	put(t, dir, "go.work", "go 1.22\n\nuse (\n\t./svc\n)\nuse ./tools\n")
	put(t, dir, "Cargo.toml", "[workspace]\nmembers = [\"crates/*\"]\n")
	if got := strings.Join(Subprojects(realConfig(t), dir), "\n"); got != ".\ncrates/x\nsvc\ntools" {
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
	put(t, dir, "apps/file", "")
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
	put(t, dir, "marker", "")
	put(t, dir, "repo/sub/x", "")
	if got := FindUp(filepath.Join(dir, "repo/sub"), filepath.Join(dir, "repo"), "marker"); got != "" {
		t.Errorf("found above stop: %q", got)
	}
	if got := FindUp(filepath.Join(dir, "repo/sub"), dir, "marker"); got != filepath.Join(dir, "marker") {
		t.Errorf("got %q", got)
	}
}
