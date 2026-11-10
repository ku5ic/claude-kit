package hooks

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

const claim = "The build fails because Foo was renamed in v2."

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
	k.overlay(fmt.Sprintf("claims:\n  command: [%q]\n%s", stub, extra))
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
			r.Want(t, tc.status)
			if tc.status == 2 {
				r.Has(t, "nothing in the session confirms", claim, "git log -S Foo", "rules/evidence.md")
			}
			if got := len(testutil.Calls(t, calls)); got != tc.runs {
				t.Errorf("verifier ran %d times, want %d", got, tc.runs)
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
		runs int // per call: 0 when the tool is never checked
		// rechecked: a third attempt, after the waived retry, is checked
		// again. A handback is checked once per agent.
		rechecked bool
	}{
		{"a subagent's handback", func(*sandbox) map[string]any { return handback("a1", claim) }, 1, false},
		{"an investigation report", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": report(k), "content": "# Debug\n\n" + claim})
		}, 1, true},
		{"a connector post", func(*sandbox) map[string]any {
			return toolCall("mcp__chat__send_message", map[string]any{"text": claim})
		}, 1, true},
		{"a connector post's description, beside its code", func(*sandbox) map[string]any {
			return toolCall("mcp__claude_ai_Figma__use_figma", map[string]any{"code": "figma.root", "description": claim})
		}, 1, true},
		{"a connector post's code", func(*sandbox) map[string]any {
			return toolCall("mcp__claude_ai_Figma__use_figma", map[string]any{"code": "// " + claim, "description": "Rename it."})
		}, 0, false},
		{"a connector read", func(*sandbox) map[string]any {
			return toolCall("mcp__chat__search_messages", map[string]any{"query": claim})
		}, 0, false},
		{"a source file", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", "README.md"), "content": claim})
		}, 0, false},
		{"another scratch file", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "notes.md"), "content": claim})
		}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			calls := k.verifier("", claim)
			first := k.run("verify-claims", tc.call(k))
			if tc.runs == 0 {
				first.Allowed(t)
				if got := testutil.Calls(t, calls); got != nil {
					t.Errorf("verifier ran: %q", got)
				}
				return
			}
			first.Denied(t)
			first.Has(t, claim, "git log -S Foo")
			// Sent back once: the retry goes through unchecked.
			k.run("verify-claims", tc.call(k)).Allowed(t)
			third, want := k.run("verify-claims", tc.call(k)), 1
			if tc.rechecked {
				third.Denied(t)
				want = 2
			} else {
				third.Allowed(t)
			}
			if got := len(testutil.Calls(t, calls)); got != want {
				t.Errorf("verifier ran %d times, want %d", got, want)
			}
		})
	}
	t.Run("a sent-back post doesn't waive a different one", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("", claim)
		post := func(text string) map[string]any {
			return toolCall("mcp__chat__send_message", map[string]any{"text": text})
		}
		k.run("verify-claims", post(claim)).Denied(t)
		k.run("verify-claims", post("Deploys run at noon because the cron says 12.")).Denied(t)
		if got := len(testutil.Calls(t, calls)); got != 2 {
			t.Errorf("verifier ran %d times, want 2", got)
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
			k.overlay(fmt.Sprintf("claims:\n  command: [%q]\n", stub))
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
	}{
		{"guard-dispatch", report},
		{"handback-dispatch", func(_ *sandbox, text string) map[string]any { return handback("a1", text) }},
		{"connector-dispatch", func(_ *sandbox, text string) map[string]any {
			return toolCall("mcp__chat__send_message", map[string]any{"text": text})
		}},
		{"subagent-dispatch", func(_ *sandbox, text string) map[string]any { return subagentStop("a1", text, false) }},
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
			fresh.run(tc.hook, tc.call(fresh, claim)).Has(t, "git log -S Foo")
			if got := len(testutil.Calls(t, freshCalls)); got != 1 {
				t.Errorf("verifier ran %d times on clean text, want 1", got)
			}
		})
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
	retry.Want(t, 2)
	retry.Has(t, "git log -S Foo")
	k.run("subagent-dispatch", subagentStop("a1", claim, true)).Want(t, 0)
	if got := len(testutil.Calls(t, calls)); got != 1 {
		t.Errorf("verifier ran %d times, want 1", got)
	}
}

func TestVerifyClaimsFailsOpen(t *testing.T) {
	t.Parallel()
	t.Run("a verifier that fails is logged, never blocks", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		stub := filepath.Join(t.TempDir(), "verifier")
		testutil.FakeTool(t, stub, filepath.Join(t.TempDir(), "calls"), "cat >/dev/null\nexit 1")
		k.overlay(fmt.Sprintf("claims:\n  command: [%q]\n", stub))
		k.run("verify-claims", subagentStop("a1", claim, false)).Want(t, 0)
		if log := read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, "claims-error") {
			t.Errorf("guards log lacks the error:\n%s", log)
		}
	})
	t.Run("claims in disabled_rules lets it through", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.verifier("disabled_rules: [claims]\n", claim)
		k.run("verify-claims", subagentStop("a1", claim, false)).Want(t, 0)
	})
	t.Run("timeout 0 turns the verifier off", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("  timeout: 0\n", claim)
		k.run("verify-claims", subagentStop("a1", claim, false)).Want(t, 0)
		if got := testutil.Calls(t, calls); got != nil {
			t.Errorf("verifier ran: %q", got)
		}
	})
}
