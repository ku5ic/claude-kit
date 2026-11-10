package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claims"
	"github.com/ku5ic/claude-kit/go/internal/connector"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// VerifyClaims sends text back when the claim verifier finds a claim in it
// that nothing in the session confirms, so each is checked or labeled
// before anyone relies on it. A chat reply is never checked; see checkClaims.
//   - SubagentStop, and PreToolUse for SubagentHandback: a subagent's
//     report, against its own transcript, before the parent reads it.
//   - PreToolUse for a Write or Edit of an explore or debug report in
//     scratch: the report, before it's written.
//   - PreToolUse for an MCP tool that writes: the post, before it's sent.
//
// Each text is sent back once, and resending it goes through. A handback
// is checked once per agent.
func VerifyClaims(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return nil
	}
	switch p.String("hook_event_name") {
	case "SubagentStop":
		marker := handbackMarker(h, cache.Claims)
		if p.Bool("stop_hook_active") || marker != "" && fsx.IsFile(marker) {
			return nil
		}
		if u := unconfirmed(h, p.String("last_assistant_message"), agentTranscript(p), "subagent report"); len(u) > 0 {
			return h.Block(claimsMessage("your report", u)+sendBack, "claims")
		}
	case "PreToolUse":
		return verifyToolInput(h)
	}
	return nil
}

// verifyToolInput checks a handback, a scratch report, or a connector post.
func verifyToolInput(h *hook.Hook) error {
	p := h.Payload
	tool := p.String("tool_name")
	var text, where, transcriptPath, marker, retry string
	switch {
	case tool == "SubagentHandback":
		text, where, transcriptPath, marker = p.String("tool_input.message"), "your report", agentTranscript(p), handbackMarker(h, cache.Claims)
		retry = sendBack
	case slices.Contains(filePathTools, tool):
		path := p.FilePath()
		if !isInvestigationReport(h, path) {
			return nil
		}
		_, after, ok := written(p, path)
		if !ok {
			return nil
		}
		text, where, transcriptPath = after, filepath.Base(path), agentTranscript(p)
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "file", filepath.Base(path))
		retry = " Then write the report again."
	default:
		cfg := h.Config()
		if cfg == nil || !connector.IsWrite(cfg.Connectors, tool) {
			return nil
		}
		text, where, transcriptPath = strings.Join(p.Strings("tool_input", cfg.Connectors.CodeFields...), "\n\n"), "the "+tool+" input", p.String("transcript_path")
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "tool", tool)
		retry = " Then send it again."
	}
	if marker == "" {
		return nil
	}
	// A handback's marker is per agent and stays, so SubagentStop knows the
	// report was checked. A post's or report's names the text sent back, so
	// only that text, resent, goes through.
	handback := tool == "SubagentHandback"
	sentBack := "sent-back"
	if !handback {
		sum := sha256.Sum256([]byte(text))
		sentBack += " " + hex.EncodeToString(sum[:])
	}
	if prev, err := os.ReadFile(marker); err == nil && string(prev) == sentBack {
		if !handback {
			_ = os.Remove(marker)
		}
		return nil
	}
	u := unconfirmed(h, text, transcriptPath, where)
	state := "seen"
	if len(u) > 0 {
		state = sentBack
	}
	if (handback || len(u) > 0) && !h.DryRun {
		_ = fsx.WriteAtomic(marker, []byte(state), 0o644)
	}
	if len(u) > 0 {
		h.Deny(claimsMessage(where, u)+retry, "claims")
	}
	return nil
}

// unconfirmed is what the verifier found unconfirmed in text, logged; nil
// when text states nothing, without kit.yml, or when the verifier fails,
// which never blocks.
func unconfirmed(h *hook.Hook, text, transcriptPath, where string) []claims.Unsupported {
	cfg := h.Config()
	if cfg == nil || h.DryRun || h.RuleDisabled("claims") || !claims.Declarative(text) {
		return nil
	}
	v, err := claims.Verify(cfg, h.Payload.Cwd(), text, transcriptPath)
	if err != nil {
		h.Log(kitlog.Guards, "claims-error", "where", where, "error", err.Error())
		return nil
	}
	h.Log(kitlog.Guards, "claims", "where", where, "unconfirmed", strconv.Itoa(len(v.Unsupported)))
	return v.Unsupported
}

// claimsMessage names each unconfirmed claim and the check that would settle it.
func claimsMessage(where string, u []claims.Unsupported) string {
	var b strings.Builder
	fmt.Fprintf(&b, "claims: %d claim(s) in %s that nothing in the session confirms:\n", len(u), where)
	for _, c := range u {
		fmt.Fprintf(&b, "- %q -> %s\n", c.Claim, c.Check)
	}
	b.WriteString("Run each check, or label the claim (likely, not verified, unverifiable) per rules/evidence.md section 1.")
	return b.String()
}

// agentTranscript is a subagent's own transcript, else the session's.
func agentTranscript(p *hook.Payload) string {
	if path := p.String("agent_transcript_path"); path != "" {
		return path
	}
	return p.String("transcript_path")
}

// isInvestigationReport is true for an explore or debug report in scratch,
// the files `kit scratch-dir explore|debug <slug>` names.
func isInvestigationReport(h *hook.Hook, path string) bool {
	base := filepath.Base(path)
	return path != "" && project.IsScratch(h.Paths, path) && strings.EqualFold(filepath.Ext(base), ".md") &&
		(strings.HasPrefix(base, "explore-") || strings.HasPrefix(base, "debug-"))
}
