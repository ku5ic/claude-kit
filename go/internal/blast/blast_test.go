package blast

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/resolve/kittool"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// repo makes a git repo holding files, each written with a trailing
// newline, and returns its physical path.
func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := testutil.Physical(t, t.TempDir())
	testutil.Git(t, dir, "init", "-q", "-b", "main")
	for name, body := range files {
		testutil.Put(t, dir, name, body+"\n")
	}
	return dir
}

// tsFiles is a TS repo where src/lib/format.ts has a relative, an alias, and
// a test consumer, and src/other.ts imports a different module.
var tsFiles = map[string]string{
	"src/lib/format.ts":       `export const formatDate = () => "";`,
	"src/app/page.ts":         `import { formatDate } from '../lib/format';`,
	"src/components/Card.tsx": `import { formatDate } from "@/lib/format";`,
	"src/lib/format.test.ts":  `import { formatDate } from './format.js';`,
	"src/other.ts":            `import { formatMoney } from './lib/money';`,
}

// withTS is tsFiles with overrides applied.
func withTS(overrides map[string]string) map[string]string {
	files := maps.Clone(tsFiles)
	maps.Copy(files, overrides)
	return files
}

func run(cfg *config.Config, args ...string) (code int, stdout, stderr string) {
	var out, errOut strings.Builder
	code = Run(cfg, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRun(t *testing.T) {
	needsASTGrep(t)
	cfg := testutil.KitConfig(t)
	tests := []struct {
		name   string
		files  map[string]string
		target string // relative to the repo
		symbol string
		want   string // the whole stdout, when set
		has    []string
		lacks  []string
	}{
		{
			name:   "TS: relative, alias, and test consumers",
			files:  tsFiles,
			target: "src/lib/format.ts",
			want: `blast-radius: src/lib/format.ts
consumers: 3 (source 2, test 1)
src/app/page.ts:1 source
src/components/Card.tsx:1 source alias-match
src/lib/format.test.ts:1 test
`,
		},
		{
			name:   "a symbol keeps only consumers that use it",
			files:  withTS(map[string]string{"src/app/page.ts": `import { other } from '../lib/format';`}),
			target: "src/lib/format.ts",
			symbol: "formatDate",
			has:    []string{"consumers: 2 (source 1, test 1)"},
			lacks:  []string{"src/app/page.ts"},
		},
		{
			name: "an index file is reached through its directory",
			files: map[string]string{
				"src/ui/index.ts": `export {};`,
				"src/main.ts":     `import { Button } from './ui';`,
			},
			target: "src/ui/index.ts",
			has:    []string{"src/main.ts:1 source"},
		},
		{
			name: "an index file is reached by a bare '..' or './' from below or beside it",
			files: map[string]string{
				"src/foo/index.ts": `export const a = 1;`,
				"src/foo/sub/b.ts": `import { a } from '..';`,
				"src/foo/c.ts":     `import { a } from "./";`,
			},
			target: "src/foo/index.ts",
			has:    []string{"consumers: 2 (source 2, test 0)"},
		},
		{
			name:   "a non-literal import() is flagged",
			files:  withTS(map[string]string{"src/lazy.ts": `const m = await import(path);`}),
			target: "src/lib/format.ts",
			has:    []string{"unresolvable imports present"},
		},
		{
			name: "workspace: a package-name import is a workspace-match",
			files: map[string]string{
				"package.json":             `{"name":"root","private":true}`,
				"pnpm-workspace.yaml":      "packages:\n  - apps/*\n  - packages/*",
				"packages/ui/package.json": `{"name":"@acme/ui"}`,
				"packages/ui/src/index.ts": `export {};`,
				"apps/web/package.json":    `{"name":"web"}`,
				"apps/web/src/page.tsx":    `import { Button } from '@acme/ui';`,
				"apps/web/src/other.tsx":   `import { x } from '@acme/uikit';`,
			},
			target: "packages/ui/src/index.ts",
			has:    []string{"consumers: 1 (source 1, test 0)", "apps/web/src/page.tsx:1 source workspace-match"},
		},
		{
			name: "Python: absolute, from-parent, relative, and test imports",
			files: map[string]string{
				"app/__init__.py":      ``,
				"app/models.py":        `class User: pass`,
				"app/views.py":         `from .models import User`,
				"app/admin.py":         `from app import models`,
				"scripts/seed.py":      `import app.models as m`,
				"tests/test_models.py": `from app.models import User`,
				"app/unrelated.py":     `from app import views`,
			},
			target: "app/models.py",
			want: `blast-radius: app/models.py
consumers: 4 (source 3, test 1)
app/admin.py:1 source
app/views.py:1 source
scripts/seed.py:1 source
tests/test_models.py:1 test
`,
		},
		{
			name: "Python: a submodule import uses its package",
			files: map[string]string{
				"app/__init__.py":        ``,
				"app/models/__init__.py": `class M: pass`,
				"app/models/user.py":     `class U: pass`,
				"b.py":                   `import app.models.user`,
			},
			target: "app/models/__init__.py",
			has:    []string{"consumers: 1 (source 1, test 0)", "b.py:1 source"},
		},
		{
			name: "another language's import of the same path isn't a consumer",
			files: map[string]string{
				"lib/f.go": `package lib`,
				"src/e.ts": `import lib from "lib";`,
			},
			target: "lib/f.go",
			has:    []string{"consumers: 0 "},
		},
		{
			name: "an asset with the module's name isn't a consumer of it",
			files: map[string]string{
				"src/a.ts":             `export const a = 1;`,
				"src/a.css":            `.a {}`,
				"src/d.ts":             `import "./a.css";`,
				"src/v2.ts":            `import { b } from "@/lib/format.v2";`,
				"src/lib/format.ts":    `export const f = 1;`,
				"src/lib/format.v2.ts": `export const b = 1;`,
			},
			target: "src/a.ts",
			has:    []string{"consumers: 0 "},
		},
		{
			name: "an alias keeps a dotted basename whole",
			files: map[string]string{
				"src/lib/format.ts":    `export const f = 1;`,
				"src/lib/format.v2.ts": `export const b = 1;`,
				"src/v2.ts":            `import { b } from "@/lib/format.v2";`,
			},
			target: "src/lib/format.ts",
			has:    []string{"consumers: 0 "},
		},
		{
			name: "consumers in hidden directories, Vue, and Svelte files count",
			files: map[string]string{
				"src/a.ts":              `export const a = 1;`,
				".storybook/preview.ts": `import { a } from "../src/a";`,
				"src/C.vue":             "<script setup>\nimport { a } from \"./a\"\n</script>",
				"src/S.svelte":          "<script>\nimport { a } from './a';\n</script>",
			},
			target: "src/a.ts",
			want: `blast-radius: src/a.ts
consumers: 3 (source 3, test 0)
.storybook/preview.ts:1 source
src/C.vue:2 source
src/S.svelte:2 source
`,
		},
		{
			name: "a file importing the target twice is listed at its first import",
			files: map[string]string{
				"src/a.ts": `export const a = 1;`,
				"src/b.ts": "import { a } from \"./a.js\";\n\nimport { b } from \"./a\";",
			},
			target: "src/a.ts",
			has:    []string{"src/b.ts:1 source"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := []string{filepath.Join(repo(t, tt.files), tt.target)}
			if tt.symbol != "" {
				args = append(args, tt.symbol)
			}
			code, stdout, stderr := run(cfg, args...)
			if code != 0 {
				t.Fatalf("exit %d, stderr %q", code, stderr)
			}
			if tt.want != "" && stdout != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", stdout, tt.want)
			}
			for _, s := range tt.has {
				if !strings.Contains(stdout, s) {
					t.Errorf("stdout lacks %q:\n%s", s, stdout)
				}
			}
			for _, s := range tt.lacks {
				if strings.Contains(stdout, s) {
					t.Errorf("stdout has %q:\n%s", s, stdout)
				}
			}
		})
	}
}

func TestRunUnsupportedFileTypeExits2(t *testing.T) {
	t.Parallel()
	dir := repo(t, map[string]string{"a.txt": "x"})
	code, _, stderr := run(testutil.KitConfig(t), filepath.Join(dir, "a.txt"))
	if code != 2 || !strings.Contains(stderr, "no import scanner for a.txt") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

// needsASTGrep skips t without ast-grep, except in CI, which installs it.
func needsASTGrep(t *testing.T) {
	t.Helper()
	if _, err := kittool.Path("ast-grep"); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal(err)
		}
		t.Skip(err) // kit: allow ast-grep is optional on a dev machine
	}
}

