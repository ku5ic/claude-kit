package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
)

// VerifyClaims holds a text until its claims were checked, so each is
// checked or labeled before anyone relies on it. A chat reply is never
// checked; see checkClaims.
//   - SubagentStop, and PreToolUse for SubagentHandback: a subagent's
//     report, by the hook's own run against its transcript, once per agent.
//   - PreToolUse for ExitPlanMode, an MCP tool that writes, or a Write or
//     Edit of a scratch report (checkedReports): the hook's own run, sent
//     back once per plan, tool, or report.
//
// It runs last in its dispatcher and skips a call an earlier check denied,
// so no run is wasted.
func VerifyClaims(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil || earlierDeny(h) {
		return nil
	}
	// The checker's report is the check; checking it again buys nothing.
	// plan-critic cites a file:line for each finding. A plugin agent's type
	// carries its namespace, so a project agent of the same name is someone
	// else's and gets checked.
	if agent := p.String(hook.KeyAgentType); agent == claimChecker || agent == planCritic {
		return nil
	}
	switch p.String(hook.KeyHookEventName) {
	case "SubagentStop":
		// The marker, not stop_hook_active, ends the retry: a stop that
		// follows subagent-slop's block is this check's first.
		marker := handbackMarker(h, cache.Claims)
		if marker == "" && p.Bool(hook.KeyStopHookActive) || marker != "" && fsx.IsFile(marker) {
			return nil
		}
		u := unconfirmed(h, p.String(hook.KeyLastAssistantMessage), agentTranscript(p), "subagent report")
		if len(u) == 0 {
			// Best effort: without it, the next stop checks again.
			if marker != "" && !h.DryRun {
				_ = fsx.WriteAtomic(marker, []byte(seenMark), 0o644)
			}
			return nil
		}
		if marker == "" || markOnce(h, marker, []byte(seenMark), "claims") {
			return h.Block(claimsMessage("your report", u, sendBack), "claims")
		}
	case "PreToolUse":
		return verifyToolInput(h)
	}
	return nil
}

// earlierDeny is true when a check before this one denied the call, so this
// one can skip its work. ExitPlanMode runs every check anyway, and the one
// deny names each reason, so a plan needs one round trip.
func earlierDeny(h *hook.Hook) bool {
	decision, _ := h.Decision()
	return decision == "deny" && h.Payload.String(hook.KeyToolName) != "ExitPlanMode"
}

// verifyToolInput checks a handback, a plan, a scratch report, or a
// connector post with the hook's own run.
func verifyToolInput(h *hook.Hook) error {
	p := h.Payload
	tool := p.String(hook.KeyToolName)
	var text, where, retry, marker string
	switch {
	case tool == "SubagentHandback":
		return verifyHandback(h)
	case tool == "ExitPlanMode":
		// Claude Code injects the plan's text and path (code.claude.com/docs/en/hooks).
		plan := filepath.Base(p.String(hook.KeyInputPlanFilePath))
		text, where, retry = p.String(hook.KeyInputPlan), "the plan "+plan, "call ExitPlanMode again"
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "plan", plan)
		// The plan changes after plan-critic's run, so while that run is
		// being asked for, nothing.
		if _, reason := h.Decision(); strings.Contains(reason, criticAsk) {
			return nil
		}
	case slices.Contains(filePathTools, tool):
		path := p.FilePath()
		if !scratchReport(h, path, checkedReports...) || !atScratchRoot(h, path) {
			return nil
		}
		_, after, ok := written(p, path)
		if !ok {
			return nil
		}
		name := filepath.Base(path)
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "file", name)
		// After its one send-back a report goes through, and its new
		// statements are still checked into the ledger for where it goes next.
		if sentBackBefore(marker) {
			ledgerPending(h, marker, name, after)
			_ = unconfirmed(h, after, agentTranscript(p), name)
			return nil
		}
		return verifyOwnRun(h, after, name, "write the report again", marker)
	default:
		cfg := h.Config()
		if cfg == nil || !connector.IsWrite(cfg.Connectors, tool) {
			return nil
		}
		text, where, retry = strings.Join(p.Strings(hook.KeyToolInput, cfg.Connectors.CodeFields...), "\n\n"), "the "+tool+" input", "send it again"
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "tool", tool)
	}
	return verifyOwnRun(h, text, where, retry, marker)
}

