package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claims"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

const claim = "The build fails because Foo was renamed in v2."

// verifierOn is the overlay block that turns the verifier on, which kit.yml
// leaves off, with a stub's path as its command.
const verifierOn = "claims:\n  timeout: 240\n  command: [%q]\n"

// verifier points kit.yml's claims.command at a stub answering every run
// with a verdict naming unconfirmed (none: ok), and writes the overlay
// with extra. It returns the stub's calls file.
func (k *sandbox) verifier(extra string, unconfirmed ...string) string {
	k.t.Helper()
	var u []map[string]string
	for _, c := range unconfirmed {
		u = append(u, map[string]string{"claim": c, "check": "git log -S Foo"})
	}
	answer, _ := json.Marshal(map[string]any{"is_error": false, "subtype": "success",
		"structured_output": map[string]any{"ok": len(u) == 0, "unsupported": u}})
	dir := k.t.TempDir()
	testutil.Write(k.t, filepath.Join(dir, "answer"), string(answer))
	stub, calls := filepath.Join(dir, "verifier"), filepath.Join(dir, "calls")
	testutil.FakeTool(k.t, stub, calls, fmt.Sprintf("cat >/dev/null\ncat %q", filepath.Join(dir, "answer")))
	k.overlay(fmt.Sprintf(verifierOn, stub) + extra)
	return calls
}

func stop(message string, active bool) map[string]any {
	return map[string]any{"hook_event_name": "Stop", "session_id": "s", "last_assistant_message": message, "stop_hook_active": active}
}

func toolCall(tool string, input map[string]any) map[string]any {
	return map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "tool_name": tool, "tool_input": input}
}

func TestVerifyClaimsSubagentStop(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		payload     map[string]any
		unconfirmed []string
		status      int
		runs        int
	}{
		{"allow: a chat reply is never checked", stop(claim, false), []string{claim}, 0, 0},
		{"block: an unconfirmed claim", subagentStop("a1", claim, false), []string{claim}, 2, 1},
		{"allow: every claim confirmed", subagentStop("a1", claim, false), nil, 0, 1},
		{"allow: nothing stated, unchecked", subagentStop("a1", "Which branch?", false), []string{claim}, 0, 0},
		{"block: a retry stop not yet checked", subagentStop("a1", claim, true), []string{claim}, 2, 1},
		{"allow: a retry stop with no agent_id", subagentStop("", claim, true), []string{claim}, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			calls := k.verifier("", tc.unconfirmed...)
			r := k.run("verify-claims", tc.payload)
			if tc.status == 2 {
				// With the run's notice line, the block is a JSON decision.
				r.Want(t, 0)
				r.Has(t, `"decision":"block"`, "nothing in the session confirms", claim, "git log -S Foo", "skills/kit-rules/reference/evidence.md", "kit claims: checked", "the hook's own kit:claim-checker run")
			} else {
				r.Want(t, tc.status)
			}
			if got := len(testutil.Calls(t, calls)); got != tc.runs {
				t.Errorf("verifier ran %d times, want %d", got, tc.runs)
			}
		})
	}
}

// A report whose only statement is a bare reference is settled with no
// verifier run, and the user gets no "checked" notice for a run that never
// happened; a broken reference still sends the report back.
func TestBareReferencesNeedNoVerifierRun(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, report string
		status       int
	}{
		{"a real reference", "See `x.go:1`.", 0},
		{"an invented line", "See `x.go:9`.", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			proj := filepath.Join(k.home, "proj")
			testutil.Put(t, proj, "x.go", "package x\n")
			calls := k.verifier("", claim)
			payload := subagentStop("a1", tc.report, false)
			payload["cwd"] = proj
			r := k.run("verify-claims", payload)
			r.Want(t, tc.status)
			if tc.status == 2 {
				r.Has(t, "x.go:9", "the file has 1 lines")
			}
			r.Lacks(t, "kit claims: checked")
			if got := len(testutil.Calls(t, calls)); got != 0 {
				t.Errorf("verifier ran %d times, want 0", got)
			}
		})
	}
}

