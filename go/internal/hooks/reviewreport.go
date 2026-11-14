package hooks

import (
	"os"
	"strconv"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// ReviewReport holds the first AskUserQuestion or Stop after a finished
// /code-review, once, when no review-*.md scratch report was written after
// it. A forked review's report reaches only Claude
// (skills/kit-rules/reference/output.md section 3), so the user would
// otherwise decide on findings they never saw.
func ReviewReport(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	stop := h.Payload.String(hook.KeyHookEventName) == "Stop"
	path := h.Payload.String(hook.KeyTranscriptPath)
	if !stop && h.Payload.String(hook.KeyToolName) != "AskUserQuestion" || !fsx.IsFile(path) {
		return nil
	}
	var walk *planWalk
	cwd := h.Payload.Cwd()
	if root := git.Toplevel(cwd); root != "" {
		// stop-checks' saved walk, read only past where it stopped.
		_, walk = readSession(h, path, root, project.PlanDir(h.Paths, cwd), codeIn(h, root))
	} else {
		walk = newPlanWalk()
		noCode := func(string) bool { return false }
		if _, _, err := transcript.EachFrom(path, 0, 0, func(e transcript.Entry, _ int64) { walk.step(h, e, "", noCode) }); err != nil {
			return nil
		}
	}
	// One block per review: the marker holds the line of the review whose
	// block was shown, so a retried Stop or a later turn can't loop on it.
	marker := h.Paths.SessionFile(cache.ReviewReport, h.Payload.SessionID())
	arrived := strconv.Itoa(walk.ReviewArrived)
	if seen, _ := os.ReadFile(marker); walk.ReviewArrived <= walk.ReviewReported || string(seen) == arrived {
		return nil
	}
	if !markOnce(h, marker, []byte(arrived), "review-report") {
		return nil
	}
	reason := "a /code-review finished, and its findings reached only you\n" +
		layout.KV("do", "write them, in the report-format skill's shape, to the path `kit scratch-dir review <slug>` prints") + "\n" +
		layout.KV("then", "reply with its path, per skills/kit-rules/reference/output.md section 3, before asking about them")
	if stop {
		return h.Block(reason, "review-report")
	}
	h.Deny(reason, "review-report")
	return nil
}
