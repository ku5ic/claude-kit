package tools

import (
	"slices"
	"testing"
)

func adapter(name string) Adapter {
	for _, a := range builtins {
		if a.Name == name {
			return a
		}
	}
	panic(name)
}

func TestDeriveFromPackageScripts(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "package.json", `{"scripts":{
		"eslint:fix": "eslint --fix --max-warnings 1 .",
		"eslint": "eslint --cache **/{src,tests}/**/ --max-warnings 715 -o report.txt",
		"test": "NODE_OPTIONS=\"${NODE_OPTIONS:-}\" NODE_ENV=test GOFLAGS=-toolexec=./x cross-env DEBUG=0 PYTHONPATH=.hooks jest --maxWorkers 2 src"
	}}`)
	d, ok := adapter("eslint").Derive(dir, dir)
	if !ok || d.Source != "package.json scripts.eslint" {
		t.Fatalf("eslint: %+v %v (the :fix script must be skipped)", d, ok)
	}
	if want := []string{"--cache", "--max-warnings", "715"}; !slices.Equal(d.Flags, want) {
		t.Errorf("eslint flags = %q, want %q (paths and -o never carry)", d.Flags, want)
	}
	d, _ = adapter("jest").Derive(dir, dir)
	if want := []string{"NODE_ENV=test", "DEBUG=0"}; !slices.Equal(d.Env, want) {
		t.Errorf("jest env = %q, want %q (a non-literal value or an unlisted name is dropped)", d.Env, want)
	}
	if want := []string{"--maxWorkers", "2"}; !slices.Equal(d.Flags, want) {
		t.Errorf("jest flags = %q", d.Flags)
	}
}

func TestDeriveThroughRunnersAndMakeVariables(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "Makefile", "RUN = uv run --no-dev\n\nformat:\n\t$(RUN) ruff format .\n\nlint:\n\t@$(RUN) ruff check --select E,F .\n")
	d, ok := adapter("ruff").Derive(dir, dir)
	if !ok || d.Source != "Makefile target lint" {
		t.Fatalf("%+v %v: ruff format isn't ruff check, and format is not a check target", d, ok)
	}
	if want := []string{"--select", "E,F"}; !slices.Equal(d.Flags, want) {
		t.Errorf("flags = %q", d.Flags)
	}
}

func TestDeriveFromCIStepInItsWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".github/workflows/ci.yml", `
jobs:
  backend:
    defaults:
      run:
        working-directory: backend
    steps:
      - name: Typecheck
        run: uv run pyright --pythonversion 3.12 apps/
  other:
    steps:
      - run: pyright --level error
`)
	d, ok := adapter("pyright").Derive(root+"/backend", root)
	if !ok || !slices.Equal(d.Flags, []string{"--pythonversion", "3.12"}) {
		t.Fatalf("backend: %+v %v", d, ok)
	}
	if d, _ := adapter("pyright").Derive(root, root); !slices.Equal(d.Flags, []string{"--level", "error"}) {
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
	if d, ok := adapter("eslint").Derive(root, root); !ok || !slices.Equal(d.Flags, []string{"--max-warnings", "0"}) {
		t.Fatalf("%+v %v", d, ok)
	}
}

func TestDeriveNothingFound(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "package.json", `{"scripts":{"lint":"biome check ."}}`)
	if _, ok := adapter("eslint").Derive(dir, dir); ok {
		t.Error("no eslint invocation: nothing to derive")
	}
}
