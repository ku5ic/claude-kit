package resolve

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// env is one Resolve test: a git repo ignoring node_modules, .venv, and
// bin, and a PATH dir that is the whole PATH, git aside.
type env struct {
	t          *testing.T
	repo, path string
	managers   []string
	userPins   []string
}

func setup(t *testing.T) *env {
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, repo: filepath.Join(tmp, "repo"), path: filepath.Join(tmp, "path")}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	testutil.Put(t, e.path, ".keep", "")
	if err := os.Symlink(git, filepath.Join(e.path, "git")); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"PATH": e.path, "VIRTUAL_ENV": "", "CONDA_PREFIX": "", "ASDF_DATA_DIR": "", "MISE_DATA_DIR": "", "XDG_DATA_HOME": "", "HOMEBREW_PREFIX": "", "HOME": tmp} {
		t.Setenv(key, value)
	}
	testutil.Put(t, e.repo, ".gitignore", "node_modules\n.venv\n/bin\n")
	testutil.Git(t, e.repo, "init", "-q")
	return e
}

// exe writes an executable shell script and returns its path.
func (e *env) exe(path string) string {
	e.t.Helper()
	testutil.Put(e.t, filepath.Dir(path), filepath.Base(path), "#!/bin/sh\n")
	if err := os.Chmod(path, 0o755); err != nil {
		e.t.Fatal(err)
	}
	return path
}

func (e *env) bin(dir, name string) Resolution {
	return New(e.repo, e.managers, e.userPins).Bin(dir, name)
}

func (e *env) wantRuns(res Resolution, path, source string) {
	e.t.Helper()
	if res.Path != path || res.Source != source {
		e.t.Errorf("got %q (%s), skip %q; want %q (%s)", res.Path, res.Source, res.Skip, path, source)
	}
}

func (e *env) wantSkip(res Resolution, parts ...string) {
	e.t.Helper()
	if res.Path != "" {
		e.t.Fatalf("ran %s (%s), want a skip", res.Path, res.Source)
	}
	for _, p := range parts {
		if !strings.Contains(res.Skip, p) {
			e.t.Errorf("skip %q lacks %q", res.Skip, p)
		}
	}
}

func TestLocalCopyWinsOverPATH(t *testing.T) {
	e := setup(t)
	e.exe(filepath.Join(e.path, "eslint"))
	local := e.exe(filepath.Join(e.repo, "node_modules/.bin/eslint"))
	e.wantRuns(e.bin(filepath.Join(e.repo, "src"), "eslint"), local, Local)
}

func TestEveryIgnoredBinDirectoryIsLocalAndATrackedOneIsNot(t *testing.T) {
	e := setup(t)
	venv := e.exe(filepath.Join(e.repo, ".venv/bin/ruff"))
	tools := e.exe(filepath.Join(e.repo, "bin/golangci-lint"))
	e.wantRuns(e.bin(e.repo, "ruff"), venv, Local)
	e.wantRuns(e.bin(e.repo, "golangci-lint"), tools, Local)

	e.exe(filepath.Join(e.repo, "scripts/bin/fmt"))
	e.wantSkip(e.bin(e.repo, "fmt"), "fmt not installed")
}

func TestAPathRunsOnlyInsideTheProject(t *testing.T) {
	e := setup(t)
	script := e.exe(filepath.Join(e.repo, "build.sh"))
	e.wantRuns(e.bin(e.repo, "./build.sh"), script, Local)
	outside := e.exe(filepath.Join(filepath.Dir(e.repo), "x.sh"))
	e.wantSkip(e.bin(e.repo, outside), "not found in the project")
}

