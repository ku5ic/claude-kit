package hooks

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
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
// missing skills log shows nothing loaded, unless the transcript shows a
// Skill call; then, or when the log can't be read, it fails open, told
// to the user. A fork counts its parent's loads. A compaction voids the main
// session's own (rearmRulesGate). An agent whose transcript shows Claude Code
// gave it neither Skill nor Read passes, logged; one whose tools can't be
// read is gated.
func RulesGate(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	tool := p.String(hook.KeyToolName)
	if gatePasses(tool) {
		return nil
	}
	cfg := h.Config()
	if cfg == nil {
		return nil
	}
	required := requiredSkills(cfg)
	if tool == "Read" && slices.ContainsFunc(required, func(skill string) bool { return inSkill(h, skill, p.FilePath()) }) {
		return nil
	}
	agent := p.String(hook.KeyAgentID)
	marker := func(skill string) string { return rulesMarker(h, agent, skill) }
	if marker(rulesSkill) == "" {
		return errors.New("no session_id, so no skill load can be told apart")
	}
	var missing []string
	for _, skill := range required {
		if _, err := os.Stat(marker(skill)); err != nil {
			missing = append(missing, skill)
		}
	}
	// A skill nothing can load (a typo, a removed skill) would deny every
	// call for good, so it's logged and left out.
	missing = slices.DeleteFunc(missing, func(skill string) bool {
		if skill == rulesSkill || skillExists(h, skill) {
			return false
		}
		h.Log(kitlog.Guards, "rules-gate-unknown-skill", "skill", skill)
		return true
	})
	if len(missing) == 0 {
		return nil
	}
	missing, err := notLoadedBy(h, agent, missing)
	if err != nil {
		return err
	}
	if len(missing) > 0 && agent != "" {
		start := subagentStart(p)
		switch {
		case start.Fork:
			if missing, err = notLoadedBy(h, "", missing); err != nil {
				return err
			}
		case withoutTools(start, "Skill", "Read"):
			h.Log(kitlog.Guards, "rules-gate-unloadable", "agent_type", p.String(hook.KeyAgentType), "tools", strings.Join(start.Tools, ","))
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
	noun := "skill isn't"
	if len(missing) > 1 {
		noun = "skills aren't"
	}
	h.Deny("the "+strings.Join(missing, " and ")+" "+noun+" loaded yet\n"+
		layout.KV("do", "call the Skill tool now, or Read "+strings.Join(files, " and ")+" if the Skill tool is unavailable")+"\n"+
		layout.KV("then", "retry this call; every tool call stays blocked until you do, so don't stop to ask"), "rules-gate")
	return nil
}

// requiredSkills is kit-rules and every global_skills skill.
func requiredSkills(cfg *config.Config) []string {
	required := []string{rulesSkill}
	for _, skill := range cfg.GlobalSkills {
		if skill != "" && !slices.Contains(required, skill) {
			required = append(required, skill)
		}
	}
	return required
}

// rulesMarker is the gate's marker that agent ("" for the main session)
// loaded skill, or "" when the session can't be named. Only the gate writes
// it, so no other check's load can lift the gate.
func rulesMarker(h *hook.Hook, agent, skill string) string {
	parts := []string{skill}
	if agent != "" {
		parts = append(parts, agent)
	}
	return h.Paths.SessionFile(cache.RulesLoaded, h.Payload.SessionID(), parts...)
}

// rearmRulesGate makes the gate ask the main session for every required
// skill again after a compaction. The log line goes first: a marker removed
// before it could be written again from the loads it voids.
func rearmRulesGate(h *hook.Hook, cfg *config.Config) {
	h.Log(kitlog.Skills, kitlog.EventCompacted, "cwd", h.Payload.String(hook.KeyCwd))
	if h.DryRun {
		return
	}
	for _, skill := range requiredSkills(cfg) {
		_ = os.Remove(rulesMarker(h, "", skill)) // a missing marker is already rearmed
	}
}

// notLoadedBy is the skills of want the skills log doesn't show agent (""
// for the main session) loading. A missing log shows none loaded, unless
// agent's transcript shows a Skill call: its load wasn't logged. That, or
// a log that can't be read, is an error: a deny could never lift, so the
// gate fails open, telling the user.
func notLoadedBy(h *hook.Hook, agent string, want []string) ([]string, error) {
	loaded, err := loadedSkills(h.Paths.LogFile(kitlog.Skills), h.Payload.SessionID(), func(id string) bool { return id == agent })
	if errors.Is(err, fs.ErrNotExist) {
		if ranSkill(h.Payload, agent) {
			return nil, errors.New("the skills log is missing though the Skill tool ran, so its load wasn't logged")
		}
		return want, nil
	}
	if err != nil {
		return nil, fmt.Errorf("the skills log can't be read, so no skill load shows: %w", err)
	}
	return notIn(h, loaded, want), nil
}

// ranSkill is true when the transcript of agent ("" for the main
// session) holds a Skill tool call.
func ranSkill(p *hook.Payload, agent string) bool {
	path := p.String(hook.KeyTranscriptPath)
	if agent != "" {
		path = agentTranscriptPath(path, agent)
	}
	if path == "" {
		return false
	}
	ran := false
	_ = transcript.Each(path, func(e transcript.Entry) {
		ran = ran || slices.ContainsFunc(e.ToolUses(), func(b transcript.Block) bool { return b.Name == "Skill" })
	})
	return ran
}

// notIn is the skills of want that loaded has neither by name (the Skill
// tool) nor as a Read of its SKILL.md in a skills dir.
func notIn(h *hook.Hook, loaded map[string]bool, want []string) []string {
	var missing []string
	for _, skill := range want {
		if !loaded[skill] && !readSkill(h, loaded, skill) {
			missing = append(missing, skill)
		}
	}
	return missing
}

// skillExists is true when name has a SKILL.md in the kit, the user's
// skills, or the project's, or names another plugin's skill, which the kit
// can't look up.
func skillExists(h *hook.Hook, name string) bool {
	if plugin, skill, ok := strings.Cut(name, ":"); ok {
		if plugin != "kit" {
			return true
		}
		name = skill
	}
	if name == "" || strings.ContainsAny(name, `/\`) {
		return false
	}
	return slices.ContainsFunc(skillDirs(h), func(dir string) bool { return fsx.IsFile(filepath.Join(dir, name, "SKILL.md")) })
}

// skillDirs are where a skill may live: the kit's, the user's, and the
// project's skills dirs.
func skillDirs(h *hook.Hook) []string {
	dirs := []string{filepath.Join(h.Paths.Root, "skills"), filepath.Join(h.Paths.ClaudeDir, "skills")}
	if root, ok := project.Root(h.Payload.Cwd()); ok {
		dirs = append(dirs, filepath.Join(root, ".claude", "skills"))
	}
	return dirs
}

// inSkill is true when path, cleaned, lies in skill's folder in one of the
// skills dirs; a kit: prefix names the kit's own skill.
func inSkill(h *hook.Hook, skill, path string) bool {
	skill = strings.TrimPrefix(skill, "kit:")
	if path == "" || skill == "" || strings.ContainsAny(skill, `/\:`) {
		return false
	}
	path = filepath.Clean(path)
	return slices.ContainsFunc(skillDirs(h), func(dir string) bool { return fsx.Within(filepath.Join(dir, skill), path) })
}

// gatePasses is true for a tool rules-gate lets through: the Skill tool is
// how a skill loads, and a subagent's hand-back is its reply.
func gatePasses(tool string) bool {
	return tool == "Skill" || tool == "SubagentHandback"
}

// subagentStart is how the subagent making a tool call started; zero for
// the main session or a transcript that can't be read.
func subagentStart(p *hook.Payload) transcript.AgentStart {
	start, _ := transcript.ReadAgentStart(subagentTranscript(p))
	return start
}

// withoutTools is true when Claude Code gave the agent none of tools, so a
// deny asking for them could only end its run. A fork starts from its
// parent's context, skills included, and is handled before this.
func withoutTools(start transcript.AgentStart, tools ...string) bool {
	return start.Tools != nil && !slices.ContainsFunc(tools, func(tool string) bool { return slices.Contains(start.Tools, tool) })
}

// subagentTranscript is the transcript of the subagent making a tool call,
// at <session>/subagents/agent-<agent_id>.jsonl
// (code.claude.com/docs/en/sub-agents), or "" when it can't be named.
func subagentTranscript(p *hook.Payload) string {
	return agentTranscriptPath(p.String(hook.KeyTranscriptPath), p.String(hook.KeyAgentID))
}

// agentTranscriptPath is the transcript of agent in the session whose
// transcript is main, or "" when it can't be named.
func agentTranscriptPath(main, agent string) string {
	if main == "" || agent == "" || strings.ContainsAny(agent, `/\`) {
		return ""
	}
	return filepath.Join(strings.TrimSuffix(main, ".jsonl"), "subagents", "agent-"+agent+".jsonl")
}

// readSkill is true when a logged path is skill's SKILL.md in one of the
// skills dirs: a Read of it. A Read of one of its references, or of a
// SKILL.md anywhere else, loads nothing.
func readSkill(h *hook.Hook, loaded map[string]bool, skill string) bool {
	name := strings.TrimPrefix(skill, "kit:")
	dirs := skillDirs(h)
	for file := range loaded {
		if slices.ContainsFunc(dirs, func(dir string) bool { return filepath.Clean(file) == filepath.Join(dir, name, "SKILL.md") }) {
			return true
		}
	}
	return false
}
