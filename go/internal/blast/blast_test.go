package blast

import (
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// repo makes a git repo holding files, each written with a trailing
// newline, and returns its physical path.
func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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
	dir := repo(t, map[string]string{"a.rb": "x"})
	code, _, stderr := run(testutil.KitConfig(t), filepath.Join(dir, "a.rb"))
	if code != 2 || !strings.Contains(stderr, "no import scanner for a.rb") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestRunResolvesARelativePathFromASubdirectoryAgainstTheRepo(t *testing.T) {
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
