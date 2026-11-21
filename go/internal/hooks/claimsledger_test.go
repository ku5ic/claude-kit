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

// A finished run passes the text only once each claim it flagged is cut;
// a reworded or labelled claim is still stated.
func TestAFlaggedClaimStillStatedIsHeld(t *testing.T) {
	t.Parallel()
	other := "The cache lives in the session directory."
	for _, tc := range []struct {
		name, text string
		allowed    bool
	}{
		{"the flagged claim, unchanged", claim + "\n\n" + other, false},
		{"the flagged claim, reworded", "Foo was renamed in v2, so the build fails.\n\n" + other, false},
		{"the flagged claim, labelled likely", "Likely: " + claim + "\n\n" + other, false},
		{"the flagged claim, as a hypothesis", "Hypothesis: " + claim + "\n\n" + other, false},
		{"the flagged claim cut", other, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			k.verifier("", claim)
			post := func(text, session string) map[string]any {
				p := toolCall("mcp__chat__send_message", map[string]any{"text": text})
				if session != "" {
					p["transcript_path"] = session
				}
				return p
			}
			id := checkID.FindString(k.run("verify-claims", post(claim+"\n\n"+other, "")).Stdout)
			session := filepath.Join(t.TempDir(), "session.jsonl")
			runs := checkerCall(id, "- "+claim+" -> git log -S Foo\n", false) + "\n"
			testutil.Write(t, session, runs)
			// A revision with new sentences gets a clean run on those first.
			if first := k.run("verify-claims", post(tc.text, session)).Stdout; strings.Contains(first, "hasn't checked") {
				next := checkID.FindString(first)
				testutil.Write(t, session, runs+strings.Replace(checkerLine(next), `"cc"`, `"cc2"`, 2)+"\n")
			}
			r := k.run("verify-claims", post(tc.text, session))
			if tc.allowed {
				r.Allowed(t)
				return
			}
			r.Denied(t)
			r.Has(t, "still states", "git log -S Foo")
			if strings.Contains(r.Stdout+r.Stderr, "label") {
				t.Errorf("the hold offers a label as a way out:\n%s", r.Stdout+r.Stderr)
			}
		})
	}
}

// A flag lifts once the checker, run again on the same claim-check id after
// Claude ran the check, no longer flags the claim.
func TestARerunThatConfirmsAFlaggedClaimLiftsIt(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	k.verifier("", claim)
	post := toolCall("mcp__chat__send_message", map[string]any{"text": claim})
	id := checkID.FindString(k.run("verify-claims", post).Stdout)
	session := filepath.Join(t.TempDir(), "session.jsonl")
	flagged := checkerCall(id, "- "+claim+" -> git log -S Foo\n", false) + "\n"
	testutil.Write(t, session, flagged)
	post["transcript_path"] = session
	r := k.run("verify-claims", post)
	r.Denied(t)
	r.Has(t, "run kit:claim-checker again", id)
	testutil.Write(t, session, flagged+strings.ReplaceAll(checkerLine(id), `"cc"`, `"cc2"`)+"\n")
	k.run("verify-claims", post).Allowed(t)
}

// In shadow, a flagged claim still stated goes through, scored as kept.
func TestAShadowedStillStatedFlagGoesThroughAsKept(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	k.verifier("", claim)
	post := toolCall("mcp__chat__send_message", map[string]any{"text": claim})
	id := checkID.FindString(k.run("verify-claims", post).Stdout)
	session := filepath.Join(t.TempDir(), "session.jsonl")
	testutil.Write(t, session, checkerCall(id, "- "+claim+" -> git log -S Foo\n", false)+"\n")
	k.overlay("shadow_rules: [claims]\n")
	post["transcript_path"] = session
	k.run("verify-claims", post).Allowed(t)
	if got := k.ledgerLines(); !strings.Contains(got, `"outcome":"kept"`) {
		t.Errorf("claims.jsonl =\n%s\nwant the flag kept", got)
	}
}

// A ledgered run's position never hides another run whose position it ends
// with.
func TestALedgeredRunIsMatchedWhole(t *testing.T) {
	t.Parallel()
	seen := []byte("21:2\n")
	if ledgered(seen, "1:2") || !ledgered(seen, "21:2") {
		t.Errorf("ledgered(%q): 1:2 %v, 21:2 %v", seen, ledgered(seen, "1:2"), ledgered(seen, "21:2"))
	}
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
