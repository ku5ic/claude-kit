package hooks

import (
	"fmt"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// StopChecks is the Stop hook: when the turn created or edited files through
// Edit/Write/MultiEdit/NotebookEdit and left the tree dirty, it runs
// kit.yml's file_checks on just those files and blocks the stop on a
// failure, so Claude fixes or reports it. A question-only turn costs
// nothing. When the turn ticked a plan's last open step, it also blocks
// until /code-review has run since the last code
// edit; it never runs the suite itself.
func StopChecks(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	// Already continuing because of this hook: one retry per failure, so a
	// check that can't be fixed never loops.
	if h.Payload.Bool("stop_hook_active") {
		return nil
	}
	cwd := h.Payload.Cwd()
	if out, err := git.Line(cwd, "rev-parse", "--is-inside-work-tree"); err != nil || out != "true" {
		return nil
	}
	transcript := h.Payload.String("transcript_path")
	if !project.IsFile(transcript) {
		return nil
	}
	root, err := git.Line(cwd, "rev-parse", "--show-toplevel") // already physical
	cfg := h.Config()
	if err != nil || cfg == nil {
		return nil
	}
	// Before the clean-tree return: plans are gitignored, so ticking the
	// last step leaves the tree clean.
	if err := planGate(h, cfg, cwd, root, transcript); err != nil {
		return err
	}
	// A clean tree means the edits were committed, which already went
	// through verification.
	if status, err := git.Output(cwd, "status", "--porcelain"); err != nil || status == "" {
		return nil
	}
	edited, err := checks.EditedFiles(transcript)
	if err != nil || len(edited) == 0 {
		return nil
	}
	out := checks.FileChecks(cfg, root, cwd, edited)
	if out == nil {
		return nil
	}
	if out.Failed {
		if err := h.Block("file checks failed; fix them or report and stop.\n"+out.Failures+out.Summary, "checks-failed"); err != nil {
			return err
		}
	}
	hook.WriteJSON(h.Stdout, map[string]string{"systemMessage": h.Name + ":\n" + out.Report + out.Summary})
	return nil
}

// planGate blocks the stop when this turn ticked a plan's last step without
// a review since the last code edit.
func planGate(h *hook.Hook, cfg *config.Config, cwd, root, transcript string) error {
	dir, err := project.Dir(cfg, h.Paths, cwd, "plans", false)
	if err != nil {
		return nil
	}
	isCode := func(path string) bool {
		return strings.HasPrefix(project.PhysicalPath(path), root+"/") && !project.IsScratch(h.Paths, path)
	}
	if plan, reviewed := planDone(transcript, dir, isCode); plan != "" {
		return endOfPlan(h, project.Rel(root, plan), reviewed)
	}
	return nil
}

// endOfPlan blocks a finished plan's stop until /code-review has run since
// the last code edit (rules/verify.md).
func endOfPlan(h *hook.Hook, plan string, reviewed bool) error {
	if reviewed {
		return nil
	}
	return h.Block(fmt.Sprintf("plan %s is done, but /code-review hasn't finished since the last code edit. Run it now, or wait for the one running, then the runtime pass (/verify) when the change has observable behavior.", plan), "plan-done")
}
