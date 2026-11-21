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
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// VerifyClaims holds a text until its claims were checked, so each is
// checked or labeled before anyone relies on it. A chat reply is never
// checked; see checkClaims.
//   - SubagentStop, and PreToolUse for SubagentHandback: a subagent's
//     report, by the hook's own run against its transcript, once per agent.
//   - PreToolUse for ExitPlanMode: the plan, which plan-critic's run covers,
//     or with plan-critic off, the claim-checker agent.
//   - PreToolUse for an MCP tool that writes: the claim-checker agent on
//     that exact text. Inside a subagent, the hook's own run, once per text.
//   - PreToolUse for a Write or Edit of a scratch report (checkedReports):
//     the hook's own run, sent back once per report.
//
// It runs last in its dispatcher and skips a call an earlier check denied,
// so no run is wasted.
func VerifyClaims(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil || earlierDeny(h) {
		return nil
	}
	// The checker's report is the check; checking it again buys nothing.
	// A plugin agent's type carries its namespace, so a project agent
	// named claim-checker is someone else's and gets checked.
	if p.String(hook.KeyAgentType) == claimChecker {
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
				_ = fsx.WriteAtomic(marker, []byte("seen"), 0o644)
			}
			return nil
		}
		if marker == "" || markOnce(h, marker, []byte("seen"), "claims") {
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

// verifyToolInput checks a handback with the hook's own run, and gates a
// plan, a scratch report, or a connector post on the claim-checker agent.
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
		if !h.RuleDisabled("plan-critic") {
			// plan-critic read the repo, so a revision after it gets only
			// the no-model pass; while its run is being asked for, nothing.
			if planCritiqued(p) {
				return referencesOnce(h, text, where, retry, marker)
			}
			if _, reason := h.Decision(); strings.Contains(reason, criticAsk) {
				return nil
			}
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
		// A report gets the hook's own run, in the main session too.
		marker := h.Paths.SessionFile(cache.Claims, p.SessionID(), "file", filepath.Base(path))
		return verifyOwnRun(h, after, filepath.Base(path), "write the report again", marker)
	default:
		cfg := h.Config()
		if cfg == nil || !connector.IsWrite(cfg.Connectors, tool) {
			return nil
		}
		text, where, retry = strings.Join(p.Strings(hook.KeyToolInput, cfg.Connectors.CodeFields...), "\n\n"), "the "+tool+" input", "send it again"
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "tool", tool)
	}
	return verifyText(h, text, where, retry, marker)
}

// referencesOnce sends text back once when a bare reference in it doesn't
// resolve, with no model run; the next text to marker goes through.
func referencesOnce(h *hook.Hook, text, where, retry, marker string) error {
	cfg := h.Config()
	if cfg == nil || h.RuleDisabled("claims") || cfg.Claims.Off() || marker == "" {
		return nil
	}
	if fsx.IsFile(marker) {
		_ = os.Remove(marker)
		return nil
	}
	root, _ := project.Root(h.Payload.Cwd())
	if u := claims.References(root, text); len(u) > 0 && markOnce(h, marker, []byte("sent-back"), "claims") {
		h.Deny(claimsMessage(where, u, retry), "claims")
	}
	return nil
}