func TestVerifyClaimsToolInput(t *testing.T) {
	t.Parallel()
	report := func(k *sandbox) string {
		return filepath.Join(k.home, "proj", ".claude", "scratch", "debug-build-20261110-1200.md")
	}
	for _, tc := range []struct {
		name string
		call func(k *sandbox) map[string]any
		// gate: "own" gets the hook's own run, "none" is never checked.
		gate string
	}{
		{"an investigation report", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": report(k), "content": "# Debug\n\n" + claim})
		}, "own"},
		{"a connector post", func(*sandbox) map[string]any {
			return toolCall("mcp__chat__send_message", map[string]any{"text": claim})
		}, "own"},
		{"a connector post's description, beside its code", func(*sandbox) map[string]any {
			return toolCall("mcp__claude_ai_Figma__use_figma", map[string]any{"code": "figma.root", "description": claim})
		}, "own"},
		{"a connector post's code", func(*sandbox) map[string]any {
			return toolCall("mcp__claude_ai_Figma__use_figma", map[string]any{"code": "// " + claim, "description": "Rename it."})
		}, "none"},
		{"a connector read", func(*sandbox) map[string]any {
			return toolCall("mcp__chat__search_messages", map[string]any{"query": claim})
		}, "none"},
		{"a source file", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", "README.md"), "content": claim})
		}, "none"},
		{"another scratch file", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "notes.md"), "content": claim})
		}, "none"},
		{"a report with no slug", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "debug.md"), "content": "# Debug\n\n" + claim})
		}, "own"},
		{"a rerun's numbered report", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "debug-build-2.md"), "content": "# Debug\n\n" + claim})
		}, "own"},
		{"a scratch file whose name only starts like a kind", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "debugging.md"), "content": claim})
		}, "none"},
		{"an audit report", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "ci-main-20261116-0900.md"), "content": "# CI\n\n" + claim})
		}, "own"},
		{"a /code report", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "impact-auth-20261116-0900.md"), "content": "# Impact\n\n" + claim})
		}, "own"},
		{"a session retro", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "session-90886c74-20261116-0900.md"), "content": "# Session\n\n" + claim})
		}, "own"},
		{"a /deps report", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "deps.md"), "content": "# Deps\n\n" + claim})
		}, "own"},
		{"a PR review draft", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "pr-review-42.md"), "content": claim})
		}, "own"},
		{"a plan critique", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "plan-critique-auth.md"), "content": claim})
		}, "own"},
		{"a report named in capitals", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "Explore-Auth.md"), "content": claim})
		}, "own"},
		{"a PR draft", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "pr-auth-20261116-0900.md"), "content": claim})
		}, "none"},
		{"a report's raw file in its sibling folder", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "ci-main-20261116-0900", "ci-raw.md"), "content": claim})
		}, "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			calls := k.verifier("", claim)
			first := k.run("verify-claims", tc.call(k))
			if tc.gate == "none" {
				first.Allowed(t)
				return
			}
			// A text goes back once per destination, so its rewrite and every
			// later revision go through with no run.
			first.Denied(t)
			first.Has(t, claim, "git log -S Foo")
			first.Lacks(t, "claim-check ")
			k.run("verify-claims", tc.call(k)).Allowed(t)
			k.run("verify-claims", tc.call(k)).Allowed(t)
			if got := len(testutil.Calls(t, calls)); got != 1 {
				t.Errorf("verifier ran %d times, want 1", got)
			}
		})
	}
	for _, agent := range []string{"kit:claim-checker", "kit:plan-critic"} {
		t.Run(agent+"'s hand-back gets no run", func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			calls := k.verifier("", claim)
			payload := handback("a1", claim)
			payload["agent_type"] = agent
			k.run("verify-claims", payload).Allowed(t)
			if got := testutil.Calls(t, calls); got != nil {
				t.Errorf("verifier ran: %q", got)
			}
		})
	}
	t.Run("a project agent named claim-checker is checked", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("", claim)
		payload := handback("a1", claim)
		payload["agent_type"] = "claim-checker"
		k.run("verify-claims", payload).Denied(t)
		if got := testutil.Calls(t, calls); got == nil {
			t.Error("verifier never ran")
		}
	})
	t.Run("a subagent's handback gets the hook's own run, once per agent", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("", claim)
		first := k.run("verify-claims", handback("a1", claim))
		first.Denied(t)
		first.Has(t, claim, "git log -S Foo")
		k.run("verify-claims", handback("a1", claim)).Allowed(t)
		k.run("verify-claims", handback("a1", claim)).Allowed(t)
		if got := len(testutil.Calls(t, calls)); got != 1 {
			t.Errorf("verifier ran %d times, want 1", got)
		}
	})
	t.Run("disabled_rules: [claims] lets it through", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.verifier("disabled_rules: [claims]\n", claim)
		k.run("verify-claims", toolCall("mcp__chat__send_message", map[string]any{"text": claim})).Allowed(t)
	})
	t.Run("a statement the session already checked goes through, with no run", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("", claim)
		k.run("verify-claims", handback("a1", claim)).Denied(t)
		k.run("verify-claims", handback("a2", claim)).Allowed(t)
		if got := len(testutil.Calls(t, calls)); got != 1 {
			t.Errorf("verifier ran %d times, want 1: once per statement", got)
		}
	})
	t.Run("a checked handback leaves the subagent's stop alone", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("")
		k.run("verify-claims", handback("a1", claim)).Allowed(t)
		k.run("verify-claims", subagentStop("a1", claim, false)).Want(t, 0)
		if got := len(testutil.Calls(t, calls)); got != 1 {
			t.Errorf("verifier ran %d times, want 1", got)
		}
	})
}

