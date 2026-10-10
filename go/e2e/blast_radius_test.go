package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// `kit blast-radius`: consumers of a file across relative, alias,
// workspace, and Python imports, split into source and test.
func TestBlastRadius(t *testing.T) {
	t.Parallel()
	output := func(r Result) string { return strings.TrimRight(r.Output, "\n") }
	// repo makes a git repo named name, the cwd of every run, and returns a
	// writer for files in it (content plus a newline, like printf '%s\n').
	repo := func(t *testing.T, name string) (*Kit, func(path, content string)) {
		k := New(t)
		dir := filepath.Join(Physical(t, t.TempDir()), name)
		Mkdir(t, dir)
		k.Git(dir, "init", "-q", "-b", "main")
		k.Dir = dir
		return k, func(path, content string) { Write(t, filepath.Join(dir, path), content+"\n") }
	}
	tsFixture := func(t *testing.T) (*Kit, func(path, content string)) {
		k, write := repo(t, "ts")
		write("src/lib/format.ts", `export const formatDate = () => "";`)
		write("src/app/page.ts", `import { formatDate } from '../lib/format';`)
		write("src/components/Card.tsx", `import { formatDate } from "@/lib/format";`)
		write("src/lib/format.test.ts", `import { formatDate } from './format.js';`)
		write("src/other.ts", `import { formatMoney } from './lib/money';`)
		return k, write
	}

	t.Run("TS: relative, alias, and test consumers", func(t *testing.T) {
		k, _ := tsFixture(t)
		r := k.Run("", "blast-radius", "src/lib/format.ts")
		r.Want(t, 0)
		want := `blast-radius: src/lib/format.ts
consumers: 3 (source 2, test 1)
src/app/page.ts:1 source
src/components/Card.tsx:1 source alias-match
src/lib/format.test.ts:1 test`
		if got := output(r); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})
	t.Run("a relative path from a subdirectory resolves against the repo", func(t *testing.T) {
		k, _ := tsFixture(t)
		k.Dir = filepath.Join(k.Dir, "src")
		k.Run("", "blast-radius", "lib/format.ts").Has(t, "blast-radius: src/lib/format.ts", "consumers: 3 (source 2, test 1)")
	})
	t.Run("a symbol keeps only consumers that use it", func(t *testing.T) {
		k, write := tsFixture(t)
		write("src/app/page.ts", `import { other } from '../lib/format';`)
		r := k.Run("", "blast-radius", "src/lib/format.ts", "formatDate")
		r.Has(t, "consumers: 2 (source 1, test 1)")
		r.Lacks(t, "src/app/page.ts")
	})
	t.Run("an index file is reached through its directory", func(t *testing.T) {
		k, write := repo(t, "idx")
		write("src/ui/index.ts", `export {};`)
		write("src/main.ts", `import { Button } from './ui';`)
		k.Run("", "blast-radius", "src/ui/index.ts").Has(t, "src/main.ts:1 source")
	})
	t.Run("an index file is reached by a bare '..' or './' from below or beside it", func(t *testing.T) {
		k, write := repo(t, "idx2")
		write("src/foo/index.ts", `export const a = 1;`)
		write("src/foo/sub/b.ts", `import { a } from '..';`)
		write("src/foo/c.ts", `import { a } from "./";`)
		k.Run("", "blast-radius", "src/foo/index.ts").Has(t, "consumers: 2 (source 2, test 0)")
	})
	t.Run("a non-literal import() is flagged", func(t *testing.T) {
		k, write := tsFixture(t)
		write("src/lazy.ts", `const m = await import(path);`)
		k.Run("", "blast-radius", "src/lib/format.ts").Has(t, "unresolvable imports present")
	})
	t.Run("workspace: a package-name import is a workspace-match", func(t *testing.T) {
		k, write := repo(t, "ws")
		write("package.json", `{"name":"root","private":true}`)
		write("pnpm-workspace.yaml", "packages:\n  - apps/*\n  - packages/*")
		write("packages/ui/package.json", `{"name":"@acme/ui"}`)
		write("packages/ui/src/index.ts", `export {};`)
		write("apps/web/package.json", `{"name":"web"}`)
		write("apps/web/src/page.tsx", `import { Button } from '@acme/ui';`)
		write("apps/web/src/other.tsx", `import { x } from '@acme/uikit';`)
		r := k.Run("", "blast-radius", "packages/ui/src/index.ts")
		r.Want(t, 0)
		r.Has(t, "consumers: 1 (source 1, test 0)", "apps/web/src/page.tsx:1 source workspace-match")
	})
	t.Run("Python: absolute, from-parent, relative, and test imports", func(t *testing.T) {
		k, write := repo(t, "py")
		write("app/__init__.py", ``)
		write("app/models.py", `class User: pass`)
		write("app/views.py", `from .models import User`)
		write("app/admin.py", `from app import models`)
		write("scripts/seed.py", `import app.models as m`)
		write("tests/test_models.py", `from app.models import User`)
		write("app/unrelated.py", `from app import views`)
		r := k.Run("", "blast-radius", "app/models.py")
		r.Want(t, 0)
		want := `blast-radius: app/models.py
consumers: 4 (source 3, test 1)
app/admin.py:1 source
app/views.py:1 source
scripts/seed.py:1 source
tests/test_models.py:1 test`
		if got := output(r); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})
	t.Run("an unsupported file type exits 2", func(t *testing.T) {
		k, write := repo(t, "other")
		write("a.rb", `x`)
		k.Run("", "blast-radius", "a.rb").Want(t, 2)
	})
}