// goFiles is a Go module where lib holds two files and an in-package test,
// two files import lib, one of them a test, and other.go imports nothing
// of the module.
var goFiles = map[string]string{
	"go.mod":               "module example.com/m\n\ngo 1.27",
	"lib/format.go":        "package lib\n\nfunc Format() string { return \"\" }",
	"lib/parse.go":         "package lib\n\nfunc Parse() {}",
	"lib/format_test.go":   "// Package lib's tests.\npackage lib\n\nvar _ = Format()",
	"cmd/app/main.go":      "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/m/lib\"\n)\n\nfunc main() { fmt.Println(lib.Format()) }",
	"cmd/app/main_test.go": "package main\n\nimport \"example.com/m/lib\"\n\nvar _ = lib.Parse",
	"other.go":             "package m\n\nimport \"strings\"\n\nvar _ = strings.ToUpper",
}

func TestRunOtherLanguagesThroughCodeGraph(t *testing.T) {
	needsASTGrep(t)
	cfg := testutil.KitConfig(t)
	tests := []struct {
		name, target, symbol, want string
		files                      map[string]string
	}{
		{
			name:   "Go: the importers of the file's package, and its own files",
			files:  goFiles,
			target: "lib/format.go",
			want: `blast-radius: lib/format.go
consumers: 4 (source 2, test 2)
cmd/app/main.go:6 source
cmd/app/main_test.go:3 test
lib/format_test.go:2 test same-package
lib/parse.go:1 source same-package
`,
		},
		{
			name:   "Go: a symbol keeps the importers that use it",
			files:  goFiles,
			target: "lib/format.go",
			symbol: "Format",
			want: `blast-radius: lib/format.go (symbol Format)
consumers: 2 (source 1, test 1)
cmd/app/main.go:6 source
lib/format_test.go:2 test same-package
`,
		},
		{
			name: "Java: a dotted import under a source root",
			files: map[string]string{
				"src/main/java/com/acme/util/Strings.java":     "package com.acme.util;\n\npublic class Strings {}",
				"src/main/java/com/acme/App.java":              "package com.acme;\n\nimport com.acme.util.Strings;\n\nclass App {}",
				"src/main/java/com/acme/Other.java":            "package com.acme;\n\nimport java.util.List;\n\nclass Other {}",
				"src/test/java/com/acme/util/StringsTest.java": "package com.acme.util;\n\nimport com.acme.util.Strings;\n\nclass StringsTest {}",
			},
			target: "src/main/java/com/acme/util/Strings.java",
			want: `blast-radius: src/main/java/com/acme/util/Strings.java
consumers: 2 (source 1, test 1)
src/main/java/com/acme/App.java:3 source
src/test/java/com/acme/util/StringsTest.java:3 test
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := []string{filepath.Join(repo(t, tt.files), tt.target)}
			if tt.symbol != "" {
				args = append(args, tt.symbol)
			}
			code, stdout, stderr := run(cfg, args...)
			if code != 0 {
				t.Fatalf("exit %d, stderr %q", code, stderr)
			}
			if stdout != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", stdout, tt.want)
			}
		})
	}
}

func TestRunNamesAScanFailureAsItself(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip(err) // kit: allow the repo helper needs git
	}
	bin := t.TempDir()
	if err := os.Symlink(gitBin, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	testutil.Put(t, bin, "ast-grep", "#!/bin/sh\necho 'rule rejected' >&2\nexit 3\n")
	if err := os.Chmod(filepath.Join(bin, "ast-grep"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := repo(t, goFiles)
	t.Setenv("PATH", bin)
	code, _, stderr := run(testutil.KitConfig(t), filepath.Join(dir, "lib/format.go"))
	if code != 2 || strings.Contains(stderr, "needs ast-grep") || !strings.Contains(stderr, "rule rejected") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestRunWithoutASTGrepExits2(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip(err) // kit: allow the repo helper needs git
	}
	bin := t.TempDir()
	if err := os.Symlink(gitBin, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	dir := repo(t, goFiles)
	t.Setenv("PATH", bin)
	code, _, stderr := run(testutil.KitConfig(t), filepath.Join(dir, "lib/format.go"))
	if code != 2 || !strings.Contains(stderr, "blast-radius: format.go needs ast-grep") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestRunResolvesARelativePathFromASubdirectoryAgainstTheRepo(t *testing.T) {
	needsASTGrep(t)
	cfg := testutil.KitConfig(t)
	dir := repo(t, tsFiles)
	t.Chdir(filepath.Join(dir, "src"))
	_, stdout, _ := run(cfg, "lib/format.ts")
	for _, s := range []string{"blast-radius: src/lib/format.ts\n", "consumers: 3 (source 2, test 1)\n"} {
		if !strings.Contains(stdout, s) {
			t.Errorf("stdout lacks %q:\n%s", s, stdout)
		}
	}
}
