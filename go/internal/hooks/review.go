package hooks

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/run"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// reviewSkill is the skill whose forks the review hooks track.
const reviewSkill = "code-review"

// classifyTimeout bounds asking the classifier for what the gap-fill cache
// lacks, as kit run-checks does.
const classifyTimeout = 60 * time.Second

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

// ReviewChecks runs the full gate, scoped to what changed, when a forked
// /code-review first stops (SubagentStop) in a checkout, and shows the user
// every verdict. A failure sends the reviewer back with it, to handle per
// rules/verify.md section 1; stop_hook_active lets the second stop through,
// so it runs once and never loops. The tree it checked is kept, so the
// turn's Stop doesn't check it again.
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
	cfg, root := h.Config(), git.Toplevel(h.Payload.Cwd())
	if cfg == nil || root == "" {
		return nil
	}
	cacheDir := h.Paths.CacheDir()
	plan := enforce.Build(cfg, enforce.Options{Root: root, CacheDir: cacheDir, Ask: true, Timeout: classifyTimeout, Changed: true})
	var report strings.Builder
	res := run.Gates(plan, cacheDir, &report)
	if tree, err := git.Snapshot(root); err == nil && !h.DryRun {
		_ = fsx.WriteAtomic(cache.FullGateTree(cacheDir, root), []byte(tree), 0o600)
	}
	if notice := run.Notice(report.String()); notice != "" {
		h.Notify("run-checks " + notice)
	}
	if res.Fail == 0 {
		return nil
	}
	return h.Block("the full gate failed. Handle each FAIL per rules/verify.md section 1, then return your full report again, "+
		"every finding you already had unchanged and in the same format.\n"+report.String(), "review-checks")
}
