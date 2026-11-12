package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claims"
	"github.com/ku5ic/claude-kit/go/internal/connector"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// VerifyClaims holds a text until its claims were checked, so each is
// checked or labeled before anyone relies on it. A chat reply is never
// checked; see checkClaims.
//   - SubagentStop, and PreToolUse for SubagentHandback: a subagent's
//     report, by the hook's own run against its transcript, once per agent.
//   - PreToolUse for ExitPlanMode: the plan, which plan-critic's run covers,
//     or with plan-critic off, the claim-checker agent.
//   - PreToolUse for a Write or Edit of an explore or debug report in
//     scratch, and for an MCP tool that writes: the claim-checker agent on
//     that exact text. Inside a subagent, the hook's own run, once per text.
//
// It runs last in its dispatcher and skips a call an earlier check denied,
// so no run is wasted.
func VerifyClaims(h *hook.Hook) error {
	p := h.Payload
	if decision, _ := h.Decision(); p.Err != nil || decision == "deny" {
		return nil
	}
	switch p.String("hook_event_name") {
	case "SubagentStop":
		// The marker, not stop_hook_active, ends the retry: a stop that
		// follows subagent-slop's block is this check's first.
		marker := handbackMarker(h, cache.Claims)
		if marker == "" && p.Bool("stop_hook_active") || marker != "" && fsx.IsFile(marker) {
			return nil
		}
		u := unconfirmed(h, p.String("last_assistant_message"), agentTranscript(p), "subagent report")
		if marker != "" && !h.DryRun {
			_ = fsx.WriteAtomic(marker, []byte("seen"), 0o644)
		}
		if len(u) > 0 {
			return h.Block(claimsMessage("your report", u, sendBack), "claims")
		}
	case "PreToolUse":
		return verifyToolInput(h)
	}
	return nil
}

// verifyToolInput checks a handback with the hook's own run, and gates a
// plan, a scratch report, or a connector post on the claim-checker agent.
func verifyToolInput(h *hook.Hook) error {
	p := h.Payload
	tool := p.String("tool_name")
	var text, where, retry, marker string
	switch {
	case tool == "SubagentHandback":
		return verifyHandback(h)
	case tool == "ExitPlanMode":
		// plan-critic checks a plan's claims in the run GuardPlan asks for.
		// A plan resent without that run gets its claims checked here.
		if !h.RuleDisabled("plan-critic") && planCritiqued(p) {
			return nil
		}
		// Claude Code injects the plan's text and path (code.claude.com/docs/en/hooks).
		plan := filepath.Base(p.String("tool_input.planFilePath"))
		text, where, retry = p.String("tool_input.plan"), "the plan "+plan, "call ExitPlanMode again"
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "plan", plan)
	case slices.Contains(filePathTools, tool):
		path := p.FilePath()
		if !isInvestigationReport(h, path) {
			return nil
		}
		_, after, ok := written(p, path)
		if !ok {
			return nil
		}
		text, where, retry = after, filepath.Base(path), "write the report again"
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "file", filepath.Base(path))
	default:
		cfg := h.Config()
		if cfg == nil || !connector.IsWrite(cfg.Connectors, tool) {
			return nil
		}
		text, where, retry = strings.Join(p.Strings("tool_input", cfg.Connectors.CodeFields...), "\n\n"), "the "+tool+" input", "send it again"
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "tool", tool)
	}
	if p.String("agent_id") != "" {
		return verifyInSubagent(h, text, where, retry, marker)
	}
	return gateOnChecker(h, text, where, retry, marker)
}

// verifyInSubagent checks a report or post written inside a subagent with
// the hook's own run, since a subagent can't start the claim-checker agent.
// The text is sent back once; that exact text, resent, goes through.
func verifyInSubagent(h *hook.Hook, text, where, retry, marker string) error {
	if marker == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(text))
	sentBack := "sent-back " + hex.EncodeToString(sum[:])
	if prev, err := os.ReadFile(marker); err == nil && string(prev) == sentBack {
		_ = os.Remove(marker)
		return nil
	}
	u := unconfirmed(h, text, agentTranscript(h.Payload), where)
	if len(u) == 0 {
		return nil
	}
	if !h.DryRun {
		_ = fsx.WriteAtomic(marker, []byte(sentBack), 0o644)
	}
	h.Deny(claimsMessage(where, u, retry), "claims")
	return nil
}

// claimChecker is the agent a main-session text waits for.
const claimChecker = "kit:claim-checker"

