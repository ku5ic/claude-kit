package hooks

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// rulesSkill is the skill every session and subagent loads before its first
// tool call.
const rulesSkill = "kit-rules"

// RulesGate is the PreToolUse hook on every tool: it denies the call until
// the session, or the subagent making it, has loaded the kit-rules skill.
// A subagent's load counts for it alone, by agent_id. The Skill tool and a
// Read of the skill's own files are the way to load it, and a subagent's
// hand-back is its reply, so those pass. A missing skills log fails open.
// A fork counts its parent's load. An agent whose transcript shows Claude
// Code gave it neither Skill nor Read passes, logged; one whose tools
// can't be read is gated.
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
	loaded, err := rulesLoaded(h, agent)
	if err != nil {
		return nil
	}
	if !loaded && agent != "" {
		start, _ := transcript.ReadAgentStart(subagentTranscript(p))
		switch {
		case start.Fork:
			// A fork starts from its parent's context, rules included.
			if loaded, err = rulesLoaded(h, ""); err != nil {
				return nil
			}
		case start.Tools != nil && !slices.Contains(start.Tools, "Skill") && !slices.Contains(start.Tools, "Read"):
			// Claude Code gave this agent no way to load the skill, so a
			// deny could only end its run.
			h.Log(kitlog.Guards, "rules-gate-unloadable", "agent_type", p.String("agent_type"), "tools", strings.Join(start.Tools, ","))
			return nil
		}
	}
	if loaded {
		if !h.DryRun {
			_ = fsx.WriteAtomic(marker, nil, 0o600)
		}
		return nil
	}
	h.Deny("load the kit-rules skill first: call the Skill tool now (or Read "+h.Paths.Root+"/skills/kit-rules/SKILL.md if the Skill tool is unavailable), then retry this call. Every tool call stays blocked until you do, so don't stop to ask", "rules-gate")
	return nil
}

// rulesLoaded reports whether the skills log shows agent ("" for the main
// session) loading the kit-rules skill.
func rulesLoaded(h *hook.Hook, agent string) (bool, error) {
	loaded, err := loadedSkills(h.Paths.LogFile(kitlog.Skills), h.Payload.SessionID(), func(id string) bool { return id == agent })
	return loaded[rulesSkill] || anyUnder(loaded, rulesSkill), err
}

// subagentTranscript is the transcript of the subagent making a tool call,
// at <session>/subagents/agent-<agent_id>.jsonl
// (code.claude.com/docs/en/sub-agents), or "" when it can't be named.
func subagentTranscript(p *hook.Payload) string {
	main, agent := p.String("transcript_path"), p.String("agent_id")
	if main == "" || agent == "" || strings.ContainsAny(agent, `/\`) {
		return ""
	}
	return filepath.Join(strings.TrimSuffix(main, ".jsonl"), "subagents", "agent-"+agent+".jsonl")
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