// A flagged text goes back once, and the rewrite that answers it goes
// through with no run, so a rewrite never loops. The text after that is a
// new one, and is checked.
func TestVerifyClaimsPassesTheRewriteAfterASendBack(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	const other = "Deploys run at noon because the cron says 12."
	calls := k.verifier("", claim, other)
	post := func(text string) map[string]any {
		p := toolCall("mcp__chat__send_message", map[string]any{"text": text})
		p["agent_id"] = "a1"
		return p
	}
	k.run("verify-claims", post(claim)).Denied(t)
	k.run("verify-claims", post(other)).Allowed(t)
	if got := len(testutil.Calls(t, calls)); got != 1 {
		t.Errorf("verifier ran %d times for a text and its rewrite, want 1", got)
	}
	k.run("verify-claims", post(other)).Denied(t)
}

// A statement is checked once per session, wherever it goes next: a report
// revised in place, and its body written to a second file, take one deny
// between them, and the verifier runs only on statements it hasn't seen.
func TestVerifyClaimsChecksAStatementOncePerSession(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	calls := k.verifier("", claim)
	scratch := filepath.Join(k.home, "proj", ".claude", "scratch")
	write := func(name, text string) result {
		return k.run("verify-claims", toolCall("Write", map[string]any{"file_path": filepath.Join(scratch, name), "content": text}))
	}
	const added = "Deploys run at noon because the cron says 12."
	denies := 0
	for _, r := range []result{
		write("explore-x.md", claim),
		write("explore-x.md", claim+"\n"),
		write("explore-x.md", claim+"\n\n"+added),
		write("explore-y.md", claim+"\n\n"+added),
	} {
		if strings.Contains(r.Stdout, `"permissionDecision":"deny"`) {
			denies++
		}
	}
	if denies != 1 {
		t.Errorf("denied %d times, want 1", denies)
	}
	if got := len(testutil.Calls(t, calls)); got != 2 {
		t.Errorf("verifier ran %d times, want 2: the draft, then the added sentence", got)
	}
}

