package hooks

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/run"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// reviewSkill is the skill whose forks the review hooks track.
const reviewSkill = "code-review"

// reviewBudget bounds the full gate inside review-checks' 1200s in
// hooks.json, leaving room for planning and the snapshot: a gate starting
// past it is skipped, so the user always gets the notice.
const reviewBudget = 1050 * time.Second

// worktreeAdd matches a command that runs git worktree add, git's own
// options allowed between, and not one that only quotes it.
var worktreeAdd = regexp.MustCompile(`(?:^|[;&|(]\s*)git(?:\s+-\S+(?:\s+[^-\s]\S*)?)*\s+worktree\s+add\b`)

// worktreeReview reports whether the reviewer checked its target out in a
// worktree: a PR reviewed there is gated by its CI, not the checkout here.
func worktreeReview(path string) bool {
	found := false
	_ = transcript.Each(path, func(e transcript.Entry) {
		for _, b := range e.Blocks {
			found = found || b.Type == "tool_use" && (b.Name == "EnterWorktree" || b.Name == "Bash" && worktreeAdd.MatchString(b.Input.Command))
		}
	})
	return found
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
	if cfg == nil || root == "" || worktreeReview(transcript) {
		return nil
	}
	deadline := time.Now().Add(reviewBudget)
	cacheDir := h.Paths.CacheDir()
	// Before the gates: an edit made while they run isn't one they checked.
	tree, snapErr := git.Snapshot(root)
	plan := enforce.Build(cfg, enforce.Options{Root: root, CacheDir: cacheDir, Ask: true, Timeout: gapfill.ClassifyTimeout, Changed: true})
	var report strings.Builder
	res, notice := run.Gates(plan, cacheDir, &report, deadline)
	h.Notify(notice)
	if res.Fail == 0 {
		// Only a whole pass lets Stop skip the tree: a known failure must
		// still block, and a gate the budget cut checked nothing.
		if snapErr == nil && res.Cut == 0 && !h.DryRun {
			_ = fsx.WriteAtomic(cache.FullGateTree(cacheDir, root), []byte(tree), 0o600)
		}
		return nil
	}
	return h.Block("the full gate failed. Handle each FAIL per rules/verify.md section 1, then return your full report again, "+
		"every finding you already had unchanged and in the same format.\n"+report.String(), "review-checks")
}
