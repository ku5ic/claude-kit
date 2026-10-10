package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// Each test writes a fixture file containing a typographic em dash, feeds a
// synthetic Write/Edit payload (pointing at that file) to the hook, and
// checks whether the em dash survived: still present means the path's
// extension/directory excluded it from the rewrite.
//
// Characters are built from code points: a literal one in this file would be
// stripped by the very hook under test.
var (
	sanitizeOutputEmDash = string(rune(0x2014))
	sanitizeOutputRLO    = string(rune(0x202e)) // right-to-left override
)

func TestSanitizeOutput(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*Kit, string) {
		k := New(t)
		return k, filepath.Join(t.TempDir(), "work")
	}
	// fixture creates work/<rel> holding an em dash.
	fixture := func(t *testing.T, work, rel string) {
		Write(t, filepath.Join(work, rel), "em dash "+sanitizeOutputEmDash+" here\n")
	}
	// bidiFixture creates work/<rel> holding an em dash and a right-to-left
	// override.
	bidiFixture := func(t *testing.T, work, rel string) {
		Write(t, filepath.Join(work, rel), "em dash "+sanitizeOutputEmDash+" rlo "+sanitizeOutputRLO+" here\n")
	}
	// hook feeds the Write-payload shape to the hook; typography turns the
	// rewrite on so the em dash marker exercises the exclusions, off leaves
	// CLAUDE_SANITIZE_TYPOGRAPHY unset.
	hook := func(k *Kit, work, rel string, typography bool) Result {
		if typography {
			k.Setenv("CLAUDE_SANITIZE_TYPOGRAPHY", "1")
		}
		return k.Hook("sanitize-output", map[string]any{"tool_input": map[string]any{"file_path": filepath.Join(work, rel)}})
	}
	has := func(t *testing.T, work, rel, char string) bool {
		return strings.Contains(Read(t, filepath.Join(work, rel)), char)
	}

	t.Run("baseline: a non-excluded path gets its em dash rewritten", func(t *testing.T) {
		t.Parallel()
		k, work := setup(t)
		fixture(t, work, "src/app.ts")
		r := hook(k, work, "src/app.ts", true)
		r.Want(t, 0)
		r.Empty(t)
		if has(t, work, "src/app.ts", sanitizeOutputEmDash) {
			t.Error("em dash survived")
		}
	})

	t.Run("default: typography is kept, bidi control characters are stripped", func(t *testing.T) {
		t.Parallel()
		k, work := setup(t)
		bidiFixture(t, work, "src/app.ts")
		r := hook(k, work, "src/app.ts", false)
		r.Want(t, 0)
		r.Empty(t)
		if !has(t, work, "src/app.ts", sanitizeOutputEmDash) {
			t.Error("em dash was rewritten")
		}
		if has(t, work, "src/app.ts", sanitizeOutputRLO) {
			t.Error("bidi control survived")
		}
	})

	t.Run("typography flag on: bidi control characters are stripped too", func(t *testing.T) {
		t.Parallel()
		k, work := setup(t)
		bidiFixture(t, work, "src/app.ts")
		r := hook(k, work, "src/app.ts", true)
		r.Want(t, 0)
		if has(t, work, "src/app.ts", sanitizeOutputEmDash) {
			t.Error("em dash survived")
		}
		if has(t, work, "src/app.ts", sanitizeOutputRLO) {
			t.Error("bidi control survived")
		}
	})

	t.Run("typography flag on: quotes, ellipsis, and arrows become ASCII", func(t *testing.T) {
		t.Parallel()
		k, work := setup(t)
		q := func(r rune) string { return string(r) }
		Write(t, filepath.Join(work, "src/q.md"), q(0x201c)+"q"+q(0x201d)+" "+q(0x2018)+"s"+q(0x2019)+" "+q(0x2026)+" "+q(0x2192)+" "+q(0x2190)+" "+q(0x21d2)+" "+q(0x2013)+"\n")
		r := hook(k, work, "src/q.md", true)
		r.Want(t, 0)
		// $(cat) drops the trailing newline.
		if got, want := strings.TrimRight(Read(t, filepath.Join(work, "src/q.md")), "\n"), `"q" 's' ... -> <- => -`; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

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
			t.Parallel()
			k, work := setup(t)
			fixture(t, work, tc.rel)
			hook(k, work, tc.rel, true).Want(t, 0)
			if got := has(t, work, tc.rel, sanitizeOutputEmDash); got != tc.kept {
				t.Errorf("em dash present = %v, want %v", got, tc.kept)
			}
		})
	}
}
