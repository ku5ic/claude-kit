package hooks

import (
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

// LogDenied is the PermissionDenied hook: it logs the tool whose call the
// auto mode classifier denied, for kit stats. It never answers, since a
// retry would second-guess the classifier.
func LogDenied(h *hook.Hook) error {
	p := h.Payload
	if tool := p.String(hook.KeyToolName); p.Err == nil && tool != "" {
		h.Log(kitlog.Guards, kitlog.EventAutoDenied, "tool", tool, "agent_type", p.String(hook.KeyAgentType))
	}
	return nil
}