// CheckPost is check-size and verify-claims on text, what a Bash command
// posts through gh or glab, read from where. It's held as a connector
// post's input is. PreToolUse for Bash, last in bash-dispatch.
func CheckPost(h *hook.Hook, text, where string) error {
	p := h.Payload
	cfg := h.Config()
	if p.Err != nil || cfg == nil || text == "" {
		return p.Err
	}
	if decision, _ := h.Decision(); decision == "deny" {
		return nil
	}
	sizeOnce(h, text, where, "send it again", cfg.SizeBudgets.Post, h.Paths.SessionFile(cache.Size, p.SessionID(), "tool", "Bash"))
	if decision, _ := h.Decision(); decision == "deny" || p.String(hook.KeyAgentType) == claimChecker {
		return nil
	}
	// One marker per destination, so a send-back passes only its rewrite.
	return verifyOwnRun(h, text, where, "send it again", h.Paths.SessionFile(cache.Claims, p.SessionID(), "tool", "Bash", textHash(where)[:12]))
}

// CheckBashReports is check-size and verify-claims on each scratch report
// in written, the files a Bash command wrote, as a Write of that report
// gets them. The command already ran, so a deny becomes a block. PostToolUse
// for Bash.
func CheckBashReports(h *hook.Hook, written []string) error {
	p := h.Payload
	if p.Err != nil || h.Config() == nil {
		return p.Err
	}
	var reasons []string
	seen := map[string]bool{}
	for _, path := range written {
		if seen[path] || !scratchReport(h, path, checkedReports...) || !atScratchRoot(h, path) {
			continue
		}
		seen[path] = true
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		raw, _ := json.Marshal(map[string]any{
			"hook_event_name": "PreToolUse", "session_id": p.SessionID(), "cwd": p.Cwd(),
			"transcript_path": p.String(hook.KeyTranscriptPath), "agent_id": p.String(hook.KeyAgentID), "agent_type": p.String(hook.KeyAgentType),
			"tool_name": "Write", "tool_input": map[string]any{"file_path": path, "content": string(content)},
		})
		sub := subHook(h, "bash-reports", raw)
		for _, c := range []hook.NamedCheck{{Name: "check-size", Check: CheckSize}, {Name: "verify-claims", Check: VerifyClaims}} {
			if b := hook.RunCheck(sub, c.Name, c.Check); b != nil {
				reasons = append(reasons, b.Reason)
				break
			}
		}
		if decision, reason := sub.Decision(); decision == "deny" {
			reasons = append(reasons, reason)
		}
	}
	if len(reasons) == 0 {
		return nil
	}
	return h.Block("a report this command wrote isn't ready\n"+strings.Join(reasons, "\n"), "bash-reports")
}

// verifyOwnRun checks a text with the hook's own run. The text is sent back
// once, and the next text to where, its rewrite, goes through with no run.
func verifyOwnRun(h *hook.Hook, text, where, retry, marker string) error {
	if marker == "" {
		return nil
	}
	ledgerPending(h, marker, where, text)
	if rewriteOf(marker) {
		return nil
	}
	u := unconfirmed(h, text, agentTranscript(h.Payload), where)
	if len(u) == 0 {
		return nil
	}
	if markOnce(h, marker, []byte(sentBackMark), "claims") {
		savePending(h, marker, u)
		if !h.Deny(claimsMessage(where, u, retry), "claims") {
			// Let through as it is, so its flags score against this text,
			// and the next text is checked as if none came before.
			ledgerPending(h, marker, where, text)
			_ = os.Remove(marker)
		}
	}
	return nil
}

