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
// tool call, whatever global_skills says.
const rulesSkill = "kit-rules"

// RulesGate is the PreToolUse hook on every tool: it denies the call until
// the session, or the subagent making it, has loaded kit-rules and every
// global_skills skill. A subagent's load counts for it alone, by agent_id.
// The Skill tool and a Read of a required skill's own files are the way to
// load one, and a subagent's hand-back is its reply, so those pass. A
// missing skills log fails open. A fork counts its parent's loads. An
// agent whose transcript shows Claude Code gave it neither Skill nor Read
// passes, logged; one whose tools can't be read is gated.
func RulesGate(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	cfg := h.Config()
	if cfg == nil {
		return nil
	}
	required := []string{rulesSkill}
	for _, skill := range cfg.GlobalSkills {
		if !slices.Contains(required, skill) {
			required = append(required, skill)
		}
	}
	switch p.String("tool_name") {
	case "Skill", "SubagentHandback":
		return nil
	case "Read":
		if slices.ContainsFunc(required, func(skill string) bool { return strings.Contains(p.FilePath(), "/skills/"+skill+"/") }) {
			return nil
		}
	}
	agent := p.String("agent_id")
	marker := func(skill string) string {
		parts := []string{skill}
		if agent != "" {
			parts = append(parts, agent)
		}
		return h.Paths.SessionFile(cache.SkillsLoaded, p.SessionID(), parts...)
	}
	var missing []string
	for _, skill := range required {
		m := marker(skill)
		if m == "" {
			return nil
		}
		if _, err := os.Stat(m); err != nil {
			missing = append(missing, skill)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	missing, err := notLoadedBy(h, agent, missing)
	if err != nil {
		return nil
	}
	if len(missing) > 0 && agent != "" {
		start, _ := transcript.ReadAgentStart(subagentTranscript(p))
		switch {
		case start.Fork:
			// A fork starts from its parent's context, skills included.
			if missing, err = notLoadedBy(h, "", missing); err != nil {
				return nil
			}
		case start.Tools != nil && !slices.Contains(start.Tools, "Skill") && !slices.Contains(start.Tools, "Read"):
			// Claude Code gave this agent no way to load a skill, so a
			// deny could only end its run.
			h.Log(kitlog.Guards, "rules-gate-unloadable", "agent_type", p.String("agent_type"), "tools", strings.Join(start.Tools, ","))
			return nil
		}
	}
	for _, skill := range required {
		if !slices.Contains(missing, skill) && !h.DryRun {
			_ = fsx.WriteAtomic(marker(skill), nil, 0o600)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	files := make([]string, len(missing))
	for i, skill := range missing {
		files[i] = h.Paths.Root + "/skills/" + skill + "/SKILL.md"
	}
	noun := "skill"
	if len(missing) > 1 {
		noun = "skills"
	}
	h.Deny("load the "+strings.Join(missing, " and ")+" "+noun+" first: call the Skill tool now (or Read "+strings.Join(files, " and ")+" if the Skill tool is unavailable), then retry this call. Every tool call stays blocked until you do, so don't stop to ask", "rules-gate")
	return nil
}

// notLoadedBy is the skills of want the skills log doesn't show agent (""
// for the main session) loading.
func notLoadedBy(h *hook.Hook, agent string, want []string) ([]string, error) {
	loaded, err := loadedSkills(h.Paths.LogFile(kitlog.Skills), h.Payload.SessionID(), func(id string) bool { return id == agent })
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, skill := range want {
		if !loaded[skill] && !anyUnder(loaded, skill) {
			missing = append(missing, skill)
		}
	}
	return missing, nil
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

// anyUnder is true when a logged path is /skills/<skill>/SKILL.md: a Read
// of it. A Read of one of its references loads nothing.
func anyUnder(loaded map[string]bool, skill string) bool {
	for file := range loaded {
		if strings.HasSuffix(file, "/skills/"+skill+"/SKILL.md") {
			return true
		}
	}
	return false
}
