package hooks

import (
	"os"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/slop"
)

// sendBack closes the deny or block that returns a report for a rewrite.
const sendBack = " Then hand back your full report again, every finding unchanged but for the rewritten sentences."

// GuardHandback sends a subagent's report back once when it holds AI tells
// in a slop.block_tiers tier. In auto mode the report is the message of the
// SubagentHandback tool, which a deny returns to the subagent; its marker
// says the agent handed back, so SubagentSlop leaves its closing text
// alone, and whether it was sent back, so the second handback goes through.
// PreToolUse for SubagentHandback.
func GuardHandback(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	marker := handbackMarker(h)
	if marker == "" {
		return nil
	}
	hits, err := reportHits(h, p.String("tool_input.message"))
	if err != nil {
		return err
	}
	if prev, _ := os.ReadFile(marker); string(prev) == "sent-back" {
		return nil
	}
	state := "seen"
	if len(hits) > 0 {
		state = "sent-back"
		h.Deny(slop.Message("your report", hits)+sendBack, "slop")
	}
	if !h.DryRun {
		_ = fsx.WriteAtomic(marker, []byte(state), 0o644)
	}
	return nil
}

// SubagentSlop sends a subagent's closing message back once when it holds
// AI tells in a slop.block_tiers tier: without auto mode, that message is
// the report. stop_hook_active lets the second stop through, and an agent
// that handed back is GuardHandback's.
// SubagentStop.
func SubagentSlop(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil || p.Bool("stop_hook_active") {
		return nil
	}
	if marker := handbackMarker(h); marker != "" && fsx.IsFile(marker) {
		return nil
	}
	hits, err := reportHits(h, p.String("last_assistant_message"))
	if err != nil || len(hits) == 0 {
		return err
	}
	return h.Block(slop.Message("your report", hits)+sendBack, "slop")
}

// handbackMarker is the session's marker for the payload's agent; "" when
// either id is missing or isn't a plain name.
func handbackMarker(h *hook.Hook) string {
	agent := h.Payload.String("agent_id")
	if agent == "" || strings.ContainsAny(agent, `/\`) {
		return ""
	}
	return h.Paths.SessionFile(cache.Slop, h.Payload.SessionID(), agent, "handback")
}

// reportHits scans a subagent report and returns the hits in a block tier,
// logging the rest. Without kit.yml it finds none.
func reportHits(h *hook.Hook, text string) ([]slop.Hit, error) {
	cfg := h.Config()
	if cfg == nil || strings.TrimSpace(text) == "" {
		return nil, nil
	}
	d, err := slop.New(cfg)
	if err != nil {
		return nil, err
	}
	segs, _ := slop.Prose(text)
	res := d.Scan(segs, "report")
	for _, hit := range d.Unblocked(res) {
		h.Log(kitlog.Guards, "slop", slop.LogPairs(hit, "subagent report")...)
	}
	return d.Blocking(res), nil
}