// claimChecker is the agent whose prompt the hook's own run uses, and
// whose report needs no check.
const claimChecker = "kit:claim-checker"

// bashEditsRecord is the session file under cache.Claims listing, one a
// line, the Bash tool uses whose command changed a file outside scratch and
// the plan dir, each with those files. transcript.Edits sees only the edit
// tools.
const bashEditsRecord = "bash-edits"

// bashEdit is one bashEditsRecord line.
type bashEdit struct {
	ID    string   `json:"id"`
	Files []string `json:"files"`
}

// recordBashEdit adds the Bash tool use to bashEditsRecord when any of
// changed is code, outside scratch and the plan dir.
func recordBashEdit(h *hook.Hook, changed []string) {
	id := h.Payload.String(hook.KeyToolUseID)
	code := slices.DeleteFunc(slices.Clone(changed), func(file string) bool { return !isCode(h, file) })
	if id == "" || h.DryRun || len(code) == 0 {
		return
	}
	line, err := json.Marshal(bashEdit{ID: id, Files: code})
	if err != nil {
		return
	}
	_ = fsx.Append(h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), bashEditsRecord), append(line, '\n'))
}

// bashEdits is the session's bashEditsRecord: each Bash tool use's id, and
// the code files it changed. A line written before the files were recorded
// is the id alone.
func bashEdits(h *hook.Hook) map[string][]string {
	data, _ := os.ReadFile(h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), bashEditsRecord))
	edits := map[string][]string{}
	for line := range strings.Lines(string(data)) {
		var edit bashEdit
		if json.Unmarshal([]byte(line), &edit) != nil {
			edit = bashEdit{ID: strings.TrimSpace(line)}
		}
		if edit.ID != "" {
			edits[edit.ID] = edit.Files
		}
	}
	return edits
}

// isCode is true for a file a claim can be about: outside scratch and the
// plan dir.
func isCode(h *hook.Hook, file string) bool {
	return file != "" && !project.IsScratch(h.Paths, file) && !fsx.Within(project.PlanDir(h.Paths, h.Payload.Cwd()), file)
}

// statementKey is a checked statement's line in the session's record.
func statementKey(s string) string {
	return textHash(s)[:16]
}

// verifyHandback checks a subagent's report with the hook's own run, once
// per agent.
func verifyHandback(h *hook.Hook) error {
	marker := handbackMarker(h, cache.Claims)
	if marker == "" {
		return nil
	}
	message := h.Payload.String(hook.KeyInputMessage)
	if wasSentBack(marker) {
		ledgerPending(h, marker, "your report", message)
		return nil
	}
	u := unconfirmed(h, message, agentTranscript(h.Payload), "your report")
	if len(u) == 0 {
		if !h.DryRun {
			_ = fsx.WriteAtomic(marker, []byte(seenMark), 0o644)
		}
		return nil
	}
	if markOnce(h, marker, []byte(sentBackMark), "claims") {
		savePending(h, marker, u)
		if !h.Deny(claimsMessage("your report", u, sendBack), "claims") {
			ledgerPending(h, marker, "your report", message)
		}
	}
	return nil
}