// verifyText checks text with the hook's own run inside a subagent, and
// gates it on the claim-checker agent in the main session.
func verifyText(h *hook.Hook, text, where, retry, marker string) error {
	if h.Payload.String(hook.KeyAgentID) != "" {
		return verifyOwnRun(h, text, where, retry, marker)
	}
	return gateOnChecker(h, text, where, retry, marker)
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
	return verifyText(h, text, where, "send it again", h.Paths.SessionFile(cache.Claims, p.SessionID(), "tool", "Bash"))
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

// verifyOwnRun checks a report, or a post written inside a subagent, with
// the hook's own run, since a subagent can't start the claim-checker agent.
// The text is sent back once, and the next text to where, its rewrite, goes
// through with no run.
func verifyOwnRun(h *hook.Hook, text, where, retry, marker string) error {
	if marker == "" {
		return nil
	}
	ledgerPending(h, marker, where, text)
	if fsx.IsFile(marker) {
		_ = os.Remove(marker)
		return nil
	}
	u := unconfirmed(h, text, agentTranscript(h.Payload), where)
	if len(u) == 0 {
		return nil
	}
	if markOnce(h, marker, []byte("sent-back"), "claims") {
		savePending(h, marker, u)
		if !h.Deny(claimsMessage(where, u, retry), "claims") {
			// Let through as it is, so its flags score against this text.
			ledgerPending(h, marker, where, text)
		}
	}
	return nil
}

// claimChecker is the agent a main-session text waits for.
const claimChecker = "kit:claim-checker"

// gateOnChecker denies the call until the transcript shows a claim-checker
// run that finished on each of this text's statements, as GuardPlan waits
// for plan-critic. A run's prompt carries the claim-check id of the
// statements it was asked about; once it finishes they're recorded, so a
// revised text asks only about the statements no run checked. The hook
// decides when a check must run, never the model making the claims;
// whether Claude acted on the report is left to the transcript. A session
// that started before the agent was installed can't run it, so a not-found
// result falls back to the hook's own run.
func gateOnChecker(h *hook.Hook, text, where, retry, marker string) error {
	cfg := h.Config()
	if cfg == nil || h.RuleDisabled("claims") || cfg.Claims.Off() || !claims.Declarative(text) {
		return nil
	}
	log := readCheckerLog(h, h.Payload.String(hook.KeyTranscriptPath))
	// No run could ever show in a transcript that can't be read.
	if log.unread {
		return verifyOwnRun(h, text, where, retry, marker)
	}
	asked := h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), "asked")
	record, _ := os.ReadFile(asked)
	checked, outcomes := checkedStatements(log, string(record))
	// passed is every exit where the text needs no new run. A claim a
	// finished run flagged and the text still states, unlabelled, holds it;
	// else its runs' flags go to the ledger.
	passed := func() error {
		runs, ids := latestRuns(log, string(record), text)
		if kept := stillStated(runs, text); len(kept) > 0 &&
			h.Deny("a kit:claim-checker run flagged claims "+where+" still states\n"+"- "+strings.Join(kept, "\n- ")+"\n"+
				layout.KV("do", "run each one's check, or cut it (skills/kit-rules/reference/evidence.md section 1), then run kit:claim-checker again with the line `"+strings.Join(ids, "`, `")+"` and the full text")+"\n"+
				layout.KV("then", retry), "claims") {
			return nil
		}
		ledgerRuns(h, log, string(record), text, where)
		return nil
	}
	all := claims.Statements(text)
	var fresh []string
	for _, s := range all {
		if !slices.Contains(checked, statementKey(s)) {
			fresh = append(fresh, s)
		}
	}
	if len(fresh) == 0 {
		return passed()
	}
	id := "claim-check " + textHash(strings.Join(fresh, "\n"))[:12]
	outcome, recorded := outcomes[id]
	if !recorded {
		outcome = log.runs([]string{id})[id]
	}
	switch outcome {
	case checkerDone:
		return passed()
	case checkerFailed:
		return verifyOwnRun(h, text, where, retry, marker)
	}
	ask := "run it first; its prompt holds the line `" + id + "` and the full text"
	var only string
	if len(fresh) < len(all) {
		ask += ", and asks it to check only these, which no run has checked:"
		only = "\n- " + strings.Join(fresh, "\n- ")
	}
	if !h.DryRun && !strings.Contains(string(record), id+"\t") {
		var keys []string
		for _, s := range fresh {
			keys = append(keys, statementKey(s))
		}
		_ = fsx.Append(asked, []byte(id+"\t"+strings.Join(keys, ",")+"\n"))
	}
	h.Deny(fmt.Sprintf("the %s agent hasn't checked %s\n", claimChecker, where)+layout.KV("do", ask)+only+"\n"+
		layout.KV("then", "check or cut each claim it reports, per skills/kit-rules/reference/evidence.md section 1, then "+retry), "claims")
	return nil
}

// checkedStatements is the keys of the statements a finished claim-checker
// run covered: each run record, the session's asked file, names by its id,
// whose run log shows done. It returns every recorded id's outcome too.
func checkedStatements(log checkerLog, record string) ([]string, map[string]int) {
	lines := readAsked(record)
	ids := make([]string, 0, len(lines))
	for _, a := range lines {
		ids = append(ids, a.id)
	}
	outcomes := log.runs(ids)
	var keys []string
	for _, a := range lines {
		if outcomes[a.id] == checkerDone {
			keys = append(keys, a.keys...)
		}
	}
	return keys, outcomes
}

// checkerLog is the session transcript at path, read once, and the place
// of its last code edit: an edit tool call, or a recorded Bash edit, on a
// file outside scratch and the plans folder. The edit counts from its
// result, or from its call while it has none: a run launched before that,
// beside it in one message too, checked code that has changed since, so
// it counts as no run.
type checkerLog struct {
	path     string
	entries  []transcript.Entry
	lastEdit transcript.Pos
	unread   bool // a transcript was named and couldn't be loaded
}

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

