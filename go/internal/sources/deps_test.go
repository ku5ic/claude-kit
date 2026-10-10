package sources

import (
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestPythonDepsReadsEveryDeclarationForm(t *testing.T) {
	dir := t.TempDir()
	testutil.Put(t, dir, "pyproject.toml", `
[project]
dependencies = ["Django>=5", "psycopg[binary]==3.2"]
[project.optional-dependencies]
docs = ["mkdocs"]
[dependency-groups]
dev = ["ruff==0.15", {include-group = "test"}]
test = ["pytest"]
[tool.poetry.dependencies]
python = "^3.12"
Black = "*"
[tool.poetry.group.lint.dependencies]
my_py = "*"
`)
	testutil.Put(t, dir, "requirements-dev.txt", "# comment\n-r requirements.txt\npyright==1.1\n")
	deps := PythonDeps(dir)
	for _, want := range []string{"django", "psycopg", "mkdocs", "ruff", "pytest", "black", "my-py", "pyright"} {
		if !deps[want] {
			t.Errorf("missing %s in %v", want, deps)
		}
	}
	if deps["python"] {
		t.Error("poetry's python constraint is not a dependency")
	}
}

func TestRubyDepsFromGemfileLock(t *testing.T) {
	dir := t.TempDir()
	testutil.Put(t, dir, "Gemfile.lock", "GEM\n  remote: https://rubygems.org/\n  specs:\n    rubocop (1.66.0)\n      json (~> 2.3)\n")
	deps := RubyDeps(dir)
	if !deps["rubocop"] || deps["json"] {
		t.Errorf("deps = %v: want rubocop, not the nested json requirement", deps)
	}
}
