package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/connector"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/layout"
)

// CheckSize sends back a deliverable whose prose runs over its kind's
// kit.yml size_budgets entry, fenced code excluded, once per text, so the
// same text resent goes through. It needs no model run, so it goes before
// verify-claims in each dispatcher. It measures:
//   - the plan, at PreToolUse for ExitPlanMode
//   - a subagent's report, at PreToolUse for SubagentHandback and at SubagentStop
//   - an explore or debug report in scratch, at PreToolUse for Edit, Write, and MultiEdit
//   - the post, at PreToolUse for a connector tool that writes
func CheckSize(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	cfg := h.Config()
	if cfg == nil {
		return nil
	}
	if earlierDeny(h) {
		return nil
	}
	b := cfg.SizeBudgets
	tool := p.String(hook.KeyToolName)
	var text, where, then, marker string
	var budget int
	switch {
	case p.String(hook.KeyHookEventName) == "SubagentStop":
		// Sent back by the stop or the hand-back, a revision again, up to
		// maxSendBacks. A marker, not stop_hook_active: another check may
		// have blocked the first stop.
		m := handbackMarker(h, cache.Size)
		report := p.String(hook.KeyLastAssistantMessage)
		if m == "" && p.Bool(hook.KeyStopHookActive) || m != "" && !mayResend(m, report) {
			return nil
		}
		if n := countWords(report, false); b.Report > 0 && n > b.Report {
			if m != "" && !markSentBack(h, m, report, "size") {
				return nil
			}
			return h.Block(sizeMessage("your report", n, b.Report, "send the shorter report"), "size")
		}
		return nil
	case tool == "SubagentHandback":
		text, where, then, budget = p.String(hook.KeyInputMessage), "your report", "hand back the shorter report", b.Report
		marker = handbackMarker(h, cache.Size)
	case tool == "ExitPlanMode":
		plan := filepath.Base(p.String(hook.KeyInputPlanFilePath))
		text, where, then, budget = p.String(hook.KeyInputPlan), "the plan "+plan, "call ExitPlanMode again", b.Plan
		marker = h.Paths.SessionFile(cache.Size, p.SessionID(), "plan", plan)
	case slices.Contains(filePathTools, tool):
		path := p.FilePath()
		if !scratchReport(h, path, "explore", "debug") {
			return nil
		}
		_, after, ok := written(p, path)
		if !ok {
			return nil
		}
		text, where, then, budget = after, filepath.Base(path), "write the report again", b.Scratch
		marker = h.Paths.SessionFile(cache.Size, p.SessionID(), "file", filepath.Base(path))
	case connector.IsWrite(cfg.Connectors, tool):
		text, where, then, budget = strings.Join(p.Strings(hook.KeyToolInput, cfg.Connectors.CodeFields...), "\n\n"), "the "+tool+" input", "send it again", b.Post
		marker = h.Paths.SessionFile(cache.Size, p.SessionID(), "tool", tool)
	default:
		return nil
	}
	if tool == "SubagentHandback" {
		if n := countWords(text, false); budget > 0 && n > budget && marker != "" && mayResend(marker, text) && markSentBack(h, marker, text, "size") {
			h.Deny(sizeMessage(where, n, budget, then), "size")
		}
		return nil
	}
	sizeOnce(h, text, where, then, budget, marker)
	return nil
}

// sizeOnce denies text when its prose runs over budget, once per text: the
// same text resent goes through.
func sizeOnce(h *hook.Hook, text, where, then string, budget int, marker string) {
	n := countWords(text, false)
	if budget <= 0 || n <= budget || marker == "" {
		return
	}
	sentBack := "sent-back " + textHash(text)
	if prev, err := os.ReadFile(marker); err == nil && string(prev) == sentBack {
		return
	}
	if markOnce(h, marker, []byte(sentBack), "size") {
		h.Deny(sizeMessage(where, n, budget, then), "size")
	}
}

// sizeMessage says how far over its budget a deliverable runs, then what to do.
func sizeMessage(where string, n, budget int, then string) string {
	return strings.Join([]string{
		fmt.Sprintf("%s runs %d words, over the %d-word budget", where, n, budget),
		layout.KV("do", "cut what doesn't serve the problem it answers; fenced code doesn't count"),
		layout.KV("then", then+", or send it unchanged to keep this length"),
	}, "\n")
}
