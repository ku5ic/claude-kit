package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/cite"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
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
//     report, once per agent.
//
// It runs before verify-claims in its dispatcher, so a broken citation is
// fixed before a model checks the claims around it.
func CheckCitations(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	if decision, _ := h.Decision(); decision == "deny" {
		return nil
	}
	root, anchored := project.Root(p.Cwd())
	if !anchored {
		return nil
	}
	switch {
	case p.String("hook_event_name") == "SubagentStop":
		if !isGatherer(p) || p.Bool("stop_hook_active") {
			return nil
		}
		if marker := handbackMarker(h, cache.Cites); marker != "" && fsx.IsFile(marker) {
			return nil
		}
		if bad := cite.Broken(root, p.String("last_assistant_message")); len(bad) > 0 {
			return h.Block(citesMessage("your report", bad, sendBack), "citations")
		}
	case p.String("tool_name") == "SubagentHandback":
		marker := handbackMarker(h, cache.Cites)
		if !isGatherer(p) || marker == "" || fsx.IsFile(marker) {
			return nil
		}
		if bad := cite.Broken(root, p.String("tool_input.message")); len(bad) > 0 {
			if !h.DryRun {
				_ = fsx.WriteAtomic(marker, []byte("sent-back"), 0o644)
			}
			h.Deny(citesMessage("your report", bad, sendBack), "citations")
		}
	case p.String("tool_name") == "ExitPlanMode":
		return planCitations(h, root)
	}
	return nil
}

// planCitations denies a plan with broken citations once per text.
func planCitations(h *hook.Hook, root string) error {
	p := h.Payload
	text := p.String("tool_input.plan")
	bad := cite.Broken(root, text)
	if len(bad) == 0 {
		return nil
	}
	plan := filepath.Base(p.String("tool_input.planFilePath"))
	marker := h.Paths.SessionFile(cache.Cites, p.SessionID(), "plan", plan)
	sum := sha256.Sum256([]byte(text))
	sentBack := "sent-back " + hex.EncodeToString(sum[:])
	if prev, err := os.ReadFile(marker); err == nil && string(prev) == sentBack {
		return nil
	}
	if !h.DryRun {
		_ = fsx.WriteAtomic(marker, []byte(sentBack), 0o644)
	}
	h.Deny(citesMessage("the plan "+plan, bad, "call ExitPlanMode again"), "citations")
	return nil
}

// isGatherer is true in a kit:gatherer run. Claude Code's agent_type for a
// plugin's agent is plugin-scoped, kit:gatherer
// (code.claude.com/docs/en/hooks); a bare name is matched too.
func isGatherer(p *hook.Payload) bool {
	return strings.TrimPrefix(p.String("agent_type"), "kit:") == gatherer
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
