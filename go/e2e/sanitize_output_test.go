package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// An em dash built from its code point: a literal one in this file would be
// stripped by the very hook under test.
var sanitizeOutputEmDash = string(rune(0x2014))

// `kit hook sanitize-output` wiring: the binary rewrites the edited file in
// place, reads CLAUDE_SANITIZE_TYPOGRAPHY, and prints nothing. What it
// rewrites and skips is tested in process, in internal/hooks.
func TestSanitizeOutput(t *testing.T) {
	t.Parallel()
	t.Run("baseline: a non-excluded path gets its em dash rewritten", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		k.Setenv("CLAUDE_SANITIZE_TYPOGRAPHY", "1")
		file := filepath.Join(t.TempDir(), "work", "src/app.ts")
		Write(t, file, "em dash "+sanitizeOutputEmDash+" here\n")
		r := k.Hook("sanitize-output", map[string]any{"tool_input": map[string]any{"file_path": file}})
		r.Want(t, 0)
		r.Empty(t)
		if strings.Contains(Read(t, file), sanitizeOutputEmDash) {
			t.Error("em dash survived")
		}
	})
}
