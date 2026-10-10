package enforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

// edit plans the fixers of file, relative to the root, after a full plan
// has filled the verdict cache.
func (e *env) edit(file string) Plan {
	e.t.Helper()
	e.plan()
	return Edit(e.cfg, Options{Root: e.root, CacheDir: e.cache}, filepath.Join(e.root, file))
}

func fixerProposal(command, evidence string, globs ...string) map[string]any {
	return map[string]any{"role": "fixer", "command": command, "file_form": command + " {files}", "globs": globs, "dir": ".", "evidence": evidence, "mutates": true}
}

func TestEditRunsEveryHookFixerThatClaimsTheFile(t *testing.T) {
	e := setup(t)
	e.write(".pre-commit-config.yaml", "repos:\n  - repo: local\n    hooks:\n      - id: ruff\n        name: ruff\n        entry: ruff check\n        language: system\n      - id: ruff-format\n        name: ruff-format\n        entry: ruff format\n        language: system\n")
	e.write("package.json", `{"lint-staged":{"*.py":"black"}}`)
	e.tool(filepath.Join(e.path, "pre-commit"))
	e.tool("bin/black")
	e.write("a b.py", "")
	e.check("ruff", check("lint"))
	e.check("ruff-format", fixer())
	e.check("*.py", fixer())
	p := e.edit("a b.py")
	want := "fix (.pre-commit-config.yaml: ruff-format): pre-commit run ruff-format --hook-stage pre-commit --files 'a b.py'\nfix (package.json: *.py): black 'a b.py'"
	if got := commands(p); got != want || !p.Claims() {
		t.Errorf("claims %v, plan:\n%s\nwant:\n%s", p.Claims(), got, want)
	}
}

func TestALefthookFixerClaimsOnlyFilesUnderItsRoot(t *testing.T) {
	e := setup(t)
	e.write("lefthook.yml", "pre-commit:\n  commands:\n    fmt:\n      root: svc/\n      glob: \"*.go\"\n      run: gofmt -w {staged_files}\n")
	e.tool(filepath.Join(e.path, "gofmt"))
	e.write("svc/go.mod", "module svc\n")
	e.write("svc/a.go", "package svc\n")
	e.write("tools/b.go", "package tools\n")
	e.check("fmt", fixer())
	if got := commands(e.edit("svc/a.go")); got != "fix (lefthook.yml: fmt): gofmt -w a.go" {
		t.Errorf("plan:\n%s", got)
	}
	if p := e.edit("tools/b.go"); p.Claims() {
		t.Errorf("claims a file outside its root:\n%s", commands(p))
	}
}

func TestEditFallsBackToTheEvidenceFormatter(t *testing.T) {
	e := setup(t)
	e.write(".prettierrc", "{}\n")
	e.write("pyproject.toml", "[tool.ruff]\n")
	e.tool("node_modules/.bin/prettier")
	e.tool("bin/ruff")
	e.write("src/a.ts", "")
	e.write("b.py", "")
	e.write("c.txt", "")
	e.proposals = []map[string]any{fixerProposal("prettier --write", ".prettierrc", "*.ts"), fixerProposal("ruff format", "pyproject.toml", "**/*.py")}
	if got := commands(e.edit("src/a.ts")); got != "fix (evidence .prettierrc): prettier --write src/a.ts" {
		t.Errorf("ts:\n%s", got)
	}
	if got := commands(e.edit("b.py")); got != "fix (evidence pyproject.toml): ruff format b.py" {
		t.Errorf("py:\n%s", got)
	}
	if p := e.edit("c.txt"); p.Claims() || len(p.Gates) != 0 {
		t.Errorf("an unclaimed file: %s", commands(p))
	}
}

func TestEditLeavesAFileTwoFormattersClaimAlone(t *testing.T) {
	e := setup(t)
	e.write("biome.json", "{}\n")
	e.write(".prettierrc", "{}\n")
	e.write("a.ts", "")
	e.proposals = []map[string]any{fixerProposal("biome format --write", "biome.json", "*.ts"), fixerProposal("prettier --write", ".prettierrc", "*.ts")}
	p := e.edit("a.ts")
	if got := commands(p); got != "fix: SKIP left alone: evidence biome.json and evidence .prettierrc each format it" || !p.Claims() {
		t.Errorf("claims %v, plan:\n%s", p.Claims(), got)
	}
}

// A fixer the project states but can't run still claims the file, so no
// fallback swaps another in.
func TestAFixerThatCantRunClaimsTheFile(t *testing.T) {
	e := setup(t)
	e.write(".prettierrc", "{}\n")
	e.write("package.json", `{"devDependencies":{"prettier":"3"}}`)
	e.write("a.md", "")
	e.proposals = []map[string]any{fixerProposal("prettier --write", ".prettierrc", "*.md")}
	p := e.edit("a.md")
	if got := commands(p); !strings.Contains(got, "fix (evidence .prettierrc): SKIP prettier declared in package.json but not installed") || !p.Claims() {
		t.Errorf("claims %v, plan:\n%s", p.Claims(), got)
	}
}

func TestAnUnclassifiedHookFixerClaimsNothing(t *testing.T) {
	e := setup(t)
	e.write("package.json", `{"lint-staged":{"*.md":"prettier --write"}}`)
	e.write("a.md", "")
	p := e.edit("a.md")
	if got := commands(p); got != "package.json: *.md: SKIP unclassified" || p.Claims() {
		t.Errorf("claims %v, plan:\n%s", p.Claims(), got)
	}
}

func TestOnEditTakesTheLastMatchingChainAndEveryNote(t *testing.T) {
	cfg := &config.Config{OnEdit: []config.EditRule{
		{Globs: []string{"*.md"}, Run: []config.EditCommand{{Cmd: "a {file}"}}},
		{Globs: []string{"*.sh"}, Note: "shellcheck {file}"},
		{Globs: []string{"docs/*.md"}, Run: []config.EditCommand{{Cmd: "b {file}"}}, Note: "vale {file}"},
	}}
	chain, notes := OnEdit(cfg, "/r", "/r/docs/x.md")
	if len(chain) != 1 || chain[0].Cmd != "b {file}" || strings.Join(notes, ",") != "vale {file}" {
		t.Errorf("chain %+v notes %v", chain, notes)
	}
	if chain, notes := OnEdit(cfg, "/r", "/r/run.sh"); chain != nil || strings.Join(notes, ",") != "shellcheck {file}" {
		t.Errorf("sh: chain %+v notes %v", chain, notes)
	}
}
