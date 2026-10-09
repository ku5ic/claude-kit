package hooks

import (
	"bytes"
	"cmp"
	"os"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
)

// LogSkills appends one skills.jsonl line per skill activation: a typed
// /skill (UserPromptExpansion), the Skill tool, or a Read of a SKILL.md,
// which is what guard-skills checks the log for.
func LogSkills(h *hook.Hook) error {
	// Cheap substring gate before parsing; widen it first if a new shape
	// is added, or nothing logs and guard-skills blocks everything mapped.
	raw := string(h.Payload.Raw)
	if !strings.Contains(raw, "SKILL.md") && !strings.Contains(raw, "Skill") && !strings.Contains(raw, "slash_command") {
		return nil
	}
	if h.Payload.Err != nil {
		return nil
	}
	p := h.Payload
	event := p.String("hook_event_name")
	expansion := p.String("expansion_type")
	tool := p.String("tool_name")
	switch event {
	case "UserPromptExpansion":
		if expansion != "slash_command" {
			return nil
		}
	case "PostToolUse":
		switch tool {
		case "Skill":
		case "Read":
			file := p.String("tool_input.file_path")
			if !guard.Glob("*/skills/*/SKILL.md", file) {
				return nil
			}
		default:
			return nil
		}
	default:
		return nil
	}
	// `kit scratch-rotate` trims the log to log_max_lines.
	h.Log("skills", event,
		"expansion_type", expansion,
		"command_name", p.String("command_name"),
		"skill_file", cmp.Or(p.String("tool_input.skill"), p.String("tool_input.file_path")),
		"tool_name", tool)
	return nil
}

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
	if cfg := h.Config(); cfg != nil && slices.ContainsFunc(cfg.SanitizeSkip, func(p string) bool { return guard.Glob(p, path) }) {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil || !isText(data) {
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

// isText is git's binary heuristic: no NUL byte in the first 8000 bytes.
func isText(data []byte) bool {
	return !bytes.Contains(data[:min(len(data), 8000)], []byte{0})
}
