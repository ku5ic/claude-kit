package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/resolve/kittool"
)

// `kit blast-radius` wiring; who uses a file is tested in
// internal/blast.
func TestBlastRadius(t *testing.T) {
	t.Parallel()
	if _, err := kittool.Path("ast-grep"); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal(err) // CI installs ast-grep
		}
		t.Skip(err) // kit: allow ast-grep is optional on a dev machine
	}
	t.Run("TS: relative, alias, and test consumers", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		dir := filepath.Join(Physical(t, t.TempDir()), "ts")
		Mkdir(t, dir)
		k.Git(dir, "init", "-q", "-b", "main")
		k.Dir = dir
		for path, content := range map[string]string{
			"src/lib/format.ts":       `export const formatDate = () => "";`,
			"src/app/page.ts":         `import { formatDate } from '../lib/format';`,
			"src/components/Card.tsx": `import { formatDate } from "@/lib/format";`,
			"src/lib/format.test.ts":  `import { formatDate } from './format.js';`,
			"src/other.ts":            `import { formatMoney } from './lib/money';`,
		} {
			Write(t, filepath.Join(dir, path), content+"\n")
		}
		r := k.Run("", "blast-radius", "src/lib/format.ts")
		r.Want(t, 0)
		want := `blast-radius: src/lib/format.ts
consumers: 3 (source 2, test 1)
src/app/page.ts:1 source
src/components/Card.tsx:1 source alias-match
src/lib/format.test.ts:1 test`
		if got := strings.TrimRight(r.Output, "\n"); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})
	t.Run("Go: the package's importers and its own files, through code-graph", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		dir := filepath.Join(Physical(t, t.TempDir()), "go")
		Mkdir(t, dir)
		k.Git(dir, "init", "-q", "-b", "main")
		k.Dir = dir
		for path, content := range map[string]string{
			"go.mod":          "module example.com/m\n\ngo 1.27",
			"lib/format.go":   "package lib\n\nfunc Format() {}",
			"lib/parse.go":    "package lib\n\nfunc Parse() {}",
			"cmd/app/main.go": "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() { lib.Format() }",
		} {
			Write(t, filepath.Join(dir, path), content+"\n")
		}
		r := k.Run("", "blast-radius", "lib/format.go")
		r.Want(t, 0)
		want := `blast-radius: lib/format.go
consumers: 2 (source 2, test 0)
cmd/app/main.go:3 source
lib/parse.go:1 source same-package`
		if got := strings.TrimRight(r.Output, "\n"); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})
}