// The verifier runs from the project root, so a path a claim names
// resolves there whichever directory the session is in.
func TestVerifyClaimsRunsFromTheProjectRoot(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	calls := k.verifier("")
	root := testutil.Physical(t, t.TempDir())
	testutil.GitID(t, root, "init", "-q")
	sub := filepath.Join(root, "go", "internal")
	testutil.Mkdir(t, sub)
	main := filepath.Join(t.TempDir(), "sess.jsonl")
	testutil.Write(t, main, `{"type":"user","message":{"content":"parent prompt"}}`+"\n")
	payload := subagentStop("a1", claim, false)
	payload["cwd"], payload["transcript_path"] = sub, main
	k.run("verify-claims", payload).Want(t, 0)
	if got := testutil.Calls(t, calls); len(got) != 1 || !strings.HasPrefix(got[0], root+"|") {
		t.Errorf("the verifier ran as %q, want once in %s", got, root)
	}
}

// A SubagentStop whose agent_transcript_path names no file is checked
// against the session's transcript, not left with no evidence at all.
func TestVerifyClaimsMissingAgentTranscriptFallsBack(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	dir := t.TempDir()
	answer, _ := json.Marshal(map[string]any{"is_error": false, "subtype": "success", "structured_output": map[string]any{"ok": true, "unsupported": []any{}}})
	testutil.Write(t, filepath.Join(dir, "answer"), string(answer))
	stdin, stub := filepath.Join(dir, "stdin"), filepath.Join(dir, "verifier")
	testutil.FakeTool(t, stub, filepath.Join(dir, "calls"), fmt.Sprintf("cat >%q\ncat %q", stdin, filepath.Join(dir, "answer")))
	k.overlay(fmt.Sprintf(verifierOn, stub))
	main := filepath.Join(dir, "sess.jsonl")
	testutil.Write(t, main, `{"type":"user","message":{"content":"parent prompt"}}`+"\n")
	payload := subagentStop("a1", claim, false)
	payload["transcript_path"], payload["agent_transcript_path"] = main, filepath.Join(dir, "gone.jsonl")
	r := k.run("verify-claims", payload)
	r.Want(t, 0)
	if strings.Contains(r.Output, "NOT CHECKED") {
		t.Errorf("a missing agent transcript failed the check:\n%s", r.Output)
	}
	if got := read(t, stdin); !strings.Contains(got, "parent prompt") {
		t.Errorf("evidence isn't the session's transcript:\n%s", got)
	}
}

// A tool call inside a subagent carries agent_id, not agent_transcript_path
// (code.claude.com/docs/en/hooks), so its text is checked against
// <session>/subagents/agent-<agent_id>.jsonl, what that subagent read.
func TestVerifyClaimsInsideASubagentUsesItsTranscript(t *testing.T) {
	t.Parallel()
	for name, call := range map[string]func(k *sandbox) map[string]any{
		"a scratch report": func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "explore-x-20261110-1200.md"), "content": "# Explore\n\n" + claim})
		},
		"a connector post": func(*sandbox) map[string]any {
			return toolCall("mcp__chat__send_message", map[string]any{"text": claim})
		},
		"a handback": func(*sandbox) map[string]any {
			return toolCall("SubagentHandback", map[string]any{"message": claim})
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			dir := t.TempDir()
			answer, _ := json.Marshal(map[string]any{"is_error": false, "subtype": "success", "structured_output": map[string]any{"ok": true, "unsupported": []any{}}})
			testutil.Write(t, filepath.Join(dir, "answer"), string(answer))
			stdin, stub := filepath.Join(dir, "stdin"), filepath.Join(dir, "verifier")
			testutil.FakeTool(t, stub, filepath.Join(dir, "calls"), fmt.Sprintf("cat >%q\ncat %q", stdin, filepath.Join(dir, "answer")))
			k.overlay(fmt.Sprintf(verifierOn, stub))
			main := filepath.Join(dir, "sess.jsonl")
			testutil.Write(t, main, `{"type":"user","message":{"content":"parent prompt"}}`+"\n")
			testutil.Write(t, filepath.Join(dir, "sess", "subagents", "agent-a1.jsonl"), `{"type":"user","isSidechain":true,"message":{"content":"agent prompt"}}`+"\n")
			payload := call(k)
			payload["transcript_path"], payload["agent_id"] = main, "a1"
			k.run("verify-claims", payload).Allowed(t)
			if got := read(t, stdin); !strings.Contains(got, "agent prompt") || strings.Contains(got, "parent prompt") {
				t.Errorf("evidence isn't the agent's transcript:\n%s", got)
			}
		})
	}
}

