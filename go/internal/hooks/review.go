package hooks

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// runChecksCall is a kit run-checks invocation at the start of a command or
// after a shell operator, and its arguments up to the next operator.
var runChecksCall = regexp.MustCompile(`(?m)(?:^|[|;&(]\s*)kit run-checks([^|;&\n]*)`)

// ranChecks reports whether the agent's transcript has a Bash call that ran
// kit run-checks, so a reviewer that already did isn't sent back to re-emit
// its report. --plan only lists the checks, so it doesn't count.
func ranChecks(path string) bool {
	ran := false
	// A read error leaves what was read: the hook fails open.
	_ = transcript.Each(path, func(e transcript.Entry) {
		for _, b := range e.ToolUses() {
			if e.Type != "assistant" || b.Name != "Bash" {
				continue
			}
			// A backslash-newline continues the line, so its --plan still counts.
			command := strings.ReplaceAll(b.Input.Command, "\\\n", " ")
			for _, m := range runChecksCall.FindAllStringSubmatch(command, -1) {
				if !slices.Contains(strings.Fields(m[1]), "--plan") {
					ran = true
				}
			}
		}
	})
	return ran
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
	if h.Config() == nil || project.Toplevel(h.Payload.Cwd()) == "" || ranChecks(transcript) {
		return nil
	}
	return h.Block("before you return: run `kit run-checks` in the checkout you reviewed, timed per rules/tooling.md section 2. "+
		"If you reviewed a target you didn't check out, don't run it. "+
		"Then return your full report again, every finding you already had unchanged and in the same format, reporting the result per rules/verify.md section 1.", "review-checks")
}