// readCheckerLog is the checkerLog of the transcript at path. The last edit
// is an edit tool's, or a Bash call's that bashEditsRecord names.
func readCheckerLog(h *hook.Hook, path string) checkerLog {
	entries, err := transcript.Load(path)
	bashed := bashEdits(h)
	log := checkerLog{path: path, entries: entries, lastEdit: transcript.Pos{Entry: -1}, unread: path != "" && err != nil}
	edits := map[string]bool{}
	for at, b := range transcript.Blocks(entries) {
		switch _, ok := bashed[b.ID]; {
		case b.Type == "tool_use" && (ok || slices.Contains(transcript.EditTools, b.Name) && isCode(h, b.Path())):
			log.lastEdit, edits[b.ID] = at, true
		case b.Type == "tool_result" && edits[b.ToolUseID]:
			log.lastEdit = at
		}
	}
	return log
}

// statementKey is a checked statement's line in the session's record.
func statementKey(s string) string {
	return textHash(s)[:16]
}

// A claim-checker run's outcome, as the transcript shows it.
const (
	checkerNotRun = iota // no run on this text has ended
	checkerDone          // a run on this text finished
	checkerFailed        // a run on this text failed, or the agent is missing
)

// runs is how the log's claim-checker runs on the text with each of ids, a
// claim-check id, ended, in one pass over the log. A failed run, or one
// whose every check errored, hands the text to the hook's own verifier, so
// a checker that keeps failing can't block it for good, nor pass it
// unchecked.
func (l checkerLog) runs(ids []string) map[string]int {
	outcome := map[string]int{}
	for _, id := range ids {
		outcome[id] = checkerNotRun
	}
	checked := map[string]bool{}
	blind := func(agent string) bool {
		if was, ok := checked[agent]; ok {
			return was
		}
		checked[agent] = checkedNothing(agentTranscriptPath(l.path, agent))
		return checked[agent]
	}
	// A run launched before the last edit checked code that has changed.
	for _, run := range transcript.AgentRuns(l.entries, claimChecker) {
		if !run.At.After(l.lastEdit) || run.Status == transcript.RunGoing {
			continue
		}
		result := checkerFailed
		if run.Status == transcript.RunDone && !blind(run.Agent) {
			result = checkerDone
		}
		for _, id := range ids {
			if strings.Contains(run.Prompt, id) && outcome[id] != checkerDone {
				outcome[id] = result
			}
		}
	}
	return outcome
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

// verifyHandback checks a subagent's report with the hook's own run, once
// per agent: a subagent can't start the claim-checker agent itself.
func verifyHandback(h *hook.Hook) error {
	marker := handbackMarker(h, cache.Claims)
	if marker == "" {
		return nil
	}
	message := h.Payload.String(hook.KeyInputMessage)
	if prev, err := os.ReadFile(marker); err == nil && string(prev) == "sent-back" {
		ledgerPending(h, marker, "your report", message)
		return nil
	}
	u := unconfirmed(h, message, agentTranscript(h.Payload), "your report")
	if len(u) == 0 {
		if !h.DryRun {
			_ = fsx.WriteAtomic(marker, []byte("seen"), 0o644)
		}
		return nil
	}
	if markOnce(h, marker, []byte("sent-back"), "claims") {
		savePending(h, marker, u)
		if !h.Deny(claimsMessage("your report", u, sendBack), "claims") {
			ledgerPending(h, marker, "your report", message)
		}
	}
	return nil
}

// unconfirmed is what the verifier found unconfirmed in text, logged with
// its cost; nil when text states nothing or without kit.yml, and selfCheck
// when the verifier fails. A text the session already checked gets that
// verdict again, with no run.
func unconfirmed(h *hook.Hook, text, transcriptPath, where string) []claims.Unsupported {
	cfg := h.Config()
	if cfg == nil || h.DryRun || h.RuleDisabled("claims") || !claims.Declarative(text) {
		return nil
	}
	cached := h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), "verdict", textHash(text)[:16])
	if data, err := os.ReadFile(cached); err == nil {
		var u []claims.Unsupported
		if json.Unmarshal(data, &u) == nil {
			h.Log(kitlog.Guards, "claims-cached", "where", where, "unconfirmed", strconv.Itoa(len(u)))
			return u
		}
	}
	root, _ := project.Root(h.Payload.Cwd())
	v, err := claims.Verify(cfg, h.Paths.Root, h.Paths.CacheDir(), root, text, transcriptPath)
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
	if data, err := json.Marshal(v.Unsupported); err == nil && cached != "" {
		_ = fsx.WriteAtomic(cached, data, 0o600)
	}
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
	"explore", "debug", "review", "session", "deps", "pr-review", "plan-critique",
	"a11y", "arch", "ci", "debt", "doc-drift", "infra", "perf", "security", "slop", "verify",
	"flags", "readiness", "tests", "tokens",
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
