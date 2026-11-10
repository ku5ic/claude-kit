package hooks

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/weaken"
)

// GuardWeaken blocks an edit that skips or focuses a test, or silences a
// linter or type checker, unless the line carries `kit: allow <reason>`,
// which is logged. A test file left with fewer assertions asks first.
// PreToolUse for Edit, Write, and MultiEdit, inside guard-dispatch.
func GuardWeaken(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	path, cfg := h.Payload.FilePath(), h.Config()
	if path == "" || cfg == nil {
		return nil
	}
	before, after, ok := written(h.Payload, path)
	if !ok {
		return nil
	}
	r := weaken.Check(cfg.WeakenedChecks, path, before, after)
	var b strings.Builder
	for _, hit := range r.Hits {
		if hit.Reason != "" {
			h.Log(kitlog.Guards, "weaken-allow", "path", path, "line", strconv.Itoa(hit.Line), "reason", hit.Reason)
			continue
		}
		fmt.Fprintf(&b, "- L%d %s: %s\n", hit.Line, hit.Kind, hit.Text)
	}
	if b.Len() > 0 {
		return h.Block(fmt.Sprintf("this edit weakens a check in %s:\n%sSay why in your reply and put `kit: allow <reason>` on the line or above it, or revert it.", filepath.Base(path), b.String()), "weaken")
	}
	if r.Lost > 0 {
		h.Decide("ask", fmt.Sprintf("this edit leaves %s with %d fewer assertion line(s). Confirm only if they're redundant or moved", filepath.Base(path), r.Lost))
	}
	return nil
}
