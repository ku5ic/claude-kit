package hooks

import (
	"bytes"
	"os"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
)

var (
	// Trojan Source: bidi embedding, override, and isolate controls.
	// Code points, never the characters or string escapes: this file is
	// itself run through the sanitizer when edited, and an escape can be
	// decoded on its way in.
	bidi = []rune{0x202A, 0x202B, 0x202C, 0x202D, 0x202E, 0x2066, 0x2067, 0x2068, 0x2069}

	typography = strings.NewReplacer(
		string(rune(0x2014)), "-", // em dash
		string(rune(0x2013)), "-", // en dash
		string(rune(0x201C)), `"`,
		string(rune(0x201D)), `"`,
		string(rune(0x2018)), "'",
		string(rune(0x2019)), "'",
		string(rune(0x2026)), "...",
		string(rune(0x2192)), "->",
		string(rune(0x2190)), "<-",
		string(rune(0x21D2)), "=>",
	)
)

// SanitizeOutput strips bidi control characters from a written text file,
// and with CLAUDE_SANITIZE_TYPOGRAPHY=1 rewrites em dashes, smart quotes,
// ellipses, and arrows to ASCII. PostToolUse for Write, Edit, MultiEdit.
// Files kit.yml's sanitize_skip matches are left alone.
func SanitizeOutput(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	path := h.Payload.FilePath()
	info, err := os.Stat(path)
	if path == "" || err != nil || !info.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(path)
	// Every rune above is U+2xxx, which UTF-8 leads with 0xE2: without that
	// byte there's nothing to do, and the config load is skipped.
	if err != nil || bytes.IndexByte(data, 0xE2) < 0 || git.IsBinary(data) {
		return nil
	}
	if cfg := h.Config(); cfg != nil && guard.GlobAny(cfg.SanitizeSkip, path) {
		return nil
	}
	out := string(data)
	for _, c := range bidi {
		out = strings.ReplaceAll(out, string(c), "")
	}
	if os.Getenv("CLAUDE_SANITIZE_TYPOGRAPHY") == "1" {
		out = typography.Replace(out)
	}
	if out != string(data) {
		return os.WriteFile(path, []byte(out), info.Mode().Perm())
	}
	return nil
}
