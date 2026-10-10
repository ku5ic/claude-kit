package tools

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestTestRunnerChoice(t *testing.T) {
	dir := t.TempDir()
	testutil.Put(t, dir, "package.json", `{"devDependencies":{"jest":"1","vitest":"1"},"scripts":{"test":"jest --ci"}}`)
	byName := map[string]Adapter{}
	for _, a := range builtins {
		byName[a.Name] = a
	}
	testutil.Put(t, dir, "a.ts", "")
	if _, ok := byName["jest"].Claims(filepath.Join(dir, "a.ts"), dir); !ok {
		t.Error("jest, named by the test script, should claim")
	}
	if _, ok := byName["vitest"].Claims(filepath.Join(dir, "a.ts"), dir); ok {
		t.Error("vitest, beside jest and not in the test script, should not claim")
	}
	testutil.Put(t, dir, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	if _, ok := byName["vitest"].Claims(filepath.Join(dir, "a.ts"), dir); !ok {
		t.Error("a lone declared vitest should claim without a test script")
	}
}

func TestDependencyClaimRunsFromTheDeclaringManifest(t *testing.T) {
	root := t.TempDir()
	testutil.Put(t, root, "backend/pyproject.toml", "[dependency-groups]\ndev = [\"ruff\"]\n")
	testutil.Put(t, root, "backend/app/x.py", "")
	var ruff Adapter
	for _, a := range builtins {
		if a.Name == "ruff" {
			ruff = a
		}
	}
	claim, ok := ruff.Claims(filepath.Join(root, "backend/app/x.py"), root)
	if !ok || claim.Dir != filepath.Join(root, "backend") {
		t.Errorf("claim = %+v, %v", claim, ok)
	}
}

func TestBuiltinsTakeKitYMLData(t *testing.T) {
	biome, ruff := adapter(t, "biome"), adapter(t, "ruff")
	if !slices.Equal(biome.Signals, []string{"biome.json", "biome.jsonc"}) || !slices.Equal(biome.Sub, []string{"lint", "check", "ci"}) {
		t.Errorf("biome: signals %q sub %q, want the biome formatter's and lint pattern's", biome.Signals, biome.Sub)
	}
	if ruff.TOML != "pyproject.toml .tool.ruff" || !slices.Equal(ruff.Sub, []string{"check"}) {
		t.Errorf("ruff: toml %q sub %q", ruff.TOML, ruff.Sub)
	}
	if !slices.Equal(adapter(t, "go-vet").Sub, []string{"vet"}) {
		t.Error("go-vet keeps its own subcommand")
	}
}