// Hooks on one event run in parallel (code.claude.com/docs/en/hooks), so
// verify-claims runs last inside each dispatcher: a slop block stops it
// before its model run, and clean text still gets checked.
func TestVerifyClaimsRunsOnlyAfterTheCheapGuards(t *testing.T) {
	t.Parallel()
	report := func(k *sandbox, text string) map[string]any {
		return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "explore-x-20261110-1200.md"), "content": "# Explore\n\n" + text})
	}
	for _, tc := range []struct {
		hook string
		call func(k *sandbox, text string) map[string]any
		// clean is what a clean text gets, the hook's run's finding; runs
		// is how many hook runs that takes.
		clean string
		runs  int
	}{
		{"guard-dispatch", report, "git log -S Foo", 1},
		{"handback-dispatch", func(_ *sandbox, text string) map[string]any { return handback("a1", text) }, "git log -S Foo", 1},
		{"connector-dispatch", func(_ *sandbox, text string) map[string]any {
			return toolCall("mcp__chat__send_message", map[string]any{"text": text})
		}, "git log -S Foo", 1},
		{"subagent-dispatch", func(_ *sandbox, text string) map[string]any { return subagentStop("a1", text, false) }, "git log -S Foo", 1},
	} {
		t.Run(tc.hook, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			calls := k.verifier("", claim)
			r := k.run(tc.hook, tc.call(k, claim+" "+tell))
			r.Has(t, "I hope this helps")
			r.Lacks(t, "git log -S Foo")
			if got := testutil.Calls(t, calls); got != nil {
				t.Errorf("verifier ran after a slop block: %q", got)
			}
			fresh := newSandbox(t)
			freshCalls := fresh.verifier("", claim)
			fresh.run(tc.hook, tc.call(fresh, claim)).Has(t, tc.clean)
			if got := len(testutil.Calls(t, freshCalls)); got != tc.runs {
				t.Errorf("verifier ran %d times on clean text, want %d", got, tc.runs)
			}
		})
	}
}

// One send-back lets through the next text to the same destination, its
// rewrite, and nothing else: a post elsewhere is checked, and a send-back
// that didn't take (claims shadowed) passes nothing on.
func TestASendBackPassesOnlyTheRewriteToTheSameDestination(t *testing.T) {
	t.Parallel()
	const other = "The cache moved to the session directory in v3."
	t.Run("a subagent's gh post to another destination", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("", claim, other)
		post := map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "agent_id": "a1", "tool_name": "Bash", "tool_input": map[string]any{"command": "gh"}}
		k.run("check-post", post, claim, "gh pr create --body").Has(t, "git log -S Foo")
		k.run("check-post", post, other, "gh issue comment --body").Has(t, "git log -S Foo")
		if got := len(testutil.Calls(t, calls)); got != 2 {
			t.Errorf("verifier ran %d times, want one per destination", got)
		}
	})
	t.Run("a report written again with claims shadowed", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("shadow_rules: [claims]\n", claim, other)
		path := filepath.Join(k.home, "proj", ".claude", "scratch", "explore-x-20261110-1200.md")
		k.run("verify-claims", toolCall("Write", map[string]any{"file_path": path, "content": "# Explore\n\n" + claim})).Allowed(t)
		k.run("verify-claims", toolCall("Write", map[string]any{"file_path": path, "content": "# Explore\n\n" + other})).Allowed(t)
		if got := len(testutil.Calls(t, calls)); got != 2 {
			t.Errorf("verifier ran %d times, want the second text checked too", got)
		}
	})
}

