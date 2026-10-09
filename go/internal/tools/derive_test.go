package tools

import (
	"slices"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// adapter is the built-in as kit.yml completes it.
func adapter(t *testing.T, name string) Adapter {
	t.Helper()
	for _, a := range All(testutil.KitConfig(t)) {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("no adapter %q", name)
	return Adapter{}
}

func TestDeriveFromPackageScripts(t *testing.T) {
	cfg := testutil.KitConfig(t)
	dir := t.TempDir()
	testutil.Put(t, dir, "package.json", `{"scripts":{
		"eslint:fix": "eslint --fix --max-warnings 1 .",
		"eslint": "eslint --cache src/** --max-warnings 715 -o report.txt",
		"test:ci": "NODE_OPTIONS=\"${NODE_OPTIONS:-}\" jest --maxWorkers 4",
		"test": "NODE_ENV=test GOFLAGS=-toolexec=./x cross-env DEBUG=0 PYTHONPATH=.hooks jest --maxWorkers 2 src"
	}}`)
	d, ok := adapter(t, "eslint").Derive(cfg, dir, dir)
	if !ok || d.Source != "npm run eslint" {
		t.Fatalf("eslint: %+v %v (the :fix script must be skipped)", d, ok)
	}
	if want := []string{"--cache", "--max-warnings", "715"}; !slices.Equal(d.Flags, want) {
		t.Errorf("eslint flags = %q, want %q (paths and -o never carry)", d.Flags, want)
	}
	d, _ = adapter(t, "jest").Derive(cfg, dir, dir)
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
	testutil.Put(t, dir, "GNUmakefile", "RUN = uv run\n\nformat:\n\t$(RUN) ruff format .\n\nlint:\n\t@$(RUN) ruff check \\\n\t  --select E,F .\n")
	d, ok := adapter(t, "ruff").Derive(testutil.KitConfig(t), dir, dir)
	if !ok || d.Source != "make lint" {
		t.Fatalf("%+v %v: ruff format isn't ruff check, and format is not a check target", d, ok)
	}
	if want := []string{"--select", "E,F"}; !slices.Equal(d.Flags, want) {
		t.Errorf("flags = %q", d.Flags)
	}
}

func TestDeriveFromCIStepInItsWorkingDirectory(t *testing.T) {
	cfg := testutil.KitConfig(t)
	root := t.TempDir()
	testutil.Put(t, root, ".github/workflows/ci.yml", `
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
	d, ok := adapter(t, "pyright").Derive(cfg, root+"/backend", root)
	if !ok || !slices.Equal(d.Flags, []string{"--pythonversion", "3.12"}) || !slices.Equal(d.Env, []string{"CI=true"}) {
		t.Fatalf("backend: %+v %v", d, ok)
	}
	if d, _ := adapter(t, "pyright").Derive(cfg, root, root); !slices.Equal(d.Flags, []string{"--level", "error"}) || d.Source != ".github/workflows/ci.yml" {
		t.Errorf("root: %+v", d)
	}
}

func TestDeriveCIPrefersAStepWithFlags(t *testing.T) {
	root := t.TempDir()
	testutil.Put(t, root, ".github/workflows/ci.yml", `
jobs:
  zeta:
    steps:
      - run: eslint --max-warnings 0 .
  alpha:
    steps:
      - run: eslint --quiet .
`)
	if d, ok := adapter(t, "eslint").Derive(testutil.KitConfig(t), root, root); !ok || !slices.Equal(d.Flags, []string{"--max-warnings", "0"}) {
		t.Fatalf("%+v %v", d, ok)
	}
}

func TestDeriveFromGitLabAndThroughAShorthand(t *testing.T) {
	root := t.TempDir()
	testutil.Put(t, root, ".gitlab-ci.yml", "lint:\n  script:\n    - pnpm eslint --max-warnings 3 .\n")
	d, ok := adapter(t, "eslint").Derive(testutil.KitConfig(t), root, root)
	if !ok || d.Source != ".gitlab-ci.yml" || !slices.Equal(d.Flags, []string{"--max-warnings", "3"}) {
		t.Fatalf("%+v %v: with no eslint script, pnpm eslint runs the binary", d, ok)
	}
}

func TestDeriveSkipsACIStepUsingSecrets(t *testing.T) {
	root := t.TempDir()
	testutil.Put(t, root, ".github/workflows/ci.yml", `
jobs:
  lint:
    steps:
      - run: eslint --max-warnings 0 .
        env:
          TOKEN: ${{ secrets.TOKEN }}
`)
	if d, ok := adapter(t, "eslint").Derive(testutil.KitConfig(t), root, root); ok {
		t.Errorf("%+v: a step a local run can't repeat derives nothing", d)
	}
}

func TestDeriveNothingFound(t *testing.T) {
	dir := t.TempDir()
	testutil.Put(t, dir, "package.json", `{"scripts":{"lint":"biome check ."}}`)
	if _, ok := adapter(t, "eslint").Derive(testutil.KitConfig(t), dir, dir); ok {
		t.Error("no eslint invocation: nothing to derive")
	}
}
