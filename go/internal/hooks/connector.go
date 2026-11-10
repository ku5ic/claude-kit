package hooks

import (
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/connector"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/slop"
)

// GuardConnector blocks an MCP tool that sends text out of the session (a
// message, a comment, a page) when that text holds AI tells in a
// slop.block_tiers tier, as guard-pr does for gh. Every string in the
// tool's input is scanned as one text on the pr surface; a tool that only
// reads goes through untouched.
// PreToolUse for mcp__.*.
func GuardConnector(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	cfg := h.Config()
	tool := p.String("tool_name")
	if cfg == nil || !connector.IsWrite(cfg.Connectors, tool) {
		return nil
	}
	text := strings.Join(p.Strings("tool_input", cfg.Connectors.CodeFields...), "\n\n")
	if strings.TrimSpace(text) == "" {
		return nil
	}
	_, hits := slop.New(cfg).Check(slop.Prose(text), "pr", logHit(h, tool))
	if len(hits) == 0 {
		return nil
	}
	return h.Block(slop.Message("the "+tool+" input", hits), "slop")
}
