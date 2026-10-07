package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

// resolveEnv is one Resolve test: a repo dir, a PATH dir that is the whole
// PATH, and the policy kit.yml ships.
type resolveEnv struct {
	t          *testing.T
	repo, path string
	cfg        *config.Config
}

func resolveSetup(t *testing.T) *resolveEnv {
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &resolveEnv{t: t, repo: filepath.Join(tmp, "repo"), path: filepath.Join(tmp, "path")}
	os.MkdirAll(e.repo, 0o755)
	os.MkdirAll(e.path, 0o755)
	t.Setenv("PATH", e.path)
	t.Setenv("VIRTUAL_ENV", "")
	t.Setenv("CONDA_PREFIX", "")
	t.Setenv("ASDF_DATA_DIR", "")
	t.Setenv("MISE_DATA_DIR", "")
	e.cfg = &config.Config{
		PackageManagers: []config.PackageManager{
			{Lockfile: "pnpm-lock.yaml", Manager: "pnpm", Ecosystem: "js"},
			{Lockfile: "uv.lock", Manager: "uv", Ecosystem: "python"},
		},
		ToolResolution: config.ToolResolution{
			PathFallback: []string{"terraform", "shellcheck"},
			PinFiles:     []string{".tool-versions", "mise.toml"},
			PinAliases:   map[string]string{"opentofu": "tofu"},
			ManagerDirs:  []string{"$ASDF_DATA_DIR/shims"},
			Install:      map[string]string{"pnpm": "pnpm install", "uv": "uv sync", "go": "go mod download"},
		},
	}
	return e
}

// exe writes an executable shell script and returns its path.
func (e *resolveEnv) exe(path, body string) string {
	e.t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		e.t.Fatal(err)
	}
	return path
}

func (e *resolveEnv) resolve(name string, mode Mode) Resolution {
	return Resolve(e.cfg, e.repo, e.repo, name, mode)
}

func (e *resolveEnv) wantRuns(res Resolution, words, source string) {
	e.t.Helper()
	if got := strings.Join(res.Words, " "); got != words || res.Source != source {
		e.t.Errorf("got %q (%s), skip %q; want %q (%s)", got, res.Source, res.Skip, words, source)
	}
}

func (e *resolveEnv) wantSkip(res Resolution, parts ...string) {
	e.t.Helper()
	if res.Words != nil {
		e.t.Fatalf("ran %v (%s), want a skip", res.Words, res.Source)
	}
	for _, p := range parts {
		if !strings.Contains(res.Skip, p) {
			e.t.Errorf("skip %q lacks %q", res.Skip, p)
		}
	}
}

func TestResolveLocalCopyWinsOverPATH(t *testing.T) {
	e := resolveSetup(t)
	e.exe(filepath.Join(e.path, "eslint"), "")
	local := e.exe(filepath.Join(e.repo, "node_modules/.bin/eslint"), "")
	e.wantRuns(e.resolve("eslint", Default), local, SourceLocal)
}

func TestResolveGoModToolBuildsOfflineWithoutDownloading(t *testing.T) {
	e := resolveSetup(t)
	put(t, e.repo, "go.mod", "module x\n\ngo 1.24\n\ntool (\n\tgolang.org/x/tools/cmd/deadcode\n)\n")
	built := e.exe(filepath.Join(e.repo, "cache/deadcode"), "")
	env := filepath.Join(e.repo, "env")
	e.exe(filepath.Join(e.path, "go"), `echo "$GOPROXY|$GOFLAGS" >`+env+`
[ "$1 $2 $3" = "tool -n deadcode" ] && echo `+built+`
`)
	e.wantRuns(e.resolve("deadcode", Default), built, SourcePM)
	if got := strings.TrimSpace(readFile(t, env)); got != "off|-mod=readonly" {
		t.Errorf("go ran with GOPROXY|GOFLAGS %q", got)
	}

	os.MkdirAll(filepath.Join(e.repo, "vendor"), 0o755)
	e.resolve("deadcode", Default)
	if got := strings.TrimSpace(readFile(t, env)); got != "off|" {
		t.Errorf("with vendor/, go ran with GOPROXY|GOFLAGS %q", got)
	}
}

func TestResolveGoModToolNotInTheCacheIsDeclaredNotInstalled(t *testing.T) {
	e := resolveSetup(t)
	put(t, e.repo, "go.mod", "module x\n\ntool github.com/golangci/golangci-lint/v2/cmd/golangci-lint\n")
	e.exe(filepath.Join(e.path, "go"), "exit 1\n")
	e.exe(filepath.Join(e.path, "golangci-lint"), "")
	e.wantSkip(e.resolve("golangci-lint", Default), "declared in go.mod but not installed; run go mod download")
}

func TestGoModDeclaresNamesTheBinaryBeforeAMajorSuffix(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "go.mod", "module x\n\ntool example.com/foo/v2\n")
	if !goModDeclares(filepath.Join(dir, "go.mod"), "foo") {
		t.Error("example.com/foo/v2 should provide foo")
	}
}

func TestResolveActiveVirtualenvOnlyInsideTheRepo(t *testing.T) {
	e := resolveSetup(t)
	inside := e.exe(filepath.Join(e.repo, "envs/dev/bin/ruff"), "")
	t.Setenv("VIRTUAL_ENV", filepath.Join(e.repo, "envs/dev"))
	e.wantRuns(e.resolve("ruff", Default), inside, SourcePM)

	outside := filepath.Join(filepath.Dir(e.repo), "elsewhere")
	e.exe(filepath.Join(outside, "bin/ruff"), "")
	t.Setenv("VIRTUAL_ENV", outside)
	res := e.resolve("ruff", Default)
	e.wantSkip(res, "ruff not installed")
}

