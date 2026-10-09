package hooks

import (
	"cmp"
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
			file := p.FilePath()
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
	h.Log(hook.SkillsLog, event,
		"expansion_type", expansion,
		"command_name", p.String("command_name"),
		"skill_file", cmp.Or(p.String("tool_input.skill"), p.FilePath()),
		"tool_name", tool)
	return nil
}
