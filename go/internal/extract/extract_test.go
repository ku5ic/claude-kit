package extract

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Ports providers.bats' extractor tests: same fixtures, same expected output.

func fixture(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func expect(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestJSONKeysInFileOrder(t *testing.T) {
	f := fixture(t, "package.json", `{"scripts":{"test":"x","lint":"y","build":"z"}}`)
	expect(t, JSONKeys(f, ".scripts"), "test", "lint", "build")
}

func TestJSONKeysMissingPathOrFile(t *testing.T) {
	f := fixture(t, "package.json", `{}`)
	expect(t, JSONKeys(f, ".scripts"))
	expect(t, JSONKeys(filepath.Join(t.TempDir(), "nope.json"), ".scripts"))
}

func TestJSONArrayNested(t *testing.T) {
	f := fixture(t, "package.json", `{"workspaces":{"packages":["apps/*","packages/*"]}}`)
	expect(t, JSONArray(f, ".workspaces.packages"), "apps/*", "packages/*")
}

func TestJSONArrayIgnoresNonArray(t *testing.T) {
	f := fixture(t, "package.json", `{"workspaces":{"packages":["a"]}}`)
	expect(t, JSONArray(f, ".workspaces"))
}

func TestJSONValue(t *testing.T) {
	f := fixture(t, "package.json", `{"version":"1.2.3","n":3,"o":{}}`)
	if JSONValue(f, ".version") != "1.2.3" || JSONValue(f, ".n") != "3" || JSONValue(f, ".o") != "" {
		t.Errorf("got %q %q %q", JSONValue(f, ".version"), JSONValue(f, ".n"), JSONValue(f, ".o"))
	}
}

func TestTOMLKeysNestedTableInOrder(t *testing.T) {
	f := fixture(t, "pyproject.toml", "[tool.poe.tasks]\ntest = \"pytest\"\nlint = \"ruff check\"\n")
	expect(t, TOMLKeys(f, ".tool.poe.tasks"), "test", "lint")
}

func TestTOMLKeysOrderAcrossInlineAndDottedForms(t *testing.T) {
	f := fixture(t, "pyproject.toml", "[tool.poe.tasks.alpha]\ncmd = \"a\"\n[tool.poe.tasks]\nmid = {cmd = \"m\"}\nzeta.cmd = \"z\"\nplain = \"p\"\n")
	expect(t, TOMLKeys(f, ".tool.poe.tasks"), "alpha", "mid", "zeta", "plain")
	expect(t, TOMLKeys(f, ".tool.poe.tasks.plain"))
	expect(t, TOMLKeys(fixture(t, "bad.toml", "[a\n"), ".a"))
}

func TestTOMLArrayWorkspaceMembers(t *testing.T) {
	f := fixture(t, "Cargo.toml", "[workspace]\nmembers = [\"crates/a\", \"crates/b\"]\n")
	expect(t, TOMLArray(f, ".workspace.members"), "crates/a", "crates/b")
}

func TestTOMLHasEmptyTable(t *testing.T) {
	f := fixture(t, "pyproject.toml", "[tool.ruff]\n[tool.other]\nx = 1\n")
	if !TOMLHas(f, ".tool.ruff") || TOMLHas(f, ".tool.black") {
		t.Error("TOMLHas: want ruff true, black false")
	}
}

func TestTOMLPackageVersionIgnoresCase(t *testing.T) {
	f := fixture(t, "uv.lock", "[[package]]\nname = \"Django\"\nversion = \"6.0.7\"\n")
	if got := TOMLPackageVersion(f, "django"); got != "6.0.7" {
		t.Errorf("got %q", got)
	}
}

func TestYAMLArrayPnpmPackages(t *testing.T) {
	f := fixture(t, "pnpm-workspace.yaml", "packages:\n  - \"packages/*\"\n  - apps/web\n")
	expect(t, YAMLArray(f, ".packages"), "packages/*", "apps/web")
}

func TestMakeTargetsSkipsSpecialPatternAndAssignments(t *testing.T) {
	f := fixture(t, "Makefile", ".PHONY: test\nVAR := 1\ntest: build\n\tgo test\nbuild:\n\tgo build\n%.o: %.c\n\tcc\n")
	expect(t, MakeTargets(f), "test", "build")
}

func TestJustRecipesFallback(t *testing.T) {
	if _, err := exec.LookPath("just"); err == nil {
		t.Skip("just is installed; this covers the fallback")
	}
	f := fixture(t, "justfile", "set shell := [\"bash\", \"-c\"]\nversion := \"1\"\n\ntest *args:\n  cargo test {{args}}\n@lint:\n  cargo clippy\n")
	expect(t, JustRecipes(f), "test", "lint")
}

func TestRegexLinesLastCaptureGroup(t *testing.T) {
	f := fixture(t, "Rakefile", "task :build do\nend\n  task \"db:migrate\" do\nputs 1\n")
	expect(t, RegexLines(f, `^[[:space:]]*task[[:space:]]+:?"?([A-Za-z0-9_:]+)`), "build", "db:migrate")
}

func TestUnknownExtractorRefused(t *testing.T) {
	if _, err := Run("rm", "x", "y"); err == nil || !strings.Contains(err.Error(), "unknown extractor in kit.yml: rm") {
		t.Errorf("err = %v", err)
	}
}
