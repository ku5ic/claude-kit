package hooks

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claims"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/stats"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// ledger logs each flagged claim to claims.jsonl as kept or dropped in
// text, the version that went out after the flag. The claim's text is
// logged there, never in guards.jsonl.
func ledger(h *hook.Hook, source, model, where string, flagged []string, text string) {
	statements := claims.Statements(text)
	for _, c := range flagged {
		outcome := "dropped"
		if stats.Kept(c, statements) {
			outcome = "kept"
		}
		h.Log(kitlog.Claims, "claim", "source", source, "model", model, "where", where, "claim", c, "outcome", outcome)
	}
}

// savePending keeps the claims the hook's own run sent back, beside its
// once-only marker, for ledgerPending to score against the next version.
func savePending(h *hook.Hook, marker string, u []claims.Unsupported) {
	var flagged []string
	for _, c := range u {
		flagged = append(flagged, c.Claim)
	}
	if data, err := json.Marshal(flagged); err == nil && !h.DryRun {
		_ = fsx.WriteAtomic(marker+".flags", data, 0o600)
	}
}

// ledgerPending logs the claims savePending kept for marker against text,
// the version sent after them, once.
func ledgerPending(h *hook.Hook, marker, where, text string) {
	data, err := os.ReadFile(marker + ".flags")
	if err != nil || h.DryRun {
		return
	}
	_ = os.Remove(marker + ".flags")
	var flagged []string
	model := "default"
	if cfg := h.Config(); cfg != nil {
		model = cmp.Or(cfg.Claims.Model, model)
	}
	if json.Unmarshal(data, &flagged) == nil {
		ledger(h, "hook", model, where, flagged, text)
	}
}

// coveringRuns is each finished claim-checker run, with a reply, on text
// or on an earlier version that shares a statement with it.
func coveringRuns(log checkerLog, record, text string) []transcript.Run {
	ids := coveringIDs(record, text)
	var runs []transcript.Run
	for _, run := range transcript.AgentRuns(log.entries, claimChecker) {
		if run.Status == transcript.RunDone && run.Result != "" &&
			slices.ContainsFunc(ids, func(id string) bool { return strings.Contains(run.Prompt, id) }) {
			runs = append(runs, run)
		}
	}
	return runs
}

// coveringIDs is the claim-check ids the session's asked record holds for
// a statement of text.
func coveringIDs(record, text string) []string {
	keys := map[string]bool{}
	for _, s := range claims.Statements(text) {
		keys[statementKey(s)] = true
	}
	var ids []string
	for _, a := range readAsked(record) {
		if slices.ContainsFunc(a.keys, func(k string) bool { return keys[k] }) {
			ids = append(ids, a.id)
		}
	}
	return ids
}

// latestRuns is, per claim-check id covering text, the last finished run
// asked about it, and those ids: a rerun on an id, after Claude ran the
// checks it named, replaces what the earlier run flagged.
func latestRuns(log checkerLog, record, text string) ([]transcript.Run, []string) {
	ids := coveringIDs(record, text)
	last := map[string]transcript.Run{}
	for _, run := range transcript.AgentRuns(log.entries, claimChecker) {
		if run.Status != transcript.RunDone || run.Result == "" {
			continue
		}
		for _, id := range ids {
			if strings.Contains(run.Prompt, id) {
				last[id] = run
			}
		}
	}
	var runs []transcript.Run
	var asked []string
	for _, id := range ids {
		if run, ok := last[id]; ok && !slices.ContainsFunc(runs, func(r transcript.Run) bool { return r.At == run.At }) {
			runs, asked = append(runs, run), append(asked, id)
		}
	}
	return runs, asked
}

// evidenceLabel marks a statement offered as an idea, not a fact, per
// skills/kit-rules/reference/evidence.md section 1: likely, a hypothesis,
// or a test not run.
var evidenceLabel = regexp.MustCompile(`(?i)\b(?:likely|hypothesis|not run)\b`)

// stillStated is each claim runs flagged that text still states in a
// statement with no evidence label, with the check the run named for it.
func stillStated(runs []transcript.Run, text string) []string {
	statements := slices.DeleteFunc(claims.Statements(text), evidenceLabel.MatchString)
	var out []string
	for _, run := range runs {
		for line := range strings.Lines(run.Result) {
			c := stats.Flagged(line)
			if len(c) != 1 || !stats.Kept(c[0], statements) {
				continue
			}
			_, check, _ := strings.Cut(line, " -> ")
			if item := c[0] + " -> " + strings.TrimSpace(check); !slices.Contains(out, item) {
				out = append(out, item)
			}
		}
	}
	return out
}

// ledgerRuns logs the flags of each finished claim-checker run on text, or
// on an earlier version that shares a statement with it, once per run.
func ledgerRuns(h *hook.Hook, log checkerLog, record, text, where string) {
	if h.DryRun {
		return
	}
	done := h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), "ledgered")
	seen, _ := os.ReadFile(done)
	for _, run := range coveringRuns(log, record, text) {
		at := fmt.Sprintf("%d:%d", run.At.Entry, run.At.Block)
		if ledgered(seen, at) {
			continue
		}
		_ = fsx.Append(done, []byte(at+"\n"))
		model := run.Model
		if model == "" {
			model = cmp.Or(agentModel(h), "inherit")
		}
		ledger(h, "agent", model, where, stats.Flagged(run.Result), text)
	}
}

// ledgered is true when seen, the ledgered file, holds at as a whole line.
func ledgered(seen []byte, at string) bool {
	for line := range strings.Lines(string(seen)) {
		if strings.TrimSuffix(line, "\n") == at {
			return true
		}
	}
	return false
}

// agentModel is the model agents/claim-checker.md pins, "" when none.
func agentModel(h *hook.Hook) string {
	data, _ := os.ReadFile(claims.AgentFile(h.Paths.Root))
	for line := range strings.Lines(string(data)) {
		if model, ok := strings.CutPrefix(line, "model:"); ok {
			return strings.TrimSpace(model)
		}
	}
	return ""
}

// asked is one line of the session's asked file: a claim-check id, the
// statement keys it covers, and the report file it was asked for.
type asked struct {
	id   string
	keys []string
	dest string
}

// readAsked is the asked file's lines, the first for each id, in order.
func readAsked(record string) []asked {
	var out []asked
	for line := range strings.Lines(record) {
		id, rest, ok := strings.Cut(strings.TrimRight(line, "\n"), "\t")
		keys, dest, _ := strings.Cut(rest, "\t")
		if ok && !slices.ContainsFunc(out, func(a asked) bool { return a.id == id }) {
			out = append(out, asked{id, strings.Split(keys, ","), dest})
		}
	}
	return out
}
