package hooks

import (
	"crypto/sha256"
	"encoding/hex"
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
//   - PreToolUse for a Write or Edit of an investigation, review, audit-*,
//     or code-* report in scratch, and for an MCP tool that writes: the claim-checker agent on
//     that exact text. Inside a subagent, the hook's own run, once per text.
//
// It runs last in its dispatcher and skips a call an earlier check denied,
// so no run is wasted.
func VerifyClaims(h *hook.Hook) error {
	p := h.Payload
	if decision, _ := h.Decision(); p.Err != nil || decision == "deny" {
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

// verifyToolInput checks a handback with the hook's own run, and gates a
// plan, a scratch report, or a connector post on the claim-checker agent.
func verifyToolInput(h *hook.Hook) error {
	p := h.Payload
	tool := p.String(hook.KeyToolName)
	var text, where, retry, marker, dest string
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
		plan := filepath.Base(p.String(hook.KeyInputPlanFilePath))
		text, where, retry = p.String(hook.KeyInputPlan), "the plan "+plan, "call ExitPlanMode again"
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "plan", plan)
	case slices.Contains(filePathTools, tool):
		path := p.FilePath()
		if !scratchReport(h, path, checkedReports...) || !atScratchRoot(h, path) {
			return nil
		}
		_, after, ok := written(p, path)
		if !ok {
			return nil
		}
		text, where, retry, dest = after, filepath.Base(path), "write the report again", path
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "file", filepath.Base(path))
	default:
		cfg := h.Config()
		if cfg == nil || !connector.IsWrite(cfg.Connectors, tool) {
			return nil
		}
		text, where, retry = strings.Join(p.Strings(hook.KeyToolInput, cfg.Connectors.CodeFields...), "\n\n"), "the "+tool+" input", "send it again"
		marker = h.Paths.SessionFile(cache.Claims, p.SessionID(), "tool", tool)
	}
	if p.String(hook.KeyAgentID) != "" {
		return verifyInSubagent(h, text, where, retry, marker)
	}
	return gateOnChecker(h, text, where, retry, marker, dest)
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
	ledgerPending(h, marker, where, text)
	if prev, err := os.ReadFile(marker); err == nil && string(prev) == sentBack {
		_ = os.Remove(marker)
		return nil
	}
	u := unconfirmed(h, text, agentTranscript(h.Payload), where)
	if len(u) == 0 {
		return nil
	}
	if markOnce(h, marker, []byte(sentBack), "claims") {
		savePending(h, marker, u)
		h.Deny(claimsMessage(where, u, retry), "claims")
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
// result falls back to the hook's own run. A report file, dest, that a run
// finished on goes through rewritten: acting on the checker's report
// rewords what it flagged, which would otherwise ask for a run again.
func gateOnChecker(h *hook.Hook, text, where, retry, marker, dest string) error {
	cfg := h.Config()
	if cfg == nil || h.RuleDisabled("claims") || cfg.Claims.Off() || !claims.Declarative(text) {
		return nil
	}
	log := readCheckerLog(h, h.Payload.String(hook.KeyTranscriptPath))
	asked := h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), "asked")
	record, _ := os.ReadFile(asked)
	checked, outcomes, done := checkedStatements(log, string(record))
	if dest != "" && slices.Contains(done, dest) {
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
		ledgerRuns(h, log, string(record), text, where)
		return nil
	}
	sum := sha256.Sum256([]byte(strings.Join(fresh, "\n")))
	id := "claim-check " + hex.EncodeToString(sum[:])[:12]
	outcome, recorded := outcomes[id]
	if !recorded {
		outcome = log.runs([]string{id})[id]
	}
	switch outcome {
	case checkerDone:
		ledgerRuns(h, log, string(record), text, where)
		return nil
	case checkerFailed:
		return verifyInSubagent(h, text, where, retry, marker)
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
		_ = fsx.Append(asked, []byte(id+"\t"+strings.Join(keys, ",")+"\t"+dest+"\n"))
	}
	h.Deny(fmt.Sprintf("the %s agent hasn't checked %s\n", claimChecker, where)+layout.KV("do", ask)+only+"\n"+
		layout.KV("then", "check or cut each claim it reports, per skills/kit-rules/reference/evidence.md section 1, then "+retry), "claims")
	return nil
}

// checkedStatements is the keys of the statements a finished claim-checker
// run covered: each run record, the session's asked file, names by its id,
// whose run log shows done. It returns every recorded id's outcome too, and
// the report files those done runs were asked for.
func checkedStatements(log checkerLog, record string) ([]string, map[string]int, []string) {
	lines := readAsked(record)
	ids := make([]string, 0, len(lines))
	for _, a := range lines {
		ids = append(ids, a.id)
	}
	outcomes := log.runs(ids)
	var keys, done []string
	for _, a := range lines {
		if outcomes[a.id] == checkerDone {
			keys = append(keys, a.keys...)
			if a.dest != "" {
				done = append(done, a.dest)
			}
		}
	}
	return keys, outcomes, done
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
	entries, _ := transcript.Load(path)
	bashed := bashEdits(h)
	log := checkerLog{path: path, entries: entries, lastEdit: transcript.Pos{Entry: -1}}
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
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:16]
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
// investigate's, every audit-* and code-* skill's, and /meta-retro's.
var checkedReports = []string{
	"explore", "debug", "review", "session",
	"a11y", "arch", "ci", "debt", "doc-drift", "infra", "perf", "security", "slop", "verify",
	"flags", "readiness", "tokens",
	"challenge", "contract", "evaluate", "from-design", "history", "impact", "migrate", "tour", "visual",
}

// atScratchRoot is true for a file directly in a scratch dir, where
// `kit scratch-dir <kind> <slug>` puts a report; a report's raw files sit
// a level down.
func atScratchRoot(h *hook.Hook, path string) bool {
	dir := filepath.Dir(filepath.Clean(path))
	return dir == h.Paths.ScratchHome() || strings.HasSuffix(dir, "/.claude/scratch")
}

// scratchReport is true for a report of one of kinds in scratch, the files
// `kit scratch-dir <kind> <slug>` names.
func scratchReport(h *hook.Hook, path string, kinds ...string) bool {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return path != "" && project.IsScratch(h.Paths, path) && strings.EqualFold(filepath.Ext(base), ".md") &&
		slices.ContainsFunc(kinds, func(kind string) bool { return name == kind || strings.HasPrefix(name, kind+"-") })
}
