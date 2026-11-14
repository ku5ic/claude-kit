package hooks

import (
	"path/filepath"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// ReviewReport holds AskUserQuestion and Stop, a retried Stop too, until a
// finished /code-review's findings are a review-*.md scratch report
// written after them. A forked review's report reaches only Claude
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
	if root := git.Toplevel(cwd); stop && root != "" {
		// Stop shares stop-checks' saved walk, reading only new lines.
		_, walk = readSession(h, path, root, project.PlanDir(h.Paths, cwd), codeIn(h, root))
	} else {
		walk = newPlanWalk()
		noCode := func(string) bool { return false }
		if _, _, err := transcript.EachFrom(path, 0, 0, func(e transcript.Entry, _ int64) { walk.step(h, e, "", noCode) }); err != nil {
			return nil
		}
	}
	if walk.ReviewArrived <= walk.ReviewReported {
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

// isReviewReport is whether path is a review report in a scratch dir.
func isReviewReport(h *hook.Hook, path string) bool {
	base := filepath.Base(path)
	return project.IsScratch(h.Paths, path) && strings.HasPrefix(base, "review-") && strings.HasSuffix(base, ".md")
}
