package hooks

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/connector"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/layout"
)

// CheckSize sends back a deliverable whose prose runs over its kind's
// kit.yml size_budgets entry, fenced code excluded, once per destination,
// so its rewrite goes through. It needs no model run, so it goes before
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
	if p.String(hook.KeyHookEventName) == "SubagentStop" {
		return sizeStop(h, cfg.SizeBudgets.Report)
	}
	t, ok := sizeTargetOf(h, cfg)
	switch {
	case !ok:
	case t.handback:
		if n := countWords(t.text, false); t.budget > 0 && n > allowance(h, t.budget) && t.marker != "" && !wasSentBack(t.marker) && markSentBack(h, t.marker, t.text, "size") {
			h.Deny(sizeMessage(t.where, n, allowance(h, t.budget), t.then), "size")
		}
	case t.document && sentBackBefore(t.marker):
	default:
		sizeOnce(h, t.text, t.where, t.then, t.budget, t.marker)
	}
	return nil
}

// sizeStop blocks a subagent's stop when its report runs over budget,
// once per agent, by the stop or the hand-back. A marker, not
// stop_hook_active: another check may have blocked the first stop.
func sizeStop(h *hook.Hook, budget int) error {
	p := h.Payload
	m := handbackMarker(h, cache.Size)
	report := p.String(hook.KeyLastAssistantMessage)
	if m == "" && p.Bool(hook.KeyStopHookActive) || m != "" && wasSentBack(m) {
		return nil
	}
	n := countWords(report, false)
	if budget <= 0 || n <= allowance(h, budget) || m != "" && !markSentBack(h, m, report, "size") {
		return nil
	}
	return h.Block(sizeMessage("your report", n, allowance(h, budget), "send the shorter report"), "size")
}

// sizeTarget is a deliverable CheckSize measures: its text, how the
// message names it and what to do next, its budget, and its send-back
// marker. A document, a plan or scratch report, is revised in place, so it
// goes back once per session; a post is a new text each time.
type sizeTarget struct {
	text, where, then, marker string
	budget                    int
	document, handback        bool
}

// sizeTargetOf is the deliverable the PreToolUse call sends, if CheckSize
// measures one.
func sizeTargetOf(h *hook.Hook, cfg *config.Config) (sizeTarget, bool) {
	p, b := h.Payload, cfg.SizeBudgets
	tool := p.String(hook.KeyToolName)
	switch {
	case tool == "SubagentHandback":
		return sizeTarget{text: p.String(hook.KeyInputMessage), where: "your report", then: "hand back the shorter report", budget: b.Report,
			marker: handbackMarker(h, cache.Size), handback: true}, true
	case tool == "ExitPlanMode":
		plan := filepath.Base(p.String(hook.KeyInputPlanFilePath))
		return sizeTarget{text: p.String(hook.KeyInputPlan), where: "the plan " + plan, then: "call ExitPlanMode again", budget: b.Plan,
			marker: h.Paths.SessionFile(cache.Size, p.SessionID(), "plan", plan), document: true}, true
	case slices.Contains(filePathTools, tool):
		return scratchSizeTarget(h, b.Scratch)
	case connector.IsWrite(cfg.Connectors, tool):
		return sizeTarget{text: strings.Join(p.Strings(hook.KeyToolInput, cfg.Connectors.CodeFields...), "\n\n"), where: "the " + tool + " input", then: "send it again", budget: b.Post,
			marker: h.Paths.SessionFile(cache.Size, p.SessionID(), "tool", tool)}, true
	}
	return sizeTarget{}, false
}

// scratchSizeTarget is an explore or debug report an edit tool writes in
// scratch.
func scratchSizeTarget(h *hook.Hook, budget int) (sizeTarget, bool) {
	p := h.Payload
	path := p.FilePath()
	if !scratchReport(h, path, "explore", "debug") {
		return sizeTarget{}, false
	}
	_, after, ok := written(p, path)
	if !ok {
		return sizeTarget{}, false
	}
	return sizeTarget{text: after, where: filepath.Base(path), then: "write the report again", budget: budget,
		marker: h.Paths.SessionFile(cache.Size, p.SessionID(), "file", filepath.Base(path)), document: true}, true
}

// allowance is budget plus reply_slack: how long a text may run before
// it's sent back.
func allowance(h *hook.Hook, budget int) int {
	if cfg := h.Config(); cfg != nil {
		return cfg.ReplyLimits.Allowance(budget)
	}
	return budget
}

// sizeOnce denies text when its prose runs past its allowance, once: the next
// text to marker, its rewrite, goes through whatever its length, and the
// one after is measured again.
func sizeOnce(h *hook.Hook, text, where, then string, budget int, marker string) {
	n := countWords(text, false)
	if budget <= 0 || marker == "" {
		return
	}
	if rewriteOf(marker) {
		return
	}
	if n > allowance(h, budget) && markOnce(h, marker, []byte(sentBackMark), "size") {
		h.Deny(sizeMessage(where, n, allowance(h, budget), then), "size")
	}
}

// wasSentBack is whether a subagent's size marker records a send-back.
func wasSentBack(marker string) bool {
	n, _ := sentBack(marker)
	return n > 0
}

// sizeMessage says how far over its allowance, the budget plus
// reply_slack, a deliverable runs, then what to do.
func sizeMessage(where string, n, limit int, then string) string {
	return strings.Join([]string{
		fmt.Sprintf("%s runs %d words, over the %d-word allowance", where, n, limit),
		layout.KV("do", "cut what doesn't serve the problem it answers; fenced code doesn't count"),
		layout.KV("then", then+", or send it unchanged to keep this length"),
	}, "\n")
}
