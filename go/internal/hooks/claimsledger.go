package hooks

import (
	"cmp"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claims"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/stats"
)

// Verdicts the statement ledger records.
const (
	statementConfirmed = "confirmed"
	statementFlagged   = "flagged"
)

// ledgers are where statement verdicts live: the session's, a statementKey
// and a verdict per line, and the project's, which keeps a confirmed
// statement citing a file across sessions, /clear included, while HEAD and
// each cited file's content match what they were.
type ledgers struct {
	session, project, root, head string
}

// statementLedgers is the payload's ledgers; the project's is "" outside a
// git work tree with a commit.
func statementLedgers(h *hook.Hook) ledgers {
	l := ledgers{session: h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), "ledger")}
	dir, _ := project.Root(h.Payload.Cwd())
	top := git.Toplevel(dir)
	if top == "" {
		return l
	}
	if head, err := git.Line(top, "rev-parse", "HEAD"); err == nil {
		l.root, l.head = top, head
		l.project = filepath.Join(h.Paths.CacheDir(), cache.Claims, cache.RootKey(top)+".ledger")
	}
	return l
}

// read is every verdict that holds now, by statementKey: the session's,
// and each project line whose HEAD and file hashes still match.
func (l ledgers) read() map[string]string {
	known := map[string]string{}
	data, _ := os.ReadFile(l.session)
	for line := range strings.Lines(string(data)) {
		if key, verdict, ok := strings.Cut(strings.TrimSpace(line), "\t"); ok {
			known[key] = verdict
		}
	}
	if l.project == "" {
		return known
	}
	data, _ = os.ReadFile(l.project)
	sums := map[string]string{}
	for line := range strings.Lines(string(data)) {
		f := strings.Split(strings.TrimSpace(line), "\t")
		if len(f) == 4 && f[2] == l.head && l.unchanged(f[3], sums) {
			known[f[0]] = f[1]
		}
	}
	return known
}

// unchanged is true when every path=hash pair of field matches the file
// now; sums holds the hashes read so far.
func (l ledgers) unchanged(field string, sums map[string]string) bool {
	for pair := range strings.SplitSeq(field, ",") {
		path, sum, ok := strings.Cut(pair, "=")
		if !ok {
			return false
		}
		if _, seen := sums[path]; !seen {
			sums[path] = fileSum(filepath.Join(l.root, path))
		}
		if sums[path] == "" || sums[path] != sum {
			return false
		}
	}
	return true
}

// fileSum is a short hash of the file at path, "" when it can't be read.
func fileSum(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return textHash(string(data))[:16]
}

// record appends verdicts, by statementKey, for statements, the texts by
// key. A confirmed statement citing a file goes to the project ledger; the
// rest, a flag included, stays the session's.
// shortcut: the project ledger only grows, one line per statement, until
// it sits a day unused and is pruned; dropping old HEADs' lines on write
// would bound it.
func (l ledgers) record(h *hook.Hook, verdicts, statements map[string]string) {
	if l.session == "" || len(verdicts) == 0 || h.DryRun {
		return
	}
	var session, project strings.Builder
	for _, key := range slices.Sorted(maps.Keys(verdicts)) {
		var files []string
		if l.project != "" && verdicts[key] == statementConfirmed {
			files = claims.CitedFiles(l.root, statements[key])
		}
		if len(files) == 0 {
			session.WriteString(key + "\t" + verdicts[key] + "\n")
			continue
		}
		pairs := make([]string, len(files))
		for i, f := range files {
			pairs[i] = f + "=" + fileSum(filepath.Join(l.root, f))
		}
		project.WriteString(key + "\t" + verdicts[key] + "\t" + l.head + "\t" + strings.Join(pairs, ",") + "\n")
	}
	if session.Len() > 0 {
		_ = fsx.Append(l.session, []byte(session.String()))
	}
	if project.Len() > 0 {
		_ = fsx.Append(l.project, []byte(project.String()))
	}
}

// recordVerdicts records in the ledger what v said of fresh, the
// statements it was given, and returns v's flags less those about decided,
// statements the ledger already holds. A statement counts as confirmed
// only when the model ran: the reference pass and a finding's evidence
// check leave the rest unread. A flag that matches no statement stays a
// flag, unrecorded.
func recordVerdicts(h *hook.Hook, l ledgers, fresh, decided []string, v claims.Verdict) []claims.Unsupported {
	verdicts, statements := map[string]string{}, map[string]string{}
	for _, s := range fresh {
		statements[statementKey(s)] = s
	}
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
	l.record(h, verdicts, statements)
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
