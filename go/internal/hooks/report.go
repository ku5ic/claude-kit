package hooks

import (
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/run"
)

// ReportChecks shows the user every verdict of a kit run-checks run a Bash
// call printed, whoever ran it, so a reply never stands between the user
// and the result. The summary line marks a run, as in ranChecks: a plan
// prints none. The command must name run-checks too, so kit explain stop,
// which reprints a Stop run the user already saw, stays quiet.
func ReportChecks(h *hook.Hook) error {
	if !strings.Contains(h.Payload.String("tool_input.command"), "run-checks") {
		return nil
	}
	if notice := run.Notice(h.Payload.String("tool_response.stdout")); notice != "" {
		h.Notify("run-checks " + notice)
	}
	return nil
}
