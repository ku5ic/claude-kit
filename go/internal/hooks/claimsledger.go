package hooks

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
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

// ledgerRuns logs the flags of each finished claim-checker run on text, or
// on an earlier version that shares a statement with it, once per run.
func ledgerRuns(h *hook.Hook, log checkerLog, record, text, where string) {
	if h.DryRun {
		return
	}
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
	done := h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), "ledgered")
	seen, _ := os.ReadFile(done)
	for _, run := range transcript.AgentRuns(log.entries, claimChecker) {
		at := fmt.Sprintf("%d:%d\n", run.At.Entry, run.At.Block)
		if run.Status != transcript.RunDone || run.Result == "" || strings.Contains(string(seen), at) ||
			!slices.ContainsFunc(ids, func(id string) bool { return strings.Contains(run.Prompt, id) }) {
			continue
		}
		_ = fsx.Append(done, []byte(at))
		model := run.Model
		if model == "" {
			model = cmp.Or(agentModel(h), "inherit")
		}
		ledger(h, "agent", model, where, stats.Flagged(run.Result), text)
	}
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