// gateOnChecker denies the call until the transcript shows a claim-checker
// run that finished, whose prompt carries this exact text's claim-check id,
// as GuardPlan waits for plan-critic. The hook decides when a check must
// run, never the model making the claims; whether Claude acted on the
// report is left to the transcript. A session that started before the
// agent was installed can't run it, so a not-found result falls back to
// the hook's own run.
func gateOnChecker(h *hook.Hook, text, where, retry, marker string) error {
	cfg := h.Config()
	if cfg == nil || h.RuleDisabled("claims") || cfg.Claims.Off() || !claims.Declarative(text) {
		return nil
	}
	sum := sha256.Sum256([]byte(text))
	id := "claim-check " + hex.EncodeToString(sum[:])[:12]
	switch checkerRun(h.Payload.String("transcript_path"), id) {
	case checkerDone:
		return nil
	case checkerFailed:
		return verifyInSubagent(h, text, where, retry, marker)
	}
	h.Deny(fmt.Sprintf("run the %s agent on %s first. Its prompt holds the line `%s` and the full text. Check or cut each claim it reports, per skills/kit-rules/reference/evidence.md section 1, then %s. A changed text needs its own run", claimChecker, where, id, retry), "claims")
	return nil
}

// A claim-checker run's outcome, as the transcript shows it.
const (
	checkerNotRun = iota // no run on this text has ended
	checkerDone          // a run on this text finished
	checkerFailed        // a run on this text failed, or the agent is missing
)

// checkerRun is how the transcript's claim-checker runs on the text with
// claim-check id ended. A failed run, or one whose every check errored,
// hands the text to the hook's own verifier, so a checker that keeps
// failing can't block it for good, nor pass it unchecked.
func checkerRun(path, id string) int {
	entries, _ := transcript.Load(path)
	calls := map[string]bool{}
	launched := map[string]string{} // background runs' agent ids, by call, ended once their notice says so
	outcome := checkerNotRun
	blind := func(agent string) bool { return checkedNothing(agentTranscriptPath(path, agent)) }
	for _, e := range entries {
		for call, agent := range launched {
			switch status := noticeStatus(e, call); {
			case status == "":
			case status == "completed" && !blind(agent):
				return checkerDone
			default:
				outcome = checkerFailed
			}
		}
		for _, b := range e.Blocks {
			switch {
			case b.Type == "tool_use" && b.Name == "Agent" && b.Input.SubagentType == claimChecker && strings.Contains(b.Input.Prompt, id):
				calls[b.ID] = true
			case b.Type != "tool_result" || !calls[b.ToolUseID]:
			case b.IsError:
				outcome = checkerFailed
			case strings.HasPrefix(b.ResultText(), asyncLaunched):
				launched[b.ToolUseID] = agentID(b.ResultText())
			case blind(agentID(b.ResultText())):
				outcome = checkerFailed
			default:
				return checkerDone
			}
		}
	}
	return outcome
}

// agentIDLine is where the Agent tool's result names the run's agent, in
// both a finished run's report and a background launch.
var agentIDLine = regexp.MustCompile(`\bagentId: ([\w-]+)`)

// agentID is the agent an Agent tool result names, "" when it names none.
func agentID(result string) string {
	if m := agentIDLine.FindStringSubmatch(result); m != nil {
		return m[1]
	}
	return ""
}

// checkedNothing is true when the agent transcript at path shows checks,
// any tool call but a Skill load, and every one of them errored: a run
// the rules gate or a permission denial kept from reading anything. A
// transcript that can't be read counts as a run that checked.
func checkedNothing(path string) bool {
	entries, err := transcript.Load(path)
	if err != nil {
		return false
	}
	checks := map[string]bool{}
	answered := false
	for _, e := range entries {
		for _, b := range e.Blocks {
			switch {
			case b.Type == "tool_use" && b.Name != "Skill":
				checks[b.ID] = true
			case b.Type == "tool_result" && checks[b.ToolUseID]:
				if !b.IsError {
					return false
				}
				answered = true
			}
		}
	}
	return answered
}

// asyncLaunched opens the Agent tool's result for a run it started in the
// background: the report comes later, in a task notification.
const asyncLaunched = "Async agent launched"

// noticeStatus is the status e's task notification gives the background
// Agent call call, "" when e isn't one for it.
func noticeStatus(e transcript.Entry, call string) string {
	text := e.Notice + e.Text
	for _, b := range e.Blocks {
		text += b.Text
	}
	if !strings.Contains(text, "<tool-use-id>"+call+"</tool-use-id>") {
		return ""
	}
	_, rest, _ := strings.Cut(text, "<status>")
	status, _, _ := strings.Cut(rest, "</status>")
	return status
}

