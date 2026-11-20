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

// ReviewPick holds the main thread's code edits from a finished
// /code-review until the user answers, in a typed prompt or an
// AskUserQuestion, so no finding is fixed before the user picked it
// (skills/kit-rules/reference/workflow.md section 4). A --fix review asked
// for its fixes, and a subagent's edit was started before the review, so
// neither holds. A Bash write isn't read here.
func ReviewPick(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil || p.String(hook.KeyAgentID) != "" || !slices.Contains(transcript.EditTools, p.String(hook.KeyToolName)) {
		return nil
	}
	path, cwd := p.String(hook.KeyTranscriptPath), p.Cwd()
	root := git.Toplevel(cwd)
	if root == "" || !fsx.IsFile(path) {
		return nil
	}
	isCode, plans := codeIn(h, root), project.PlanDir(h.Paths, cwd)
	if target := fsx.Abs(cwd, p.FilePath()); !isCode(target) || fsx.Within(plans, target) {
		return nil
	}
	if _, walk := readSession(h, path, root, plans, isCode); walk.HoldFrom > walk.Answered {
		h.Deny("an edit before the user picked which /code-review findings to fix\n"+
			layout.KV("do", "stop at the review's report and ask which findings to fix, per skills/kit-rules/reference/workflow.md section 4"), "review-pick")
	}
	return nil
}
