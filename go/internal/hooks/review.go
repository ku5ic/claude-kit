package hooks

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// reviewSkill is the skill whose forks the review hooks track.
const reviewSkill = "code-review"

// persisted is where Claude Code saved a tool result too large to inline.
var persisted = regexp.MustCompile(`(?s)^\s*<persisted-output>.*?saved to: (\S+)`)

// ranChecks reports whether the agent's transcript shows a kit run-checks
// run, by its summary line in a tool result (or the file a large one was
// saved to), so a reviewer that already ran it isn't sent back to re-emit
// its report. Reading the output, not the command, needs no shell parsing:
// a mention in quotes prints no summary.
func ranChecks(path string) bool {
	ran := false
	// A read error leaves what was read: the hook fails open.
	_ = transcript.Each(path, func(e transcript.Entry) {
		for _, b := range e.Blocks {
			if ran || b.Type != "tool_result" {
				continue
			}
			text := b.ResultText()
			if m := persisted.FindStringSubmatch(text); m != nil {
				if data, err := os.ReadFile(m[1]); err == nil {
					text = string(data)
				}
			}
			ran = checks.Summary.MatchString(text)
		}
	})
	return ran
}

// forkedSkill is the skill a forked subagent runs: from Claude Code's
// .forked-skill.json sidecar, else, for a foreground fork that writes none,
// "code-review" when the parent's code-review Skill call has no result yet.
func forkedSkill(agentTranscript, parent string) string {
	if data, err := os.ReadFile(strings.TrimSuffix(agentTranscript, ".jsonl") + ".forked-skill.json"); err == nil {
		var fork struct {
			SkillName string `json:"skillName"`
		}
		_ = json.Unmarshal(data, &fork)
		return fork.SkillName
	}
	pending := map[string]bool{}
	_ = transcript.Each(parent, func(e transcript.Entry) {
		for _, b := range e.Blocks {
			switch {
			case b.Type == "tool_use" && b.Name == "Skill" && b.Input.Skill == reviewSkill:
				pending[b.ID] = true
			case b.Type == "tool_result":
				delete(pending, b.ToolUseID)
			}
		}
	})
	if len(pending) > 0 {
		return reviewSkill
	}
	return ""
}

// ReviewChecks holds a forked /code-review at its first stop (SubagentStop)
// and sends it back to run the full suite (rules/verify.md) and open its
// report with the result. The reviewer knows its own target, so it, not this
// hook, decides whether local checks apply. stop_hook_active lets the second
// stop through, so it nudges once and never loops.
func ReviewChecks(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil || p.Bool("stop_hook_active") {
		return nil
	}
	transcript := p.String("agent_transcript_path")
	if transcript == "" {
		return nil
	}
	// A forked skill runs as agent_type "general-purpose"; only undocumented
	// Claude Code records name the skill. If they move, the hook goes quiet
	// rather than nudging every subagent.
	if forkedSkill(transcript, p.String("transcript_path")) != reviewSkill {
		return nil
	}
	if h.Config() == nil || project.Toplevel(h.Payload.Cwd()) == "" || ranChecks(transcript) {
		return nil
	}
	return h.Block("before you return: run `kit run-checks` in the checkout you reviewed, timed per rules/tooling.md section 2. "+
		"If you reviewed a target you didn't check out, don't run it. "+
		"Then return your full report again, every finding you already had unchanged and in the same format, reporting the result per rules/verify.md section 1.", "review-checks")
}
