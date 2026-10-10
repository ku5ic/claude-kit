package hooks

import (
	"fmt"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
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
	path := h.Payload.String("transcript_path")
	if !fsx.IsFile(path) {
		return nil
	}
	root := git.Toplevel(cwd) // "" outside a work tree
	cfg := h.Config()
	if root == "" || cfg == nil {
		return nil
	}
	// A read error leaves what was read: the hook fails open.
	entries, _ := transcript.Load(path)
	// Before the clean-tree return: plans are gitignored, so ticking the
	// last step leaves the tree clean.
	if err := planGate(h, cfg, cwd, root, entries); err != nil {
		return err
	}
	// A clean tree means the edits were committed, which already went
	// through verification.
	if changes, err := git.Status(cwd, false); err != nil || len(changes) == 0 {
		return nil
	}
	edited := transcript.Edits(transcript.LastTurn(entries))
	if len(edited) == 0 {
		return nil
	}
	out := checks.FileChecks(cfg, root, cwd, edited)
	if out == nil {
		return nil
	}
	// Silent on a pass: what ran and what was skipped is kept for kit
	// explain stop.
	if !h.DryRun {
		_ = fsx.WriteAtomic(cache.StopReport(h.Paths.CacheDir(), root), []byte(out.Report+out.Summary), 0o600)
	}
	if out.Failed {
		return h.Block("file checks failed; fix them or report and stop.\n"+out.Failures+out.Summary, "checks-failed")
	}
	return nil
}

// planGate blocks the stop when this turn ticked a plan's last step without
// a review since the last code edit.
func planGate(h *hook.Hook, cfg *config.Config, cwd, root string, entries []transcript.Entry) error {
	dir, err := project.Dir(cfg, h.Paths, cwd, "plans", false)
	if err != nil {
		return nil
	}
	isCode := func(path string) bool {
		return strings.HasPrefix(fsx.PhysicalPath(path), root+"/") && !project.IsScratch(h.Paths, path)
	}
	// A finished plan waits for /code-review since the last code edit (rules/verify.md).
	if plan, reviewed := planDone(entries, dir, isCode); plan != "" && !reviewed {
		return h.Block(fmt.Sprintf("plan %s is done, but /code-review hasn't finished since the last code edit. Run it now, or wait for the one running, then the runtime pass (/verify) when the change has observable behavior.", project.Rel(root, plan)), "plan-done")
	}
	return nil
}
