package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// ledgerLines is session s's claims.jsonl, "" when nothing was logged.
func (k *sandbox) ledgerLines() string {
	data, _ := os.ReadFile(k.paths.LogFile(kitlog.Claims))
	return string(data)
}

// A claim the claim-checker agent flagged is logged once, with whether the
// text that went out after it still states it.
func TestAFlagFromTheCheckerIsLoggedWithWhatBecameOfIt(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	k.verifier("", claim)
	other := "The cache lives in the session directory."
	post := func(text string) map[string]any {
		return toolCall("mcp__chat__send_message", map[string]any{"text": text})
	}
	id := checkID.FindString(k.run("verify-claims", post(claim+"\n\n"+other)).Stdout)
	session := filepath.Join(t.TempDir(), "session.jsonl")
	testutil.Write(t, session, checkerCall(id, "- "+claim+" -> git log -S Foo\n", false)+"\n")
	revised := post(other)
	revised["transcript_path"] = session
	k.run("verify-claims", revised).Allowed(t)
	k.run("verify-claims", revised).Allowed(t)
	got := k.ledgerLines()
	if strings.Count(got, "\n") != 1 || !strings.Contains(got, `"source":"agent"`) || !strings.Contains(got, `"outcome":"dropped"`) ||
		!strings.Contains(got, `"claim":"`+claim+`"`) {
		t.Errorf("claims.jsonl =\n%s\nwant one dropped agent flag", got)
	}
}

// The hook's own run on a subagent's report is scored against the report
// handed back after it.
func TestAFlagFromTheHooksRunIsScoredOnTheNextReport(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ next, outcome string }{
		"cut":    {"## Findings\n\nThe cache lives in the session directory.", "dropped"},
		"resent": {claim, "kept"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			k.verifier("", claim)
			k.run("verify-claims", handback("a1", claim)).Denied(t)
			if got := k.ledgerLines(); got != "" {
				t.Fatalf("logged before the next report:\n%s", got)
			}
			k.run("verify-claims", handback("a1", tc.next)).Allowed(t)
			if got := k.ledgerLines(); !strings.Contains(got, `"source":"hook"`) || !strings.Contains(got, `"outcome":"`+tc.outcome+`"`) {
				t.Errorf("claims.jsonl =\n%s\nwant a %s hook flag", got, tc.outcome)
			}
		})
	}
}
