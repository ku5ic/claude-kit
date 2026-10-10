package hooks

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// Characters are built from code points: a literal one in this file would
// be stripped by the very hook under test.
var (
	emDash = string(rune(0x2014))
	rlo    = string(rune(0x202e)) // right-to-left override
)

// Not parallel: the typography rewrite reads CLAUDE_SANITIZE_TYPOGRAPHY from
// the environment. Each case writes a file under a work dir, runs the hook
// on it, and checks which characters survived.
func TestSanitizeOutput(t *testing.T) {
	sanitize := func(t *testing.T, work, rel string, typography bool) result {
		t.Helper()
		if typography {
			t.Setenv("CLAUDE_SANITIZE_TYPOGRAPHY", "1")
		}
		return newSandbox(t).run("sanitize-output", map[string]any{"tool_input": map[string]any{"file_path": filepath.Join(work, rel)}})
	}
	has := func(t *testing.T, work, rel, char string) bool {
		return strings.Contains(read(t, filepath.Join(work, rel)), char)
	}

	t.Run("default: typography is kept, bidi control characters are stripped", func(t *testing.T) {
		work := t.TempDir()
		testutil.Write(t, filepath.Join(work, "src/app.ts"), "em dash "+emDash+" rlo "+rlo+" here\n")
		r := sanitize(t, work, "src/app.ts", false)
		r.Want(t, 0)
		r.Empty(t)
		if !has(t, work, "src/app.ts", emDash) {
			t.Error("em dash was rewritten")
		}
		if has(t, work, "src/app.ts", rlo) {
			t.Error("bidi control survived")
		}
	})
	t.Run("typography flag on: bidi control characters are stripped too", func(t *testing.T) {
		work := t.TempDir()
		testutil.Write(t, filepath.Join(work, "src/app.ts"), "em dash "+emDash+" rlo "+rlo+" here\n")
		sanitize(t, work, "src/app.ts", true).Want(t, 0)
		if has(t, work, "src/app.ts", emDash) {
			t.Error("em dash survived")
		}
		if has(t, work, "src/app.ts", rlo) {
			t.Error("bidi control survived")
		}
	})
	t.Run("typography flag on: quotes, ellipsis, and arrows become ASCII", func(t *testing.T) {
		work := t.TempDir()
		q := func(r rune) string { return string(r) }
		testutil.Write(t, filepath.Join(work, "src/q.md"), q(0x201c)+"q"+q(0x201d)+" "+q(0x2018)+"s"+q(0x2019)+" "+q(0x2026)+" "+q(0x2192)+" "+q(0x2190)+" "+q(0x21d2)+" "+q(0x2013)+"\n")
		sanitize(t, work, "src/q.md", true).Want(t, 0)
		if got, want := read(t, filepath.Join(work, "src/q.md")), `"q" 's' ... -> <- => -`+"\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	// kit.yml's sanitize_skip: a kept em dash means the path was excluded.
	for _, tc := range []struct {
		name, rel string
		kept      bool
	}{
		{"skip: */locales/* directory is not rewritten", "locales/en.json", true},
		{"skip: */locales/* nested deeper in the tree is not rewritten", "a/locales/b/en.json", true},
		{"skip: */messages/* directory is not rewritten", "messages/en.json", true},
		{"skip: */i18n/* directory is not rewritten", "i18n/en.json", true},
		{"skip: *.snap file is not rewritten", "foo.snap", true},
		{"skip: */fixtures/* directory is not rewritten", "fixtures/data.json", true},
		{"skip: */__snapshots__/* directory is not rewritten", "__snapshots__/x.snap", true},
		{"skip: */testdata/* directory is not rewritten", "testdata/x.json", true},
		{"no false positive: a directory that merely contains 'locales' as a substring is still rewritten", "mylocalesdir/en.json", false},
		{"no false positive: a .snap.bak file does not match the *.snap extension pattern", "foo.snap.bak", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work := filepath.Join(t.TempDir(), "work")
			testutil.Write(t, filepath.Join(work, tc.rel), "em dash "+emDash+" here\n")
			sanitize(t, work, tc.rel, true).Want(t, 0)
			if got := has(t, work, tc.rel, emDash); got != tc.kept {
				t.Errorf("em dash present = %v, want %v", got, tc.kept)
			}
		})
	}
}
