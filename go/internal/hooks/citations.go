package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/cite"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// gatherer is the agent whose reports are facts with file:line citations.
const gatherer = "gatherer"

// CheckCitations sends back a plan or a gatherer report that cites a
// file:line that doesn't resolve in the project, with no model run.
//   - PreToolUse for ExitPlanMode: the plan, once per text, so the same
//     plan resent goes through.
//   - PreToolUse for SubagentHandback, and SubagentStop: a kit:gatherer
//     report, a revision again, up to maxSendBacks per agent.
//
// It runs before verify-claims in its dispatcher, so a broken citation is
// fixed before a model checks the claims around it.
func CheckCitations(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	if earlierDeny(h) {
		return nil
	}
	root, anchored := project.Root(p.Cwd())
	if !anchored {
		return nil
	}
	switch {
	case p.String(hook.KeyHookEventName) == "SubagentStop":
		return stopCitations(h, root)
	case p.String(hook.KeyToolName) == "SubagentHandback":
		handbackCitations(h, root)
	case p.String(hook.KeyToolName) == "ExitPlanMode":
		return planCitations(h, root)
	}
	return nil
}

// stopCitations blocks a gatherer's stop on broken citations: sent back by
// the stop or the hand-back, a revision again, up to maxSendBacks. A
// marker, not stop_hook_active: another check may have blocked the first
// stop.
func stopCitations(h *hook.Hook, root string) error {
	p := h.Payload
	marker := handbackMarker(h, cache.Cites)
	text := p.String(hook.KeyLastAssistantMessage)
	if !isGatherer(p) || marker == "" && p.Bool(hook.KeyStopHookActive) || marker != "" && !mayResend(marker, text) {
		return nil
	}
	bad := cite.Broken(root, text)
	if len(bad) == 0 || marker != "" && !markSentBack(h, marker, text, "citations") {
		return nil
	}
	return h.Block(citesMessage("your report", bad, sendBack), "citations")
}

// handbackCitations denies a gatherer's hand-back on broken citations.
func handbackCitations(h *hook.Hook, root string) {
	p := h.Payload
	marker := handbackMarker(h, cache.Cites)
	text := p.String(hook.KeyInputMessage)
	if !isGatherer(p) || marker == "" || !mayResend(marker, text) {
		return
	}
	if bad := cite.Broken(root, text); len(bad) > 0 && markSentBack(h, marker, text, "citations") {
		h.Deny(citesMessage("your report", bad, sendBack), "citations")
	}
}

// planCitations denies a plan with broken citations once per text.
func planCitations(h *hook.Hook, root string) error {
	p := h.Payload
	text := p.String(hook.KeyInputPlan)
	bad := cite.Broken(root, text)
	if len(bad) == 0 {
		return nil
	}
	plan := filepath.Base(p.String(hook.KeyInputPlanFilePath))
	marker := h.Paths.SessionFile(cache.Cites, p.SessionID(), "plan", plan)
	sentBack := "sent-back " + textHash(text)
	if prev, err := os.ReadFile(marker); err == nil && string(prev) == sentBack {
		return nil
	}
	if !markOnce(h, marker, []byte(sentBack), "citations") {
		return nil
	}
	h.Deny(citesMessage("the plan "+plan, bad, "call ExitPlanMode again"), "citations")
	return nil
}

// isGatherer is true in a kit:gatherer run. Claude Code's agent_type for a
// plugin's agent is plugin-scoped, kit:gatherer
// (code.claude.com/docs/en/hooks); a bare name is matched too.
func isGatherer(p *hook.Payload) bool {
	return strings.TrimPrefix(p.String(hook.KeyAgentType), "kit:") == gatherer
}

// citesMessage names each citation that doesn't resolve and why, at most
// eight, then what to do.
func citesMessage(where string, bad []cite.Problem, then string) string {
	lines := []string{fmt.Sprintf("%d citation(s) in %s that don't resolve", len(bad), where)}
	for i, b := range bad {
		if i == 8 {
			lines = append(lines, layout.KV("more", fmt.Sprintf("%d more", len(bad)-i)))
			break
		}
		lines = append(lines, layout.KV(b.Cite, b.Reason))
	}
	lines = append(lines, layout.KV("do", "point each at a file:line that exists, or drop it"), layout.KV("then", then))
	return strings.Join(lines, "\n")
}
