package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/slop"
)

// sendBack is the then line of a deny or block that returns a report for a
// rewrite.
const sendBack = "hand back your full report again, every finding unchanged but for the rewritten sentences"

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
	marker := handbackMarker(h, cache.Slop)
	if marker == "" {
		return nil
	}
	text := p.String(hook.KeyInputMessage)
	hits := reportHits(h, text)
	if !mayResend(marker, text) {
		return nil
	}
	if len(hits) == 0 {
		// A clean report marks the hand-back without resetting the count.
		if !h.DryRun && !fsx.IsFile(marker) {
			_ = fsx.WriteAtomic(marker, []byte("seen"), 0o644)
		}
		return nil
	}
	if markSentBack(h, marker, text, "slop") {
		h.Deny(slop.Message("your report", hits, sendBack), "slop")
	}
	return nil
}

// SubagentSlop sends a subagent's closing message back when it holds AI
// tells in a slop.block_tiers tier: without auto mode, that message is the
// report. A revision is checked again, up to maxSendBacks; an agent that
// handed back a clean report is GuardHandback's. A marker, not
// stop_hook_active: another check may have blocked the first stop.
// SubagentStop.
func SubagentSlop(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return nil
	}
	marker := handbackMarker(h, cache.Slop)
	text := p.String(hook.KeyLastAssistantMessage)
	if prev, _ := os.ReadFile(marker); marker == "" && p.Bool(hook.KeyStopHookActive) || string(prev) == "seen" || marker != "" && !mayResend(marker, text) {
		return nil
	}
	hits := reportHits(h, text)
	if len(hits) == 0 {
		return nil
	}
	if marker != "" && !markSentBack(h, marker, text, "slop") {
		return nil
	}
	return h.Block(slop.Message("your report", hits, sendBack), "slop")
}

// handbackMarker is the session's marker, in cache dir kind, for the
// payload's agent; "" when either id is missing or isn't a plain name.
func handbackMarker(h *hook.Hook, kind string) string {
	agent := h.Payload.String(hook.KeyAgentID)
	if agent == "" || strings.ContainsAny(agent, `/\`) {
		return ""
	}
	return h.Paths.SessionFile(kind, h.Payload.SessionID(), agent, "handback")
}

// markOnce writes data to marker, the guard that shows a block or deny for
// rule only once. False when the write fails: with no guard, the block
// would come back on every retry, so the caller lets the call through and
// the user is told.
// maxSendBacks is how many times a cheap check (slop, size, citations)
// sends one agent's report back before it lets the report through: a
// revision is checked again, and the cap ends a loop.
const maxSendBacks = 2

// sentBack is what a handback marker records: how many times the agent's
// report was sent back, and the hash of the last text sent back. A missing
// marker, and "seen" (a clean hand-back), are none; a bare "sent-back",
// from before the count, is one.
func sentBack(marker string) (n int, last string) {
	data, err := os.ReadFile(marker)
	if err != nil {
		return 0, ""
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 || fields[0] != sentBackMark {
		return 0, ""
	}
	if len(fields) < 3 {
		return 1, ""
	}
	n, _ = strconv.Atoi(fields[1])
	return n, fields[2]
}

// mayResend is true when the report text is checked again: it isn't the
// text last sent back, and the agent hasn't reached maxSendBacks.
func mayResend(marker, text string) bool {
	n, last := sentBack(marker)
	return n < maxSendBacks && last != textHash(text)
}

// markSentBack records one more send-back of text, as markOnce does.
func markSentBack(h *hook.Hook, marker, text, rule string) bool {
	n, _ := sentBack(marker)
	return markOnce(h, marker, []byte(fmt.Sprintf("%s %d %s", sentBackMark, n+1, textHash(text))), rule)
}

// textHash is text's sha256, in hex.
func textHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func markOnce(h *hook.Hook, marker string, data []byte, rule string) bool {
	if h.DryRun {
		return true
	}
	if err := fsx.WriteAtomic(marker, data, 0o600); err != nil {
		h.Notify("kit " + rule + ": let through unchecked, its once-only marker can't be saved: " + err.Error())
		return false
	}
	return true
}

// sentBackMark is what a once-only marker holds after its text was sent back.
const sentBackMark = "sent-back"

// rewriteOf is whether marker holds a send-back, so the text now is its
// rewrite and goes through; the marker is cleared, so the next is checked.
func rewriteOf(marker string) bool {
	if !fsx.IsFile(marker) {
		return false
	}
	_ = os.Remove(marker)
	return true
}

// sentBackBefore is whether marker records a send-back of a document, a plan
// or a scratch report revised in place. The marker stays, so every later
// revision goes through: one document goes back once per session, however
// many edits it takes, and an edit the user refused spends nothing.
func sentBackBefore(marker string) bool { return fsx.IsFile(marker) }

// reportHits scans a subagent report and returns the hits in a block tier,
// logging every hit. Without kit.yml it finds none.
func reportHits(h *hook.Hook, text string) []slop.Hit {
	cfg := h.Config()
	if cfg == nil || strings.TrimSpace(text) == "" {
		return nil
	}
	_, blocking := slop.New(cfg).Check(slop.Prose(text), "report", logHit(h, "subagent report"))
	return blocking
}

// logHit logs a slop hit in where to the guards log.
func logHit(h *hook.Hook, where string) func(slop.Hit) {
	return func(hit slop.Hit) { h.Log(kitlog.Guards, "slop", slop.LogPairs(hit, where)...) }
}
