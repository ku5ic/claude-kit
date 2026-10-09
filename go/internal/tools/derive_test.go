package tools

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

func adapter(name string) Adapter {
	for _, a := range builtins {
		if a.Name == name {
			return a
		}
	}
	panic(name)
}

func kitConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, _, err := config.Load(config.Paths{Base: "../../../kit.yml", Overlay: filepath.Join(t.TempDir(), "none.yml")})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestDeriveFromPackageScripts(t *testing.T) {
	cfg := kitConfig(t)
	dir := t.TempDir()
	put(t, dir, "package.json", `{"scripts":{
		"eslint:fix": "eslint --fix --max-warnings 1 .",
		"eslint": "eslint --cache src/** --max-warnings 715 -o report.txt",
		"test:ci": "NODE_OPTIONS=\"${NODE_OPTIONS:-}\" jest --maxWorkers 4",
		"test": "NODE_ENV=test GOFLAGS=-toolexec=./x cross-env DEBUG=0 PYTHONPATH=.hooks jest --maxWorkers 2 src"
	}}`)
	d, ok := adapter("eslint").Derive(cfg, dir, dir)
	if !ok || d.Source != "npm run eslint" {
		t.Fatalf("eslint: %+v %v (the :fix script must be skipped)", d, ok)
	}
	if want := []string{"--cache", "--max-warnings", "715"}; !slices.Equal(d.Flags, want) {
		t.Errorf("eslint flags = %q, want %q (paths and -o never carry)", d.Flags, want)
	}
	d, _ = adapter("jest").Derive(cfg, dir, dir)
	if d.Source != "npm run test" {
		t.Errorf("jest source = %q (a command with an expansion can't be read)", d.Source)
	}
	if want := []string{"NODE_ENV=test", "DEBUG=0"}; !slices.Equal(d.Env, want) {
		t.Errorf("jest env = %q, want %q (an unlisted name is dropped)", d.Env, want)
	}
	if want := []string{"--maxWorkers", "2"}; !slices.Equal(d.Flags, want) {
		t.Errorf("jest flags = %q", d.Flags)
	}
}

func TestDeriveThroughRunnersAndMakeVariables(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "GNUmakefile", "RUN = uv run\n\nformat:\n\t$(RUN) ruff format .\n\nlint:\n\t@$(RUN) ruff check \\\n\t  --select E,F .\n")
	d, ok := adapter("ruff").Derive(kitConfig(t), dir, dir)
	if !ok || d.Source != "make lint" {
		t.Fatalf("%+v %v: ruff format isn't ruff check, and format is not a check target", d, ok)
	}
	if want := []string{"--select", "E,F"}; !slices.Equal(d.Flags, want) {
		t.Errorf("flags = %q", d.Flags)
	}
}

func TestDeriveFromCIStepInItsWorkingDirectory(t *testing.T) {
	cfg := kitConfig(t)
	root := t.TempDir()
	put(t, root, ".github/workflows/ci.yml", `
jobs:
  backend:
    defaults:
      run:
        working-directory: backend
    env:
      CI: "true"
    steps:
      - name: Typecheck
        run: uv run pyright --pythonversion 3.12 apps/
  other:
    steps:
      - run: pyright --level error
`)
	d, ok := adapter("pyright").Derive(cfg, root+"/backend", root)
	if !ok || !slices.Equal(d.Flags, []string{"--pythonversion", "3.12"}) || !slices.Equal(d.Env, []string{"CI=true"}) {
		t.Fatalf("backend: %+v %v", d, ok)
	}
	if d, _ := adapter("pyright").Derive(cfg, root, root); !slices.Equal(d.Flags, []string{"--level", "error"}) || d.Source != ".github/workflows/ci.yml" {
		t.Errorf("root: %+v", d)
	}
}

func TestDeriveCIPrefersAStepWithFlags(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".github/workflows/ci.yml", `
jobs:
  zeta:
    steps:
      - run: eslint --max-warnings 0 .
  alpha:
    steps:
      - run: eslint --quiet .
`)
	if d, ok := adapter("eslint").Derive(kitConfig(t), root, root); !ok || !slices.Equal(d.Flags, []string{"--max-warnings", "0"}) {
		t.Fatalf("%+v %v", d, ok)
	}
}

func TestDeriveFromGitLabAndThroughAShorthand(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".gitlab-ci.yml", "lint:\n  script:\n    - pnpm eslint --max-warnings 3 .\n")
	d, ok := adapter("eslint").Derive(kitConfig(t), root, root)
	if !ok || d.Source != ".gitlab-ci.yml" || !slices.Equal(d.Flags, []string{"--max-warnings", "3"}) {
		t.Fatalf("%+v %v: with no eslint script, pnpm eslint runs the binary", d, ok)
	}
}

func TestDeriveSkipsACIStepUsingSecrets(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".github/workflows/ci.yml", `
jobs:
  lint:
    steps:
      - run: eslint --max-warnings 0 .
        env:
          TOKEN: ${{ secrets.TOKEN }}
`)
	if d, ok := adapter("eslint").Derive(kitConfig(t), root, root); ok {
		t.Errorf("%+v: a step a local run can't repeat derives nothing", d)
	}
}

func TestDeriveNothingFound(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "package.json", `{"scripts":{"lint":"biome check ."}}`)
	if _, ok := adapter("eslint").Derive(kitConfig(t), dir, dir); ok {
		t.Error("no eslint invocation: nothing to derive")
	}
}
