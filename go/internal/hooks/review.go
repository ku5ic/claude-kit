package hooks

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/hook"
)

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
	// A forked skill runs as agent_type "general-purpose"; only this sidecar,
	// an undocumented Claude Code file, names the skill. If it moves, the
	// hook goes quiet rather than nudging every subagent.
	data, err := os.ReadFile(strings.TrimSuffix(transcript, ".jsonl") + ".forked-skill.json")
	if err != nil {
		return nil
	}
	var fork struct {
		SkillName string `json:"skillName"`
	}
	if json.Unmarshal(data, &fork) != nil || fork.SkillName != "code-review" {
		return nil
	}
	return h.Block("before you return: if you haven't run `kit run-checks` in this review, run it in the checkout you reviewed, with a 600000 ms Bash timeout. "+
		"Then return your full report again, every finding you already had unchanged and in the same format, with its `checks: N passed, M failed` line first. "+
		"You own every statement in it: add a FAIL as a finding only after you've verified its cause in the code and are at least 90% sure, never twice for something you already reported; "+
		"for any FAIL you don't report, say in one line why (pre-existing, flaky, unrelated). "+
		"If you reviewed a target you didn't check out, don't run it; open the report with one line saying local checks didn't run.", "review-checks")
}