// verifyHandback checks a subagent's report with the hook's own run, once
// per agent: a subagent can't start the claim-checker agent itself.
func verifyHandback(h *hook.Hook) error {
	marker := handbackMarker(h, cache.Claims)
	if marker == "" {
		return nil
	}
	if prev, err := os.ReadFile(marker); err == nil && string(prev) == "sent-back" {
		return nil
	}
	u := unconfirmed(h, h.Payload.String("tool_input.message"), agentTranscript(h.Payload), "your report")
	state := "seen"
	if len(u) > 0 {
		state = "sent-back"
	}
	if !h.DryRun {
		_ = fsx.WriteAtomic(marker, []byte(state), 0o644)
	}
	if len(u) > 0 {
		h.Deny(claimsMessage("your report", u, sendBack), "claims")
	}
	return nil
}

// unconfirmed is what the verifier found unconfirmed in text, logged with
// its cost; nil when text states nothing, without kit.yml, or when the
// verifier fails, which never blocks but tells the user the text went
// unchecked. A text the session already checked gets that verdict again,
// with no run.
func unconfirmed(h *hook.Hook, text, transcriptPath, where string) []claims.Unsupported {
	cfg := h.Config()
	if cfg == nil || h.DryRun || h.RuleDisabled("claims") || !claims.Declarative(text) {
		return nil
	}
	sum := sha256.Sum256([]byte(text))
	cached := h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), "verdict", hex.EncodeToString(sum[:])[:16])
	if data, err := os.ReadFile(cached); err == nil {
		var u []claims.Unsupported
		if json.Unmarshal(data, &u) == nil {
			h.Log(kitlog.Guards, "claims-cached", "where", where, "unconfirmed", strconv.Itoa(len(u)))
			return u
		}
	}
	v, err := claims.Verify(cfg, h.Paths.Root, h.Paths.CacheDir(), h.Payload.Cwd(), text, transcriptPath)
	if errors.Is(err, claims.ErrOff) {
		return nil
	}
	if err != nil {
		h.Log(kitlog.Guards, "claims-error", append([]string{"where", where, "error", err.Error()}, v.Usage.Pairs()...)...)
		h.Notify(fmt.Sprintf("kit claims: NOT CHECKED %s, the verifier failed: %s", where, err))
		// For the turn's receipt at Stop.
		_ = fsx.Append(claimsUnchecked(h), []byte(where+"\n"))
		return nil
	}
	if data, err := json.Marshal(v.Unsupported); err == nil && cached != "" {
		_ = fsx.WriteAtomic(cached, data, 0o644)
	}
	h.Log(kitlog.Guards, "claims", append([]string{"where", where, "unconfirmed", strconv.Itoa(len(v.Unsupported))}, v.Usage.Pairs()...)...)
	h.Notify(fmt.Sprintf("kit claims: checked %s with the hook's own %s run, %d unconfirmed, $%.2f in %s", where, claimChecker, len(v.Unsupported), v.Usage.CostUSD, v.Usage.Duration.Round(time.Second)))
	return v.Unsupported
}

// claimsUnchecked is the session's list of texts the verifier failed on
// this turn; the turn-tree hook clears it at each prompt.
func claimsUnchecked(h *hook.Hook) string {
	return h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), "unchecked")
}

// claimsMessage names each unconfirmed claim, enough of it to find, and the
// check that would settle it, then what to do after.
func claimsMessage(where string, u []claims.Unsupported, then string) string {
	lines := []string{fmt.Sprintf("%d claim(s) in %s that nothing in the session confirms", len(u), where)}
	for i, c := range u {
		claim := c.Claim
		if r := []rune(claim); len(r) > 60 {
			claim = string(r[:57]) + "..."
		}
		lines = append(lines, layout.KV(strconv.Itoa(i+1), strconv.Quote(claim)), layout.KV("check", c.Check))
	}
	lines = append(lines, layout.KV("do", "run each check, or cut the claim, per skills/kit-rules/reference/evidence.md section 1"))
	if then != "" {
		lines = append(lines, layout.KV("then", then))
	}
	return strings.Join(lines, "\n")
}

// agentTranscript is a subagent's own transcript when it exists, else the
// session's.
func agentTranscript(p *hook.Payload) string {
	if path := p.String("agent_transcript_path"); path != "" && fsx.IsFile(path) {
		return path
	}
	// Only SubagentStop names the agent's transcript; a tool call inside a
	// subagent carries agent_id.
	if path := subagentTranscript(p); path != "" && fsx.IsFile(path) {
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
