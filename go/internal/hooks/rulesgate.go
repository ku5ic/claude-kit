package hooks

import (
	"os"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

// rulesSkill is the skill every session and subagent loads before its first
// tool call.
const rulesSkill = "kit-rules"

// RulesGate is the PreToolUse hook on every tool: it denies the call until
// the session, or the subagent making it, has loaded the kit-rules skill.
// A subagent's load counts for it alone, by agent_id. The Skill tool and a
// Read of the skill's own files are the way to load it, and a subagent's
// hand-back is its reply, so those pass. A missing skills log fails open.
func RulesGate(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	switch p.String("tool_name") {
	case "Skill", "SubagentHandback":
		return nil
	case "Read":
		if strings.Contains(p.FilePath(), "/skills/"+rulesSkill+"/") {
			return nil
		}
	}
	agent := p.String("agent_id")
	parts := []string{rulesSkill}
	if agent != "" {
		parts = append(parts, agent)
	}
	marker := h.Paths.SessionFile(cache.SkillsLoaded, p.SessionID(), parts...)
	if marker == "" || h.Config() == nil {
		return nil
	}
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	loaded, err := loadedSkills(h.Paths.LogFile(kitlog.Skills), p.SessionID(), func(id string) bool { return id == agent })
	if err != nil {
		return nil
	}
	if loaded[rulesSkill] || anyUnder(loaded, rulesSkill) {
		if !h.DryRun {
			_ = fsx.WriteAtomic(marker, nil, 0o600)
		}
		return nil
	}
	h.Deny("load the kit-rules skill first: call the Skill tool now (or Read "+h.Paths.Root+"/skills/kit-rules/SKILL.md if the Skill tool is unavailable), then retry this call. Every tool call stays blocked until you do, so don't stop to ask", "rules-gate")
	return nil
}

// anyUnder is true when a logged path holds /skills/<skill>/: a Read of its
// SKILL.md.
func anyUnder(loaded map[string]bool, skill string) bool {
	for file := range loaded {
		if strings.Contains(file, "/skills/"+skill+"/") {
			return true
		}
	}
	return false
}
