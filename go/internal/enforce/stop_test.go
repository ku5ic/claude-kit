package enforce

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestStopMatchesAMidPatternGlobStarAtAnyDepth(t *testing.T) {
	e := setup(t)
	e.write("package.json", `{"lint-staged":{"src/**/*.ts":"eslint"}}`)
	e.tool("node_modules/.bin/eslint")
	e.write("src/a.ts", "")
	e.write("src/b/c.ts", "")
	e.write("lib/a.ts", "")
	e.check("src/**/*.ts", check("lint"))
	got := commands(e.stop("src/a.ts", "src/b/c.ts", "lib/a.ts"))
	if got != "lint (package.json: src/**/*.ts): eslint src/a.ts src/b/c.ts" {
		t.Errorf("plan:\n%s", got)
	}
}

func TestGlobMatch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		glob, rel string
		want      bool
	}{
		{"a/**/b/**/*.go", "a/b/x.go", true}, // a mid-pattern **/ matches no directory too
		{"**/*.ts", "a.ts", true},
	} {
		if got := globMatch(tc.glob, tc.rel); got != tc.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", tc.glob, tc.rel, got, tc.want)
		}
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

// Of several file forms of one kind, a file gets the one with the fewest
// arguments: a marker subset and a coverage run lose to the plain run.
func TestStopRunsTheLeastFilteredFileFormOncePerFileAndKind(t *testing.T) {
	e := setup(t)
	e.tool("bin/pytest")
	e.write(".github/workflows/ci.yml", "jobs:\n  test:\n    steps:\n      - run: pytest -m unit\n      - run: pytest\n      - run: pytest --cov --cov-fail-under=80\n")
	e.write("test_a.py", "")
	e.check("test/1", map[string]any{"role": "check", "kind": "test", "file_form": "pytest -m unit {files}", "globs": []string{"test_*.py"}})
	e.check("test/2", map[string]any{"role": "check", "kind": "test", "file_form": "pytest {files}", "globs": []string{"test_*.py"}})
	e.check("test/3", map[string]any{"role": "check", "kind": "test", "file_form": "pytest --cov --cov-fail-under=80 {files}", "globs": []string{"test_*.py"}})
	if got, want := commands(e.stop("test_a.py")), "test (.github/workflows/ci.yml: test/2): pytest test_a.py"; got != want {
		t.Errorf("plan:\n%s\nwant:\n%s", got, want)
	}
}

// Two file forms of one kind both run when neither covers the other.
func TestStopRunsBothFileFormsWhenNeitherCoversTheOther(t *testing.T) {
	for _, tc := range []struct {
		name, kind, glob, ci, file string
		tools                      []string
		forms, want                [2]string
	}{
		// A flag other than a selector checks more than the plain run.
		{"a stricter form beside the plain one", "lint", "*.js",
			"jobs:\n  lint:\n    steps:\n      - run: eslint .\n      - run: eslint --max-warnings 0 .\n", "a.js",
			[]string{"bin/eslint"}, [2]string{"eslint {files}", "eslint --max-warnings 0 {files}"}, [2]string{": eslint a.js", "eslint --max-warnings 0 a.js"}},
		{"two forms with different selectors", "test", "*.py",
			"jobs:\n  test:\n    steps:\n      - run: pytest -m unit\n      - run: pytest -k integration\n", "test_a.py",
			[]string{"bin/pytest"}, [2]string{"pytest -m unit {files}", "pytest -k integration {files}"}, [2]string{"pytest -m unit test_a.py", "pytest -k integration test_a.py"}},
		// Only another form of the same tool loses its files.
		{"two tools of one kind", "lint", "*.py",
			"jobs:\n  lint:\n    steps:\n      - run: pylint .\n      - run: ruff check .\n", "a.py",
			[]string{"bin/ruff", "bin/pylint"}, [2]string{"pylint {files}", "ruff check {files}"}, [2]string{"pylint a.py", "ruff check a.py"}},
		// Two package scripts are two tools, though one runner runs both.
		{"two scripts of one kind", "lint", "*.ts",
			"jobs:\n  lint:\n    steps:\n      - run: npm run lint\n      - run: npm run lint:css\n", "a.ts",
			[]string{"bin/npm"}, [2]string{"npm run lint -- {files}", "npm run lint:css -- {files}"}, [2]string{"npm run lint -- a.ts", "npm run lint:css -- a.ts"}},
		// A script name is its dir's own: web's lint is another script.
		{"the same script name in each dir", "lint", "**/*.ts",
			"jobs:\n  lint:\n    steps:\n      - run: npm run lint\n      - run: npm run lint\n        working-directory: web\n", "web/a.ts",
			[]string{"bin/npm"}, [2]string{"npm run lint -- {files}", "npm run lint -- {files}"}, [2]string{"npm run lint -- web/a.ts", "npm run lint -- a.ts"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := setup(t)
			for _, tool := range tc.tools {
				e.tool(tool)
			}
			e.write(".github/workflows/ci.yml", tc.ci)
			e.write(tc.file, "")
			for i, form := range tc.forms {
				e.check(fmt.Sprintf("%s/%d", tc.kind, i+1), map[string]any{"role": "check", "kind": tc.kind, "file_form": form, "globs": []string{tc.glob}})
			}
			got := commands(e.stop(tc.file))
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("plan lacks %q:\n%s", want, got)
				}
			}
		})
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