func TestGoModToolBuildsOfflineWithoutDownloading(t *testing.T) {
	e := setup(t)
	testutil.Put(t, e.repo, "go.mod", "module x\n\ngo 1.24\n\ntool (\n\tgolang.org/x/tools/cmd/deadcode\n)\n")
	built := e.exe(filepath.Join(e.repo, "cache/deadcode"))
	envFile := filepath.Join(e.repo, "env")
	testutil.Put(t, e.path, "go", `#!/bin/sh
echo "$GOPROXY|$GOFLAGS" >`+envFile+`
[ "$1 $2 $3" = "tool -n deadcode" ] && echo `+built+"\n")
	if err := os.Chmod(filepath.Join(e.path, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.wantRuns(e.bin(e.repo, "deadcode"), built, GoModTool)
	if got := strings.TrimSpace(testutil.Read(t, envFile)); got != "off|-mod=readonly" {
		t.Errorf("go ran with GOPROXY|GOFLAGS %q", got)
	}
}

func TestGoModToolNotInTheCacheIsDeclaredNotInstalled(t *testing.T) {
	e := setup(t)
	testutil.Put(t, e.repo, "go.mod", "module x\n\ntool github.com/golangci/golangci-lint/v2/cmd/golangci-lint\n")
	testutil.Put(t, e.path, "go", "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(filepath.Join(e.path, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.exe(filepath.Join(e.path, "golangci-lint"))
	e.wantSkip(e.bin(e.repo, "golangci-lint"), "golangci-lint declared in go.mod but not installed")
}

func TestGoModDeclaresNamesTheBinaryBeforeAMajorSuffix(t *testing.T) {
	dir := t.TempDir()
	testutil.Put(t, dir, "go.mod", "module x\n\ntool example.com/foo/v2\n")
	if !GoModDeclares(filepath.Join(dir, "go.mod"), "foo") {
		t.Error("example.com/foo/v2 should provide foo")
	}
}

func TestActiveVirtualenvOnlyInsideTheProject(t *testing.T) {
	e := setup(t)
	inside := e.exe(filepath.Join(e.repo, "envs/dev/bin/ruff"))
	t.Setenv("VIRTUAL_ENV", filepath.Join(e.repo, "envs/dev"))
	e.wantRuns(e.bin(e.repo, "ruff"), inside, Environment)

	outside := filepath.Join(filepath.Dir(e.repo), "elsewhere")
	e.exe(filepath.Join(outside, "bin/ruff"))
	t.Setenv("VIRTUAL_ENV", outside)
	e.wantSkip(e.bin(e.repo, "ruff"), "ruff not installed")
}

func TestDeclaredButNotInstalledSkipsEvenWithAPATHCopy(t *testing.T) {
	e := setup(t)
	e.exe(filepath.Join(e.path, "prettier"))
	testutil.Put(t, e.repo, "package.json", `{"devDependencies":{"prettier":"3.6.2"}}`)
	e.wantSkip(e.bin(e.repo, "prettier"), "prettier declared in package.json but not installed")

	e.exe(filepath.Join(e.path, "ruff"))
	testutil.Put(t, e.repo, "pyproject.toml", "[dependency-groups]\ndev = [\"ruff>=0.15\"]\n")
	e.wantSkip(e.bin(e.repo, "ruff"), "ruff declared in pyproject.toml but not installed")
}

func TestDeclaredInAParentManifest(t *testing.T) {
	e := setup(t)
	testutil.Put(t, e.repo, "package.json", `{"devDependencies":{"vitest":"2.0.0"}}`)
	e.wantSkip(e.bin(filepath.Join(e.repo, "packages/a"), "vitest"), "vitest declared in package.json but not installed")
}

func TestPATHOnlyIsRefused(t *testing.T) {
	e := setup(t)
	yl := e.exe(filepath.Join(e.path, "yamllint"))
	e.wantSkip(e.bin(e.repo, "yamllint"), "yamllint only on PATH ("+yl+"); nothing in the project pins or provides it")
	e.wantSkip(e.bin(e.repo, "tsc"), "tsc not installed")
}

func TestAManifestsToolchainRunsFromPATH(t *testing.T) {
	e := setup(t)
	gofmt := e.exe(filepath.Join(e.path, "gofmt"))
	e.wantSkip(e.bin(e.repo, "gofmt"), "only on PATH")
	testutil.Put(t, e.repo, "go.mod", "module x\n")
	e.wantRuns(e.bin(filepath.Join(e.repo, "cmd"), "gofmt"), gofmt, Toolchain)
}

func TestATaskRunnersManifestStatesTheRunner(t *testing.T) {
	e := setup(t)
	just := e.exe(filepath.Join(e.path, "just"))
	e.wantSkip(e.bin(e.repo, "just"), "only on PATH")
	testutil.Put(t, e.repo, "justfile", "lint:\n    echo\n")
	e.wantRuns(e.bin(e.repo, "just"), just, Toolchain)
}

func TestTheSystemsOwnCommandsRun(t *testing.T) {
	e := setup(t)
	t.Setenv("PATH", e.path+":/usr/bin:/bin")
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh in /usr/bin or /bin")
	}
	e.wantRuns(e.bin(e.repo, "sh"), sh, System)
}

func TestAVerifiedManagerRunsFromPATH(t *testing.T) {
	e := setup(t)
	pnpm := e.exe(filepath.Join(e.path, "pnpm"))
	e.wantSkip(e.bin(e.repo, "pnpm"), "only on PATH")
	e.managers = []string{"pnpm"}
	e.wantRuns(e.bin(e.repo, "pnpm"), pnpm, Manager)
}

func TestPinnedToolRunsOnlyFromItsManager(t *testing.T) {
	e := setup(t)
	asdf := filepath.Join(filepath.Dir(e.repo), "asdf")
	t.Setenv("ASDF_DATA_DIR", asdf)
	testutil.Put(t, e.repo, ".tool-versions", "# tools\nyamllint 1.35.1\n")

	stray := e.exe(filepath.Join(e.path, "yamllint"))
	e.wantSkip(e.bin(e.repo, "yamllint"), "pinned in .tool-versions, but PATH has "+stray)

	shim := e.exe(filepath.Join(asdf, "shims/yamllint"))
	t.Setenv("PATH", filepath.Join(asdf, "shims")+":"+e.path)
	e.wantRuns(e.bin(e.repo, "yamllint"), shim, Pinned)
}

func TestPinNamesArePluginNames(t *testing.T) {
	e := setup(t)
	mise := filepath.Join(filepath.Dir(e.repo), "mise")
	t.Setenv("MISE_DATA_DIR", mise)
	t.Setenv("PATH", filepath.Join(mise, "shims")+":"+e.path)
	testutil.Put(t, e.repo, "mise.toml", "[tools]\n\"npm:prettier\" = \"3.6.2\"\nopentofu = \"1.9\"\n")
	for _, name := range []string{"prettier", "tofu"} {
		shim := e.exe(filepath.Join(mise, "shims", name))
		e.wantRuns(e.bin(e.repo, name), shim, Pinned)
	}
}

func TestABrewfileFormulaRunsFromHomebrew(t *testing.T) {
	e := setup(t)
	prefix := filepath.Join(filepath.Dir(e.repo), "brew")
	t.Setenv("HOMEBREW_PREFIX", prefix)
	testutil.Put(t, e.repo, "Brewfile", "brew 'shellcheck'  # shell lint\n")
	real := e.exe(filepath.Join(prefix, "Cellar/shellcheck/0.11.0/bin/shellcheck"))
	link := filepath.Join(prefix, "bin/shellcheck")
	testutil.Put(t, filepath.Dir(link), ".keep", "")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(link)+":"+e.path)
	e.wantRuns(e.bin(e.repo, "shellcheck"), link, Pinned)
}

// A user_pins file pins for every project, after the project's own.
func TestAUserPinCountsForEveryProject(t *testing.T) {
	e := setup(t)
	prefix := filepath.Join(filepath.Dir(e.repo), "brew")
	t.Setenv("HOMEBREW_PREFIX", prefix)
	real := e.exe(filepath.Join(prefix, "Cellar/jq/1.8.1/bin/jq"))
	t.Setenv("PATH", filepath.Dir(real)+":"+e.path)
	e.wantSkip(e.bin(e.repo, "jq"), "only on PATH")

	dotfiles := filepath.Join(filepath.Dir(e.repo), "dotfiles")
	testutil.Put(t, dotfiles, "Brewfile", "brew 'jq'  # JSON\n")
	e.userPins = []string{filepath.Join(dotfiles, "Brewfile")}
	e.wantRuns(e.bin(e.repo, "jq"), real, Pinned)
}

// A user pin only allows: a formula named like a binary PATH finds outside
// Homebrew (brew 'grep' installs ggrep) leaves that binary to the other
// rules.
func TestAUserPinNeverRefuses(t *testing.T) {
	e := setup(t)
	dotfiles := filepath.Join(filepath.Dir(e.repo), "dotfiles")
	testutil.Put(t, dotfiles, "Brewfile", "brew 'yamllint'\n")
	e.userPins = []string{filepath.Join(dotfiles, "Brewfile")}
	stray := e.exe(filepath.Join(e.path, "yamllint"))
	e.wantSkip(e.bin(e.repo, "yamllint"), "only on PATH ("+stray+")")
}

// install puts prettier at version under dir/node_modules, its binary in
// dir/node_modules/.bin, and returns the binary's path.
func (e *env) install(dir, version string) string {
	e.t.Helper()
	testutil.Put(e.t, dir, "node_modules/prettier/package.json", `{"name":"prettier","version":"`+version+`"}`)
	e.exe(filepath.Join(dir, "node_modules/prettier/bin/prettier.cjs"))
	link := filepath.Join(dir, "node_modules/.bin/prettier")
	testutil.Put(e.t, filepath.Dir(link), ".keep", "")
	if err := os.Symlink("../prettier/bin/prettier.cjs", link); err != nil {
		e.t.Fatal(err)
	}
	return link
}

// workspace is a root package.json plus packages/a declaring prettier spec;
// it returns packages/a's dir.
func (e *env) workspace(spec string) string {
	e.t.Helper()
	testutil.Put(e.t, e.repo, "package.json", `{"private":true}`)
	a := filepath.Join(e.repo, "packages/a")
	testutil.Put(e.t, a, "package.json", `{"devDependencies":{"prettier":"`+spec+`"}}`)
	return a
}

func TestWorkspacePackageCopyWinsWhenItSatisfies(t *testing.T) {
	e := setup(t)
	a := e.workspace("^3.6.0")
	e.install(e.repo, "3.9.6")
	own := e.install(a, "3.6.2")
	e.wantRuns(e.bin(filepath.Join(a, "src"), "prettier"), own, Local)
}

func TestStaleOwnCopySkips(t *testing.T) {
	e := setup(t)
	a := e.workspace("^3.6.0")
	e.install(a, "3.5.0")
	e.wantSkip(e.bin(a, "prettier"), "packages/a/package.json declares prettier ^3.6.0, installed is 3.5.0 at packages/a; reinstall the project's packages")
}

func TestHoistedCopyRunsOnlyWhenItSatisfiesThePackage(t *testing.T) {
	e := setup(t)
	a := e.workspace("~3.6.0")
	hoisted := e.install(e.repo, "3.6.2")
	e.wantRuns(e.bin(a, "prettier"), hoisted, Local)
	testutil.Put(t, e.repo, "node_modules/prettier/package.json", `{"name":"prettier","version":"3.9.6"}`)
	e.wantSkip(e.bin(a, "prettier"), "packages/a/package.json declares prettier ~3.6.0, installed is 3.9.6 at .")
}

func TestUnreadableSpecRunsWithANote(t *testing.T) {
	e := setup(t)
	a := e.workspace("workspace:*")
	hoisted := e.install(e.repo, "3.9.6")
	res := e.bin(a, "prettier")
	e.wantRuns(res, hoisted, Local)
	if !strings.Contains(res.Note, "workspace:* not checked") {
		t.Errorf("note %q", res.Note)
	}
}

// The link a node_modules/.bin entry is names its package, so a binary
// named apart from it (biome, of @biomejs/biome) is checked too.
func TestTheBinLinkNamesItsPackage(t *testing.T) {
	e := setup(t)
	testutil.Put(t, e.repo, "package.json", `{"devDependencies":{"@biomejs/biome":"^2.0.0"}}`)
	testutil.Put(t, e.repo, "node_modules/@biomejs/biome/package.json", `{"version":"1.9.4"}`)
	e.exe(filepath.Join(e.repo, "node_modules/@biomejs/biome/bin/biome"))
	testutil.Put(t, e.repo, "node_modules/.bin/.keep", "")
	if err := os.Symlink("../@biomejs/biome/bin/biome", filepath.Join(e.repo, "node_modules/.bin/biome")); err != nil {
		t.Fatal(err)
	}
	e.wantSkip(e.bin(e.repo, "biome"), "package.json declares @biomejs/biome ^2.0.0, installed is 1.9.4 at .")
}

func TestSatisfiesFollowsNpmRangeRules(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		spec, version string
		want          Range
	}{
		{"^3.6.0", "3.7.0-beta.1", OutOfRange}, // a plain range never takes a prerelease
		{"^3.7.0-beta.0", "3.7.0-beta.1", InRange},
		{">=3.6 <4", "3.9.6", InRange},
		{"~3.6.0", "3.7.0", OutOfRange},
		{"3.6.2", "3.6.2", InRange},
		{"*", "1.0.0", InRange},
		{"npm:other@^1", "1.0.0", Unchecked},
	} {
		testutil.Put(t, dir, "node_modules/p/package.json", `{"version":"`+c.version+`"}`)
		if _, got := Satisfies(dir, "p", c.spec); got != c.want {
			t.Errorf("%s vs %s: range %d, want %d", c.spec, c.version, got, c.want)
		}
	}
}

func TestCommandNeverFetches(t *testing.T) {
	e := setup(t)
	r := New(e.repo, nil, nil)
	for words, why := range map[string]string{
		"npx prettier --check .":       "npx prettier with no local copy",
		"npm exec -- eslint .":         "npm exec eslint with no local copy",
		"pnpm dlx prettier .":          "pnpm dlx",
		"yarn dlx eslint":              "yarn dlx",
		"bunx eslint":                  "bunx",
		"uvx ruff check":               "uvx",
		"pipx run black .":             "pipx run",
		"go run x.dev/lint@v1.2 ./...": "go run x.dev/lint@v1.2",
	} {
		if res := r.Command(e.repo, strings.Fields(words)); res.Skip != "fetches a package ("+why+")" {
			t.Errorf("%s: %+v", words, res)
		}
	}
	e.exe(filepath.Join(e.repo, "node_modules/.bin/prettier"))
	e.exe(filepath.Join(e.path, "npx"))
	testutil.Put(t, e.repo, "package.json", `{}`)
	if res := New(e.repo, nil, nil).Command(e.repo, strings.Fields("npx prettier --check .")); res.Source != Toolchain {
		t.Errorf("npx with a local copy runs, npx from package.json's toolchain: %+v", res)
	}
}
