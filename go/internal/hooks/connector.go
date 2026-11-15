package hooks

import (
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/attribution"
	"github.com/ku5ic/claude-kit/go/internal/connector"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/slop"
)

// GuardConnector blocks an MCP tool that sends text out of the session (a
// message, a comment, a page) when that text holds AI tells in a
// slop.block_tiers tier, as guard-pr does for gh. Every string in the
// tool's input but connectors.code_fields is scanned as one text on the pr
// surface; a tool that only reads goes through untouched.
// A GitHub server's tool that doesn't only read blocks first on an AI
// tool's attribution in any string of its input (kit.yml attribution).
// PreToolUse for mcp__.*.
func GuardConnector(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	cfg := h.Config()
	if cfg == nil {
		return nil
	}
	tool := p.String(hook.KeyToolName)
	if strings.Contains(strings.ToLower(connector.Server(tool)), "github") && !connector.IsRead(cfg.Connectors, tool) {
		if line, ok := attribution.New(cfg.Attribution).Find(strings.Join(p.Strings(hook.KeyToolInput), "\n")); ok {
			return h.Block(attribution.Message("the "+tool+" input", line), "ai-attribution")
		}
	}
	if !connector.IsWrite(cfg.Connectors, tool) {
		return nil
	}
	text := strings.Join(p.Strings(hook.KeyToolInput, cfg.Connectors.CodeFields...), "\n\n")
	if strings.TrimSpace(text) == "" {
		return nil
	}
	_, hits := slop.New(cfg).Check(slop.Prose(text), "pr", logHit(h, tool))
	if len(hits) == 0 {
		return nil
	}
	return h.Block(slop.Message("the "+tool+" input", hits, ""), "slop")
}
