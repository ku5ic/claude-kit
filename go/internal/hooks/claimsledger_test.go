package hooks

import (
	"os"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

// ledgerLines is session s's claims.jsonl, "" when nothing was logged.
func (k *sandbox) ledgerLines() string {
	data, _ := os.ReadFile(k.paths.LogFile(kitlog.Claims))
	return string(data)
}

// With claims in shadow_rules, a flagged report goes out as it was, so its
// flags are logged as kept then, never scored against a later report.
func TestAShadowedClaimsFlagIsKept(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	k.verifier("shadow_rules: [claims]\n", claim)
	k.run("verify-claims", handback("a1", claim)).Allowed(t)
	k.run("verify-claims", handback("a1", "## Findings\n\nThe cache lives in the session directory.")).Allowed(t)
	if got := k.ledgerLines(); strings.Count(got, "\n") != 1 || !strings.Contains(got, `"outcome":"kept"`) {
		t.Errorf("claims.jsonl =\n%s\nwant one kept flag", got)
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