// A main-session post of a body the session already checked passes with no
// run and no ask for the claim-checker agent.
func TestAMainSessionPostOfACheckedBodyPasses(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	calls := k.verifier("")
	path := filepath.Join(k.home, "proj", ".claude", "scratch", "pr-description-x.md")
	k.run("verify-claims", toolCall("Write", map[string]any{"file_path": path, "content": claim})).Allowed(t)
	post := map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "tool_name": "Bash", "tool_input": map[string]any{"command": "gh"}}
	r := k.run("check-post", post, claim, "gh pr edit --body")
	r.Allowed(t)
	r.Lacks(t, "kit:claim-checker")
	if got := len(testutil.Calls(t, calls)); got != 1 {
		t.Errorf("verifier ran %d times, want 1: the report, not the post", got)
	}
}

// A report slop sent back still has its claims checked when the subagent
// stops again, and only once.
func TestSubagentDispatchChecksClaimsAfterASlopRetry(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	calls := k.verifier("", claim)
	k.run("subagent-dispatch", subagentStop("a1", claim+" "+tell, false)).Has(t, "I hope this helps")
	retry := k.run("subagent-dispatch", subagentStop("a1", claim, true))
	retry.Want(t, 0)
	retry.Has(t, `"decision":"block"`, "git log -S Foo")
	k.run("subagent-dispatch", subagentStop("a1", claim, true)).Want(t, 0)
	if got := len(testutil.Calls(t, calls)); got != 1 {
		t.Errorf("verifier ran %d times, want 1", got)
	}
}

func TestVerifyClaimsFailsOpen(t *testing.T) {
	t.Parallel()
	t.Run("a verifier that fails sends the report back once, for Claude to check itself", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		stub := filepath.Join(t.TempDir(), "verifier")
		testutil.FakeTool(t, stub, filepath.Join(t.TempDir(), "calls"), "cat >/dev/null\nexit 1")
		k.overlay(fmt.Sprintf(verifierOn, stub))
		r := k.run("verify-claims", subagentStop("a1", claim, false))
		r.Want(t, 2)
		r.Has(t, "BLOCKED", "the verifier failed", "check each claim with a tool yourself")
		r.Lacks(t, "NOT CHECKED")
		if log := read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, "claims-error") {
			t.Errorf("guards log lacks the error:\n%s", log)
		}
		k.run("verify-claims", subagentStop("a1", claim, true)).Empty(t)
	})
	t.Run("after a failure the verifier is paused for every text, which goes back to Claude", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		stub, calls := filepath.Join(t.TempDir(), "verifier"), filepath.Join(t.TempDir(), "calls")
		testutil.FakeTool(t, stub, calls, "cat >/dev/null\nexit 1")
		k.overlay(fmt.Sprintf(verifierOn, stub))
		k.run("verify-claims", subagentStop("a1", claim, false)).Has(t, "the verifier failed")
		r := k.run("verify-claims", subagentStop("a2", "The cache lives in the session directory.", false))
		r.Want(t, 2)
		r.Has(t, "BLOCKED", "the verifier is paused after failing at ", "check each claim with a tool yourself")
		r.Lacks(t, "NOT CHECKED")
		if got := len(testutil.Calls(t, calls)); got != 1 {
			t.Errorf("verifier ran %d times, want 1", got)
		}
	})
	t.Run("a verifier run logs what it cost", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		stub := filepath.Join(t.TempDir(), "verifier")
		testutil.FakeTool(t, stub, filepath.Join(t.TempDir(), "calls"),
			`cat >/dev/null; echo '{"is_error":false,"structured_output":{"ok":true,"unsupported":[]},"total_cost_usd":0.62,"duration_ms":9009}'`)
		k.overlay(fmt.Sprintf(verifierOn, stub))
		k.run("verify-claims", subagentStop("a1", claim, false)).Want(t, 0)
		if log := read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, `"cost_usd":"0.6200","duration_ms":"9009"`) {
			t.Errorf("guards log lacks the cost:\n%s", log)
		}
	})
	t.Run("claims in disabled_rules lets it through", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.verifier("disabled_rules: [claims]\n", claim)
		k.run("verify-claims", subagentStop("a1", claim, false)).Want(t, 0)
	})
	for preset, runs := range map[string]int{"standard": 0, "light": 0, "strict": 1} {
		t.Run("the verifier on the "+preset+" preset", func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			calls := k.verifier("", claim)
			k.overlay(fmt.Sprintf("preset: %s\nclaims:\n  command: [%q]\n", preset, filepath.Join(filepath.Dir(calls), "verifier")))
			r := k.run("verify-claims", subagentStop("a1", claim, false))
			if runs == 0 {
				r.Want(t, 0)
				r.Empty(t)
			} else {
				r.Has(t, "BLOCKED", claim)
			}
			if got := len(testutil.Calls(t, calls)); got != runs {
				t.Errorf("verifier ran %d times, want %d", got, runs)
			}
		})
	}
}

