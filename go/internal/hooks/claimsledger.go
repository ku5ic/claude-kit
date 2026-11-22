package hooks

import (
	"cmp"
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claims"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/stats"
)

// Verdicts the statement ledger records.
const (
	statementConfirmed = "confirmed"
	statementFlagged   = "flagged"
)

// statementLedger is the session's verdict per statement, a statementKey
// and a verdict per line, shared by every text and destination, so a
// statement is checked once per session.
func statementLedger(h *hook.Hook) string {
	return h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), "ledger")
}

// readStatements is the ledger at path, by statementKey; empty when none.
func readStatements(path string) map[string]string {
	known := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return known
	}
	for line := range strings.Lines(string(data)) {
		if key, verdict, ok := strings.Cut(strings.TrimSpace(line), "\t"); ok {
			known[key] = verdict
		}
	}
	return known
}

// recordStatements appends verdicts to the ledger at path.
func recordStatements(h *hook.Hook, path string, verdicts map[string]string) {
	if path == "" || len(verdicts) == 0 || h.DryRun {
		return
	}
	var b strings.Builder
	for _, key := range slices.Sorted(maps.Keys(verdicts)) {
		b.WriteString(key + "\t" + verdicts[key] + "\n")
	}
	_ = fsx.Append(path, []byte(b.String()))
}

// recordVerdicts records in the ledger what v said of fresh, the
// statements it was given, and returns v's flags less those about decided,
// statements the ledger already holds. A statement counts as confirmed
// only when the model ran: the reference pass and a finding's evidence
// check leave the rest unread. A flag that matches no statement stays a
// flag, unrecorded.
func recordVerdicts(h *hook.Hook, ledger string, fresh, decided []string, v claims.Verdict) []claims.Unsupported {
	verdicts := map[string]string{}
	var out []claims.Unsupported
	for _, u := range v.Unsupported {
		matched := false
		for _, s := range fresh {
			if stats.Kept(u.Claim, []string{s}) {
				verdicts[statementKey(s)] = statementFlagged
				matched = true
			}
		}
		if !matched && stats.Kept(u.Claim, decided) {
			continue
		}
		out = append(out, u)
	}
	if v.Usage.Runs > 0 {
		for _, s := range fresh {
			if _, ok := verdicts[statementKey(s)]; !ok {
				verdicts[statementKey(s)] = statementConfirmed
			}
		}
	}
	recordStatements(h, ledger, verdicts)
	return out
}

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
