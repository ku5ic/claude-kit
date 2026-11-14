package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/connector"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
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
	if decision, _ := h.Decision(); decision == "deny" {
		return nil
	}
	b := cfg.SizeBudgets
	tool := p.String(hook.KeyToolName)
	var text, where, then, marker string
	var budget int
	switch {
	case p.String(hook.KeyHookEventName) == "SubagentStop":
		// Sent back once, by the stop or the hand-back. A marker, not
		// stop_hook_active: another check may have blocked the first stop.
		m := handbackMarker(h, cache.Size)
		if m == "" && p.Bool(hook.KeyStopHookActive) || m != "" && fsx.IsFile(m) {
			return nil
		}
		if n := countWords(p.String(hook.KeyLastAssistantMessage), false); b.Report > 0 && n > b.Report {
			if m != "" && !h.DryRun {
				_ = fsx.WriteAtomic(m, []byte("sent-back"), 0o644)
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
	n := countWords(text, false)
	if budget <= 0 || n <= budget || marker == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(text))
	sentBack := "sent-back " + hex.EncodeToString(sum[:])
	if prev, err := os.ReadFile(marker); err == nil && string(prev) == sentBack {
		return nil
	}
	if !markOnce(h, marker, []byte(sentBack), "size") {
		return nil
	}
	h.Deny(sizeMessage(where, n, budget, then), "size")
	return nil
}

// sizeMessage says how far over its budget a deliverable runs, then what to do.
func sizeMessage(where string, n, budget int, then string) string {
	return strings.Join([]string{
		fmt.Sprintf("%s runs %d words, over the %d-word budget", where, n, budget),
		layout.KV("do", "cut what doesn't serve the problem it answers; fenced code doesn't count"),
		layout.KV("then", then+", or send it unchanged to keep this length"),
	}, "\n")
}
