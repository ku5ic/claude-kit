package hooks

import (
	"cmp"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

// LogSkills appends one skills.jsonl line per skill activation: a typed
// /skill (UserPromptExpansion), the Skill tool, or a Read of a SKILL.md,
// which is what guard-skills checks the log for, or of a skill reference,
// which counts how often a loaded skill's references get read.
func LogSkills(h *hook.Hook) error {
	// Cheap substring gate before parsing; widen it first if a new shape
	// is added, or nothing logs and guard-skills blocks everything mapped.
	raw := string(h.Payload.Raw)
	if !strings.Contains(raw, "SKILL.md") && !strings.Contains(raw, "Skill") && !strings.Contains(raw, "slash_command") && !strings.Contains(raw, "/reference/") {
		return nil
	}
	if h.Payload.Err != nil {
		return nil
	}
	p := h.Payload
	event := p.String(hook.KeyHookEventName)
	expansion := p.String(hook.KeyExpansionType)
	tool := p.String(hook.KeyToolName)
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
			switch {
			case guard.Glob("*/skills/*/reference/*.md", file):
				// Its own event, so no reader takes it for a load.
				event = kitlog.EventReferenceRead
			case !guard.Glob("*/skills/*/SKILL.md", file):
				return nil
			}
		default:
			return nil
		}
	default:
		return nil
	}
	// `kit scratch-rotate` trims the log to log_max_lines.
	h.Log(kitlog.Skills, event,
		"expansion_type", expansion,
		"command_name", p.String(hook.KeyCommandName),
		"skill_file", cmp.Or(p.String(hook.KeyInputSkill), p.FilePath()),
		"tool_name", tool)
	return nil
}