func TestResolveDeclaredButNotInstalledSkipsEvenWithAPATHCopy(t *testing.T) {
	e := resolveSetup(t)
	e.exe(filepath.Join(e.path, "prettier"), "")
	put(t, e.repo, "package.json", `{"devDependencies":{"prettier":"3.6.2"}}`)
	put(t, e.repo, "pnpm-lock.yaml", "")
	e.wantSkip(e.resolve("prettier", AnyPath), "prettier declared in package.json but not installed; run pnpm install")
}

func TestResolveDeclaredPythonToolNamesTheManagersInstall(t *testing.T) {
	e := resolveSetup(t)
	e.exe(filepath.Join(e.path, "ruff"), "")
	put(t, e.repo, "pyproject.toml", "[dependency-groups]\ndev = [\"ruff>=0.15\"]\n")
	put(t, e.repo, "uv.lock", "")
	e.wantSkip(e.resolve("ruff", Default), "declared in pyproject.toml but not installed; run uv sync")
}

func TestResolveDeclaredInAParentManifest(t *testing.T) {
	e := resolveSetup(t)
	put(t, e.repo, "package.json", `{"devDependencies":{"@biomejs/biome":"2.0.0"}}`)
	e.cfg.ToolResolution.BinPackages = map[string]string{"biome": "@biomejs/biome"}
	sub := filepath.Join(e.repo, "packages/a")
	os.MkdirAll(sub, 0o755)
	res := Resolve(e.cfg, sub, e.repo, "biome", Default)
	e.wantSkip(res, "biome declared in package.json but not installed; run npm install")
}

func TestResolveLocalOnlyNeverTakesPATH(t *testing.T) {
	e := resolveSetup(t)
	e.exe(filepath.Join(e.path, "mypy"), "")
	e.wantSkip(e.resolve("mypy", LocalOnly), "mypy not in the project environment")
}

func TestResolveNothingAnywhereIsMissing(t *testing.T) {
	e := resolveSetup(t)
	res := e.resolve("tsc", Default)
	e.wantSkip(res, "tsc not installed")
	if !res.Missing {
		t.Error("Missing is false")
	}
}

func TestResolvePinnedToolRunsOnlyFromTheVersionManager(t *testing.T) {
	e := resolveSetup(t)
	asdf := filepath.Join(filepath.Dir(e.repo), "asdf")
	t.Setenv("ASDF_DATA_DIR", asdf)
	put(t, e.repo, ".tool-versions", "# tools\nyamllint 1.35.1\n")

	stray := e.exe(filepath.Join(e.path, "yamllint"), "")
	e.wantSkip(e.resolve("yamllint", Default), "pinned in .tool-versions, but PATH has "+stray)

	shim := e.exe(filepath.Join(asdf, "shims/yamllint"), "")
	t.Setenv("PATH", filepath.Join(asdf, "shims")+":"+e.path)
	e.wantRuns(e.resolve("yamllint", Default), shim, SourceManager)
}

func TestResolvePinNamesArePluginNames(t *testing.T) {
	e := resolveSetup(t)
	asdf := filepath.Join(filepath.Dir(e.repo), "asdf")
	t.Setenv("ASDF_DATA_DIR", asdf)
	t.Setenv("PATH", filepath.Join(asdf, "shims"))
	put(t, e.repo, "mise.toml", "[tools]\n\"npm:prettier\" = \"3.6.2\"\nopentofu = \"1.9\"\n")
	for _, name := range []string{"prettier", "tofu"} {
		shim := e.exe(filepath.Join(asdf, "shims", name), "")
		e.wantRuns(e.resolve(name, Default), shim, SourceManager)
	}
}

func TestResolvePathFallbackAndAmbiguousPATHCopies(t *testing.T) {
	e := resolveSetup(t)
	sc := e.exe(filepath.Join(e.path, "shellcheck"), "")
	e.wantRuns(e.resolve("shellcheck", Default), sc, SourcePATH)

	yl := e.exe(filepath.Join(e.path, "yamllint"), "")
	e.wantSkip(e.resolve("yamllint", Default), "yamllint only on PATH ("+yl+")", "path_fallback")
	e.wantRuns(e.resolve("yamllint", AnyPath), yl, SourcePATH)
}

func TestResolveToolchainTakesTheFirstCandidateThatRuns(t *testing.T) {
	e := resolveSetup(t)
	tc := config.ToolchainCheck{Stack: "opentofu", Name: "fmt", Cmd: "{bin} fmt -check", Bin: []string{"tofu", "terraform"}}
	run := ResolveToolchain(e.cfg, tc, e.repo, e.repo)
	e.wantSkip(run.Resolution, "tofu not installed")

	tf := e.exe(filepath.Join(e.path, "terraform"), "")
	run = ResolveToolchain(e.cfg, tc, e.repo, e.repo)
	if got := strings.Join(run.Words, " "); got != tf+" fmt -check" {
		t.Errorf("words %q", got)
	}
	if got := strings.Join(run.Shown, " "); got != "terraform fmt -check" {
		t.Errorf("shown %q", got)
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
