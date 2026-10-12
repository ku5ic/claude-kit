package sources

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func fixture(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	testutil.Put(t, dir, name, body)
	return filepath.Join(dir, name)
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

func TestYAMLArrayPnpmPackages(t *testing.T) {
	f := fixture(t, "pnpm-workspace.yaml", "packages:\n  - \"packages/*\"\n  - apps/web\n")
	expect(t, YAMLArray(f, ".packages"), "packages/*", "apps/web")
}

func TestMakeTargetsSkipsSpecialPatternAndAssignments(t *testing.T) {
	f := fixture(t, "Makefile", ".PHONY: test\nVAR := 1\ntest: build\n\tgo test\nbuild:\n\tgo build\n%.o: %.c\n\tcc\n")
	expect(t, MakeTargets(f), "test", "build")
}

// Not parallel: each case sets PATH to its own just, or none.
func TestJustRecipes(t *testing.T) {
	justfile := "set shell := [\"bash\", \"-c\"]\nversion := \"1\"\n\ntest *args:\n  cargo test {{args}}\n@lint:\n  cargo clippy\n"
	withJust := func(t *testing.T, script string) {
		bin := t.TempDir()
		if script != "" {
			testutil.FakeTool(t, filepath.Join(bin, "just"), filepath.Join(t.TempDir(), "calls"), script)
		}
		t.Setenv("PATH", bin)
	}
	t.Run("with no just installed, recipe header lines", func(t *testing.T) {
		withJust(t, "")
		expect(t, JustRecipes(fixture(t, "justfile", justfile)), "test", "lint")
	})
	t.Run("just --summary when it succeeds", func(t *testing.T) {
		withJust(t, "echo build check")
		expect(t, JustRecipes(fixture(t, "justfile", justfile)), "build", "check")
	})
	t.Run("recipe header lines when just --summary fails", func(t *testing.T) {
		withJust(t, "exit 1")
		expect(t, JustRecipes(fixture(t, "justfile", justfile)), "test", "lint")
	})
}

func TestRegexLinesLastCaptureGroup(t *testing.T) {
	f := fixture(t, "Rakefile", "task :build do\nend\n  task \"db:migrate\" do\nputs 1\n")
	expect(t, RegexLines(f, `^[[:space:]]*task[[:space:]]+:?"?([A-Za-z0-9_:]+)`), "build", "db:migrate")
}