func TestClaimsMessageClipsByCharacter(t *testing.T) {
	t.Parallel()
	msg := claimsMessage("your report", []claims.Unsupported{{Claim: strings.Repeat("é", 70), Check: "read it"}}, "")
	if want := strings.Repeat("é", 57) + "..."; !strings.Contains(msg, want) {
		t.Errorf("claim not clipped to 57 characters:\n%s", msg)
	}
	if strings.Contains(msg, `\x`) {
		t.Errorf("clip split a character:\n%s", msg)
	}
}

// A new skill's or agent's report kind would otherwise skip the check.
// unslop's is the user's own draft, and prompt's a prompt, neither a set of
// claims. A kind ends at any character a kind can't hold, a backtick
// included.
func TestCheckedReportsCoverEverySkillsReport(t *testing.T) {
	kind := regexp.MustCompile("kit scratch-dir ([a-z0-9-]+)(?:[^a-z0-9-]|$)")
	skills, _ := filepath.Glob(filepath.Join("..", "..", "..", "skills", "*", "SKILL.md"))
	agents, _ := filepath.Glob(filepath.Join("..", "..", "..", "agents", "*.md"))
	for _, skill := range append(skills, agents...) {
		name := strings.TrimSuffix(filepath.Base(skill), ".md")
		if name == "SKILL" {
			name = filepath.Base(filepath.Dir(skill))
		}
		text, err := os.ReadFile(skill)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range kind.FindAllStringSubmatch(string(text), -1) {
			if m[1] != "unslop" && m[1] != "prompt" && !slices.Contains(checkedReports, m[1]) {
				t.Errorf("%s writes a %q report, missing from checkedReports", name, m[1])
			}
		}
	}
}

// The statement ledger holds the claims a session made; only its owner
// reads it.
func TestACachedVerdictIsTheOwnersAlone(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	k.verifier("", claim)
	k.run("verify-claims", subagentStop("a1", claim, false))
	verdicts, _ := filepath.Glob(filepath.Join(k.paths.CacheDir(), cache.Claims, "s-ledger"))
	if len(verdicts) != 1 {
		t.Fatalf("verdicts = %q", verdicts)
	}
	if info, err := os.Stat(verdicts[0]); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, %v; want 0600", info.Mode(), err)
	}
}

// A report sits directly in a scratch dir, the session scratchpad
// included; a raw file a level down isn't one.
func TestAtScratchRootTakesEveryScratchDir(t *testing.T) {
	t.Parallel()
	paths := config.Paths{ClaudeDir: "/home/u/.claude"}
	pad := fmt.Sprintf("/tmp/claude-%d/proj/sess/scratchpad", os.Getuid())
	for path, want := range map[string]bool{
		"/home/u/.claude/scratch/review-x.md":     true,
		"/repo/.claude/scratch/review-x.md":       true,
		pad + "/review-x.md":                      true,
		"/repo/.claude/scratch/review-x/notes.md": false,
		pad + "/raw/review-x.md":                  false,
		"/repo/docs/review-x.md":                  false,
	} {
		if got := atScratchRoot(paths, path); got != want {
			t.Errorf("atScratchRoot(%s) = %v, want %v", path, got, want)
		}
	}
}
