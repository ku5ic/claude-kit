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

// A confirmed statement citing a file keeps its verdict in a new session,
// as after /clear, while HEAD and the file are unchanged; one citing no
// file is checked again in each session.
func TestAConfirmedVerdictSurvivesANewSession(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	calls := k.verifier("")
	dir := repo(t, filepath.Join(k.home, "proj"))
	testutil.Put(t, dir, "x.go", "package x\n")
	runs := func() int { return len(testutil.Calls(t, calls)) }
	write := func(session, text string) {
		t.Helper()
		p := toolCall("Write", map[string]any{"file_path": filepath.Join(dir, ".claude", "scratch", "explore-"+session+".md"), "content": text})
		p["session_id"], p["cwd"] = session, dir
		k.run("verify-claims", p).Allowed(t)
	}
	const cited = "The loader in `x.go` returns early when the cache is warm."
	for _, step := range []struct {
		session, text, change string
		want                  int
	}{
		{"s1", cited, "", 1},
		{"s2", cited, "", 1},
		{"s3", cited, "edit", 2},
		{"s4", cited, "", 2},
		{"s5", cited, "commit", 3},
		{"s6", "Deploys run at noon because the cron says 12.", "", 4},
		{"s7", "Deploys run at noon because the cron says 12.", "", 5},
	} {
		switch step.change {
		case "edit":
			testutil.Put(t, dir, "x.go", "package x\n\nvar warm bool\n")
		case "commit":
			testutil.Git(t, dir, "commit", "-q", "--allow-empty", "-m", "next")
		}
		write(step.session, step.text)
		if got := runs(); got != step.want {
			t.Fatalf("%s: verifier ran %d times, want %d", step.session, got, step.want)
		}
	}
}

// A flag never leaves its session: the statement is checked again in the
// next one, so an unchecked claim can't pass on an old verdict.
func TestAFlaggedVerdictStaysInItsSession(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	const cited = "The loader in `x.go` returns early when the cache is warm."
	calls := k.verifier("", cited)
	dir := repo(t, filepath.Join(k.home, "proj"))
	testutil.Put(t, dir, "x.go", "package x\n")
	for _, session := range []string{"s1", "s2"} {
		p := toolCall("Write", map[string]any{"file_path": filepath.Join(dir, ".claude", "scratch", "explore-"+session+".md"), "content": cited})
		p["session_id"], p["cwd"] = session, dir
		k.run("verify-claims", p).Denied(t)
	}
	if got := len(testutil.Calls(t, calls)); got != 2 {
		t.Errorf("verifier ran %d times, want once per session", got)
	}
}