// unconfirmed is what the verifier found unconfirmed in text, logged with
// its cost; nil when text states nothing or without kit.yml, and selfCheck
// when the verifier fails. Only the statements no ledger holds go to the
// verifier; a text with none needs no run.
func unconfirmed(h *hook.Hook, text, transcriptPath, where string) []claims.Unsupported {
	cfg := h.Config()
	if cfg == nil || h.DryRun || h.RuleDisabled("claims") || !claims.Declarative(text) {
		return nil
	}
	ledger := statementLedgers(h)
	known := ledger.read()
	var fresh, decided []string
	for _, s := range claims.Statements(text) {
		if _, ok := known[statementKey(s)]; ok {
			decided = append(decided, s)
		} else {
			fresh = append(fresh, s)
		}
	}
	if len(fresh) == 0 {
		h.Log(kitlog.Guards, "claims-cached", "where", where, "unconfirmed", "0")
		return nil
	}
	root, _ := project.Root(h.Payload.Cwd())
	v, err := claims.Verify(cfg, h.Paths.Root, h.Paths.CacheDir(), root, strings.Join(fresh, "\n\n"), transcriptPath)
	if errors.Is(err, claims.ErrOff) {
		return nil
	}
	// With no verdict, the text goes back once for Claude to check itself.
	if paused, ok := errors.AsType[*claims.Paused](err); ok {
		h.Log(kitlog.Guards, "claims-paused", "where", where, "error", paused.Why)
		return selfCheck(where, fmt.Sprintf("the verifier is paused after failing at %s (%s)", paused.At.Format("15:04"), paused.Why))
	}
	if err != nil {
		h.Log(kitlog.Guards, "claims-error", append([]string{"where", where, "error", err.Error()}, v.Usage.Pairs()...)...)
		return selfCheck(where, "the verifier failed: "+err.Error())
	}
	v.Unsupported = recordVerdicts(h, ledger, fresh, decided, v)
	h.Log(kitlog.Guards, "claims", append([]string{"where", where, "unconfirmed", strconv.Itoa(len(v.Unsupported))}, v.Usage.Pairs()...)...)
	if v.Usage.Runs == 0 {
		return v.Unsupported
	}
	h.Notify(fmt.Sprintf("kit claims: checked %s with the hook's own %s run, %d unconfirmed, $%.2f in %s", where, claimChecker, len(v.Unsupported), v.Usage.CostUSD, v.Usage.Duration.Round(time.Second)))
	return v.Unsupported
}

// selfCheck is the one flag a text gets when the verifier gave no verdict,
// for why: Claude checks every claim in where itself.
func selfCheck(where, why string) []claims.Unsupported {
	return []claims.Unsupported{{Claim: "every claim in " + where, Check: why + "; check each claim with a tool yourself"}}
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
	if path := p.String(hook.KeyAgentTranscriptPath); path != "" && fsx.IsFile(path) {
		return path
	}
	// Only SubagentStop names the agent's transcript; a tool call inside a
	// subagent carries agent_id.
	if path := subagentTranscript(p); path != "" && fsx.IsFile(path) {
		return path
	}
	return p.String(hook.KeyTranscriptPath)
}

// checkedReports are the scratch report kinds whose claims are checked:
// every skill's but unslop's, the user's own draft, and prompt's.
var checkedReports = []string{
	"explore", "debug", "review", "session", "improve", "deps", "pr-review", "plan-critique",
	"a11y", "arch", "ci", "debt", "doc-drift", "infra", "perf", "security", "slop", "verify",
	"flags", "readiness", "skills", "tests", "tokens",
	"challenge", "contract", "evaluate", "from-design", "history", "impact", "migrate", "tour", "visual",
	"incident", "launch", "pr-description", "pr-reply", "queue", "release-notes", "retro", "rfc", "runbook", "stakeholder", "tickets",
}

// atScratchRoot is true for a file directly in a scratch dir, where
// `kit scratch-dir <kind> <slug>` puts a report; a report's raw files sit
// a level down.
func atScratchRoot(h *hook.Hook, path string) bool {
	dir := filepath.Dir(filepath.Clean(path))
	return dir == h.Paths.ScratchHome() || strings.HasSuffix(dir, "/.claude/scratch")
}

// scratchReport is true for a report of one of kinds in scratch, the files
// `kit scratch-dir <kind> <slug>` names, in any case.
func scratchReport(h *hook.Hook, path string, kinds ...string) bool {
	base := filepath.Base(path)
	name := strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
	return path != "" && project.IsScratch(h.Paths, path) && strings.EqualFold(filepath.Ext(base), ".md") &&
		slices.ContainsFunc(kinds, func(kind string) bool { return name == kind || strings.HasPrefix(name, kind+"-") })
}
