package enforce

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stop plans Stop's checks of files, relative to the root, after a full
// plan has filled the verdict cache: Stop only reads it.
func (e *env) stop(files ...string) Plan {
	e.t.Helper()
	e.plan()
	var abs []string
	for _, f := range files {
		abs = append(abs, filepath.Join(e.root, f))
	}
	return Stop(e.cfg, Options{Root: e.root, CacheDir: e.cache}, abs)
}

// commands is each planned gate as "<label>: <command>", or "<label>:
// SKIP <why>".
func commands(p Plan) string {
	var lines []string
	for _, g := range p.Gates {
		if g.Skip != "" {
			lines = append(lines, g.Label+": SKIP "+g.Skip)
			continue
		}
		lines = append(lines, g.Label+": "+g.Command())
	}
	return strings.Join(lines, "\n")
}

func fixer() map[string]any { return map[string]any{"role": "fixer", "mutates": true} }

func TestStopRunsPreCommitCheckHooksByIdOnTheEditedFiles(t *testing.T) {
	e := setup(t)
	e.write(".pre-commit-config.yaml", "repos:\n  - repo: local\n    hooks:\n      - id: ruff\n        name: ruff\n        entry: ruff check\n        language: system\n      - id: ruff-format\n        name: ruff-format\n        entry: ruff format\n        language: system\n")
	e.tool(filepath.Join(e.path, "pre-commit"))
	e.write("src/a.py", "x = 1\n")
	e.write("b.py", "y = 2\n")
	e.check("ruff", check("lint"))
	e.check("ruff-format", fixer())
	got := commands(e.stop("src/a.py", "b.py"))
	if got != "lint (.pre-commit-config.yaml: ruff): pre-commit run ruff --hook-stage pre-commit --files src/a.py b.py" {
		t.Errorf("plan:\n%s", got)
	}
}

func TestStopRunsLintStagedChecksOnTheFilesTheirGlobsMatch(t *testing.T) {
	e := setup(t)
	e.write("package.json", `{"lint-staged":{"*.{ts,tsx}":"eslint --max-warnings=0","*.css":"prettier --write","*.md":"markdownlint"}}`)
	e.tool("node_modules/.bin/eslint")
	e.tool("node_modules/.bin/prettier")
	e.write("src/deep/a.ts", "")
	e.write("a.css", "")
	e.write("notes.md", "")
	e.check("*.{ts,tsx}", check("lint"))
	e.check("*.css", fixer())
	e.check("*.md", check("lint"))
	got := commands(e.stop("src/deep/a.ts", "a.css"))
	if got != "lint (package.json: *.{ts,tsx}): eslint --max-warnings=0 src/deep/a.ts" {
		t.Errorf("plan:\n%s", got)
	}
}

func TestStopRunsLefthookCommandsWithTheirFilesInPlace(t *testing.T) {
	e := setup(t)
	e.write("lefthook.yml", "pre-commit:\n  commands:\n    vet:\n      root: svc/\n      glob: \"*.go\"\n      run: go vet {staged_files}\n")
	e.tool(filepath.Join(e.path, "go"))
	e.write("svc/go.mod", "module svc\n")
	e.write("svc/pkg/a.go", "package pkg\n")
	e.write("svc/README", "")
	e.write("tools/b.go", "package tools\n")
	e.check("vet", check("lint"))
	// A file outside root: isn't the command's, whatever its glob matches.
	if got := commands(e.stop("svc/pkg/a.go", "svc/README", "tools/b.go")); got != "lint (lefthook.yml: vet): go vet pkg/a.go" {
		t.Errorf("plan:\n%s", got)
	}
}

// A kind a hook config checks a file for takes no file form of a CI check;
// a file it doesn't check still gets one.
func TestStopFallsBackToCIFileFormsPerFileAndKind(t *testing.T) {
	e := setup(t)
	e.write("package.json", `{"lint-staged":{"*.ts":"eslint"}}`)
	e.tool("node_modules/.bin/eslint")
	e.tool("bin/ruff")
	e.write(".github/workflows/ci.yml", "jobs:\n  lint:\n    steps:\n      - run: eslint .\n      - run: ruff check .\n")
	e.write("a.ts", "")
	e.write("a.py", "")
	e.check("*.ts", check("lint"))
	e.check("lint/1", map[string]any{"role": "check", "kind": "lint", "file_form": "eslint {files}", "globs": []string{"*.ts"}})
	e.check("lint/2", map[string]any{"role": "check", "kind": "lint", "file_form": "ruff check {files}", "globs": []string{"*.py"}})
	got := commands(e.stop("a.ts", "a.py"))
	want := "lint (package.json: *.ts): eslint a.ts\nlint (.github/workflows/ci.yml: lint/2): ruff check a.py"
	if got != want {
		t.Errorf("plan:\n%s\nwant:\n%s", got, want)
	}
}

// Ignored, deleted, and outside files never reach a check.
func TestStopChecksOnlyFilesTheRepoKeeps(t *testing.T) {
	e := setup(t)
	e.write("package.json", `{"lint-staged":{"*.ts":"eslint"}}`)
	e.tool("node_modules/.bin/eslint")
	e.write(".gitignore", "node_modules\n/bin\ngen/\n")
	e.write("gen/x.ts", "")
	e.write("kept.ts", "")
	e.check("*.ts", check("lint"))
	outside := filepath.Join(filepath.Dir(e.root), "elsewhere.ts")
	if err := os.WriteFile(outside, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	p := e.stop("gen/x.ts", "gone.ts", "kept.ts")
	p2 := Stop(e.cfg, Options{Root: e.root, CacheDir: e.cache}, []string{outside, filepath.Join(e.root, "kept.ts")})
	if got := commands(p); got != "lint (package.json: *.ts): eslint kept.ts" {
		t.Errorf("plan:\n%s", got)
	}
	if got := commands(p2); got != "lint (package.json: *.ts): eslint kept.ts" {
		t.Errorf("with an outside file first:\n%s", got)
	}
}

func TestStopRecordsAnUnclassifiedHookEntry(t *testing.T) {
	e := setup(t)
	e.write("package.json", `{"lint-staged":{"*.ts":"eslint"}}`)
	e.write("a.ts", "")
	p := e.stop("a.ts")
	if got := commands(p); !strings.Contains(got, "package.json: *.ts: SKIP unclassified") || p.Unclassified != 1 {
		t.Errorf("unclassified %d, plan:\n%s", p.Unclassified, got)
	}
}

func TestCommandPutsEachFileInItsWord(t *testing.T) {
	root := "/r"
	g := Gate{Dir: root, Files: []string{root + "/src/a b.ts", root + "/src/x&y.ts", root + "/top.ts"}}
	tests := []struct{ body, want string }{
		{"lint {files}", "lint 'src/a b.ts' 'src/x&y.ts' top.ts"},
		{"lint --file={files} --fix=false", "lint --file='src/a b.ts' --file='src/x&y.ts' --file=top.ts --fix=false"},
		{"go vet {dirs}", "go vet . ./src"},
	}
	for _, tt := range tests {
		g.Body = tt.body
		if got := g.Command(); got != tt.want {
			t.Errorf("%q: %s, want %s", tt.body, got, tt.want)
		}
	}
}
