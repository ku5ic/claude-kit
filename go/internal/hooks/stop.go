package hooks

import (
	"fmt"
	"os"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/run"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// StopChecks is the Stop hook: when the turn created or edited files through
// Edit/Write/MultiEdit/NotebookEdit and left the tree dirty, it runs the
// project's file-scoped checks (enforce.Stop) on just those files and blocks
// the stop on a failure, so Claude fixes or reports it. A question-only turn costs
// nothing. When the turn ticked a plan's last open step, it also blocks
// until /code-review has run since the last code
// edit; that review's hook runs the full gate. A tree the full gate already
// checked isn't checked again.
func StopChecks(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	// Already continuing because of this hook: one retry per failure, so a
	// check that can't be fixed never loops.
	if h.Payload.Bool("stop_hook_active") {
		return nil
	}
	deadline := time.Now().Add(stopBudget)
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
	if err := planGate(h, cwd, root, entries); err != nil {
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
	if coveredByFullGate(h.Paths.CacheDir(), root) {
		h.Notify("stop checks: skipped, the full gate already checked this tree")
		return nil
	}
	for i, path := range edited {
		edited[i] = fsx.Abs(cwd, path)
	}
	p := enforce.Stop(cfg, enforce.Options{Root: root, CacheDir: h.Paths.CacheDir()}, edited)
	if len(p.Gates) == 0 {
		return nil
	}
	out := run.Files(p, h.Paths.CacheDir(), time.Duration(cfg.GateTimeout)*time.Second, deadline)
	if !h.DryRun {
		_ = fsx.WriteAtomic(cache.StopReport(h.Paths.CacheDir(), root), []byte(out.Report()), 0o600)
	}
	// Every verdict goes to the user, a block's too; what to fix goes to
	// Claude in the block's reason.
	h.Notify(out.Notice)
	if out.Failed {
		return h.Block("file checks failed; fix them or report and stop.\n"+out.Failures, "checks-failed")
	}
	return nil
}

// coveredByFullGate is true when review-checks' last full gate checked the
// work tree as it is now.
func coveredByFullGate(cacheDir, root string) bool {
	checked, err := os.ReadFile(cache.FullGateTree(cacheDir, root))
	if err != nil {
		return false
	}
	tree, err := git.Snapshot(root)
	return err == nil && tree == string(checked)
}

// stopBudget bounds a Stop run, planning and every gate, inside
// stop-checks' 300s in hooks.json. The 60s left covers the snapshot and
// diffs after the gates (git calls of up to git.Timeout each) and the report.
const stopBudget = 240 * time.Second

// planGate blocks the stop when this turn ticked a plan's last step without
// a review since the last code edit.
func planGate(h *hook.Hook, cwd, root string, entries []transcript.Entry) error {
	dir := project.PlanDir(h.Paths, cwd)
	isCode := func(path string) bool {
		return fsx.Within(root, fsx.PhysicalPath(path)) && !project.IsScratch(h.Paths, path)
	}
	// A finished plan waits for /code-review since the last code edit (rules/verify.md).
	if plan, reviewed := planDone(entries, dir, isCode); plan != "" && !reviewed {
		return h.Block(fmt.Sprintf("plan %s is done, but /code-review hasn't finished since the last code edit. Run it now, or wait for the one running, then the runtime pass (/verify) when the change has observable behavior.", fsx.Rel(root, plan)), "plan-done")
	}
	return nil
}
