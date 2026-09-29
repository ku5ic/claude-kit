package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	"github.com/ku5ic/dotfiles/claude-kit/go/internal/checks"
	"github.com/ku5ic/dotfiles/claude-kit/go/internal/hook"
	"github.com/ku5ic/dotfiles/claude-kit/go/internal/project"
)

// StopChecks is the Stop hook: when the turn created or edited files through
// Edit/Write/MultiEdit/NotebookEdit and left the tree dirty, it runs
// kit.yml's file_checks on just those files and blocks the stop on a
// failure, so Claude fixes or reports it. A question-only turn costs
// nothing. Whole-project checks (run-checks) are left to /code-review.
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
	// A clean tree means the edits were committed, which already went
	// through verification.
	if status, err := git("status", "--porcelain"); err != nil || status == "" {
		return nil
	}
	edited, err := checks.EditedFiles(transcript)
	if err != nil || len(edited) == 0 {
		return nil
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

	report, failures, summary, failed, ran := checks.FileChecks(cfg, root, cwd, edited)
	if !ran {
		return nil
	}
	if failed {
		if err := h.Block("file checks failed; fix them or report and stop.\n"+failures+summary, "checks-failed"); err != nil {
			return err
		}
	}
	// Pretty-printed, as `jq -n` printed it.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]string{"systemMessage": h.Name + ": " + report + summary})
	h.Stdout.Write(buf.Bytes())
	return nil
}
