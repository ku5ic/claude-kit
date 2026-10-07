package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// StopChecks is the Stop hook: when the turn created or edited files through
// Edit/Write/MultiEdit/NotebookEdit and left the tree dirty, it runs
// kit.yml's file_checks on just those files and blocks the stop on a
// failure, so Claude fixes or reports it. A question-only turn costs
// nothing. When the turn ticked a plan's last open step, it also blocks
// until /code-review (which runs the full suite) has run since the last code
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
	cwd := cwdOf(h)
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = cwd
		out, err := cmd.Output()
		return string(out), err
	}
	if out, err := git("rev-parse", "--is-inside-work-tree"); err != nil || strings.TrimSpace(out) != "true" {
		return nil
	}
	transcript := h.Payload.String("transcript_path")
	if f, err := os.Open(transcript); err != nil {
		return nil
	} else {
		f.Close()
	}
	top, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return nil
	}
	root := project.PhysicalPath(strings.TrimSpace(top))
	cfg := h.Config()
	if cfg == nil {
		return nil
	}

	// Before the clean-tree return: plans are gitignored, so ticking the
	// last step leaves the tree clean.
	if dir, err := project.Dir(cfg, h.Paths, cwd, "plans", false); err == nil {
		isCode := func(path string) bool {
			return strings.HasPrefix(project.PhysicalPath(path), root+"/") && !project.IsScratch(h.Paths, path)
		}
		if plan, reviewed := planDone(transcript, dir, isCode); plan != "" {
			if err := endOfPlan(h, strings.TrimPrefix(plan, root+"/"), reviewed); err != nil {
				return err
			}
		}
	}

	// A clean tree means the edits were committed, which already went
	// through verification.
	if status, err := git("status", "--porcelain"); err != nil || status == "" {
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

// endOfPlan blocks a finished plan's stop until /code-review, which runs
// the full suite, has run since the last code edit (rules/verify.md).
func endOfPlan(h *hook.Hook, plan string, reviewed bool) error {
	if reviewed {
		return nil
	}
	return h.Block(fmt.Sprintf("plan %s is done, but /code-review hasn't run since the last code edit. Run it now (it runs the full suite), then the runtime pass (/verify) when the change has observable behavior.", plan), "plan-done")
}
