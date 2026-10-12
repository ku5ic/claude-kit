package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// `kit blast-radius` wiring; the import scanners are tested in
// internal/blast.
func TestBlastRadius(t *testing.T) {
	t.Parallel()
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
}