// A check changed mid-session is an entry the cache lacks: a Stop that may
// ask gets its verdict and runs it, and asks nothing when the cache has
// every answer.
func TestStopAsksForAnEntryTheCacheLacks(t *testing.T) {
	e := setup(t)
	e.tool("node_modules/.bin/eslint")
	e.write("package.json", `{"lint-staged":{"*.ts":"eslint"}}`)
	e.write("a.ts", "")
	e.check("*.ts", check("lint"))
	e.plan()
	e.write("package.json", `{"lint-staged":{"*.ts":"eslint --max-warnings 0"}}`)
	e.answer()
	files := []string{filepath.Join(e.root, "a.ts")}
	if p := Stop(e.cfg, Options{Root: e.root, CacheDir: e.cache}, files); p.Unclassified != 1 {
		t.Errorf("from the cache alone: unclassified %d, plan:\n%s", p.Unclassified, commands(p))
	}
	ask := Options{Root: e.root, CacheDir: e.cache, Ask: true, Timeout: 10 * time.Second}
	p := Stop(e.cfg, ask, files)
	if got := commands(p); got != "lint (package.json: *.ts): eslint --max-warnings 0 a.ts" || p.Classifier.Runs == 0 {
		t.Errorf("asked %d times, plan:\n%s", p.Classifier.Runs, got)
	}
	if p := Stop(e.cfg, ask, files); p.Classifier.Runs != 0 {
		t.Errorf("a full cache asked %d times", p.Classifier.Runs)
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
		// Text after the placeholder closes the word around the whole list.
		{`test -z "$(gofmt -l {files})"`, `test -z "$(gofmt -l 'src/a b.ts' 'src/x&y.ts' top.ts)"`},
		{`test -z "$(gofmt -l "{files}")"`, `test -z "$(gofmt -l 'src/a b.ts' 'src/x&y.ts' top.ts)"`},
		// A closed subshell or a backtick substitution earlier doesn't hide one.
		{`(cd sub && make lint); test -z "$(gofmt -l {files})"`, `(cd sub && make lint); test -z "$(gofmt -l 'src/a b.ts' 'src/x&y.ts' top.ts)"`},
		{"test -z \"`gofmt -l {files}`\"", "test -z \"`gofmt -l 'src/a b.ts' 'src/x&y.ts' top.ts`\""},
		{`echo "a)"; lint {files}`, `echo "a)"; lint 'src/a b.ts' 'src/x&y.ts' top.ts`},
		// Quotes around the bare placeholder are the script's, not a word's.
		{`prettier --check "{files}"`, "prettier --check 'src/a b.ts' 'src/x&y.ts' top.ts"},
		{`prettier --check '{files}'`, "prettier --check 'src/a b.ts' 'src/x&y.ts' top.ts"},
	}
	for _, tt := range tests {
		g.Body = tt.body
		if got := g.Command(); got != tt.want {
			t.Errorf("%q: %s, want %s", tt.body, got, tt.want)
		}
	}
	// A file named like a flag stays a file.
	g = Gate{Dir: root, Body: "prettier --check {files}", Files: []string{root + "/--write.ts"}}
	if got, want := g.Command(), "prettier --check ./--write.ts"; got != want {
		t.Errorf("a dash-led file: %s, want %s", got, want)
	}
}

func TestFormTool(t *testing.T) {
	t.Parallel()
	for form, want := range map[string]string{
		"pytest -m unit {files}":            "pytest",
		"uv run ruff check {files}":         "ruff",
		"pnpm exec eslint {files}":          "eslint",
		"CI=1 env ./node_modules/.bin/x":    "x",
		"go tool golangci-lint run {files}": "golangci-lint",
		"npm run lint:css -- {files}":       "npm run lint:css",
		"npm test -- {files}":               "npm test",
		"npm run -s lint -- {files}":        "npm run lint",
		"make lint FILES={files}":           "make lint",
		"make -C web lint FILES={files}":    "make lint",
		"make -j 4 test FILES={files}":      "make test",
	} {
		if got := formTool(form); got != want {
			t.Errorf("formTool(%q) = %q, want %q", form, got, want)
		}
	}
}
