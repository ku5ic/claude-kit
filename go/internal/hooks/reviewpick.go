package hooks

import (
	"slices"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// readingAgents are the agents that only read, which a held review lets
// the main thread launch: the kit's read-only ones, every agents/*.md
// without Edit or Write in its tools (TestReadingAgentsListEveryReadOnlyAgent),
// and Claude Code's Explore.
var readingAgents = []string{"kit:auditor", "kit:gatherer", "kit:researcher", "kit:claim-checker", "kit:plan-critic", "Explore"}

// ReviewPick holds the main thread from a finished /code-review until the
// user answers, in a typed prompt or an AskUserQuestion, so no finding is
// fixed before the user picked it (skills/kit-rules/reference/workflow.md
// section 4): an edit to code, and launching an agent that could make one.
// A --fix review asked for its fixes, and a subagent already running was
// started before the review, so neither holds. A Bash write isn't read here.
func ReviewPick(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil || p.String(hook.KeyAgentID) != "" {
		return nil
	}
	tool := p.String(hook.KeyToolName)
	edit := slices.Contains(transcript.EditTools, tool)
	// A workflow's agents can edit, whatever its script asks of them.
	launch := tool == "Workflow" || tool == "Agent" && !slices.Contains(readingAgents, p.String(hook.KeyInputSubagentType))
	if !edit && !launch {
		return nil
	}
	path, cwd := p.String(hook.KeyTranscriptPath), p.Cwd()
	root := git.Toplevel(cwd)
	if root == "" || !fsx.IsFile(path) {
		return nil
	}
	isCode, plans := codeIn(h, root), project.PlanDir(h.Paths, cwd)
	if target := fsx.Abs(cwd, p.FilePath()); edit && (!isCode(target) || fsx.Within(plans, target)) {
		return nil
	}
	if _, walk := readSession(h, path, root, plans, isCode, false); walk.HoldFrom > walk.Answered {
		h.Deny("an edit, or an agent that could make one, before the user picked which /code-review findings to fix\n"+
			layout.KV("do", "stop at the review's report and ask which findings to fix, per skills/kit-rules/reference/workflow.md section 4"), "review-pick")
	}
	return nil
}
