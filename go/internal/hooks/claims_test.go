package hooks

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/claims"
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

// checkID finds the claim-check id a deny names.
var checkID = regexp.MustCompile(`claim-check [0-9a-f]{12}`)

// checkerLine is a transcript line for a claim-checker run whose prompt
// carries id.
func checkerLine(id string) string {
	return checkerCall(id, "No unconfirmed claims.", false)
}

// checkerCall is a claim-checker run on id and its result, as two lines.
func checkerCall(id, result string, failed bool) string {
	call, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{map[string]any{
		"type": "tool_use", "id": "cc", "name": "Agent",
		"input": map[string]any{"subagent_type": "kit:claim-checker", "prompt": "Check the claims.\n" + id + "\n\nThe text."},
	}}}})
	done, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{
		"type": "tool_result", "tool_use_id": "cc", "is_error": failed, "content": result,
	}}}})
	return string(call) + "\n" + string(done)
}

// checkerRan points payload at a session transcript holding a claim-checker
// run on id.
func checkerRan(k *sandbox, payload map[string]any, id string) map[string]any {
	path := filepath.Join(k.t.TempDir(), "session.jsonl")
	testutil.Write(k.t, path, checkerLine(id)+"\n")
	payload["transcript_path"] = path
	return payload
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

func TestVerifyClaimsToolInput(t *testing.T) {
	t.Parallel()
	report := func(k *sandbox) string {
		return filepath.Join(k.home, "proj", ".claude", "scratch", "debug-build-20261110-1200.md")
	}
	for _, tc := range []struct {
		name string
		call func(k *sandbox) map[string]any
		// gate: "agent" waits for a claim-checker run, "none" is never
		// checked.
		gate string
	}{
		{"an investigation report", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": report(k), "content": "# Debug\n\n" + claim})
		}, "agent"},
		{"a connector post", func(*sandbox) map[string]any {
			return toolCall("mcp__chat__send_message", map[string]any{"text": claim})
		}, "agent"},
		{"a connector post's description, beside its code", func(*sandbox) map[string]any {
			return toolCall("mcp__claude_ai_Figma__use_figma", map[string]any{"code": "figma.root", "description": claim})
		}, "agent"},
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			calls := k.verifier("", claim)
			first := k.run("verify-claims", tc.call(k))
			if tc.gate == "none" {
				first.Allowed(t)
			} else {
				first.Denied(t)
				first.Has(t, "kit:claim-checker", "claim-check ")
				id := checkID.FindString(first.Stdout)
				// Resent without a run, it's denied again; with a run on
				// another text, too; with a run on this one, it goes through.
				k.run("verify-claims", tc.call(k)).Denied(t)
				k.run("verify-claims", checkerRan(k, tc.call(k), "claim-check 000000000000")).Denied(t)
				k.run("verify-claims", checkerRan(k, tc.call(k), id)).Allowed(t)
			}
			// The main session's texts never start the hook's own run.
			if got := testutil.Calls(t, calls); got != nil {
				t.Errorf("verifier ran: %q", got)
			}
		})
	}
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
	t.Run("a changed post needs its own run", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		post := func(text string) map[string]any {
			return toolCall("mcp__chat__send_message", map[string]any{"text": text})
		}
		id := checkID.FindString(k.run("verify-claims", post(claim)).Stdout)
		other := k.run("verify-claims", checkerRan(k, post("Deploys run at noon because the cron says 12."), id))
		other.Denied(t)
		if checkID.FindString(other.Stdout) == id {
			t.Errorf("two texts share the id %s", id)
		}
	})
	t.Run("a background checker run counts once its notice says it completed", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		post := toolCall("mcp__chat__send_message", map[string]any{"text": claim})
		id := checkID.FindString(k.run("verify-claims", post).Stdout)
		path := filepath.Join(t.TempDir(), "session.jsonl")
		launched := checkerCall(id, "Async agent launched successfully.\nagentId: a9", false) + "\n"
		testutil.Write(t, path, launched)
		post["transcript_path"] = path
		k.run("verify-claims", post).Denied(t)
		notice, _ := json.Marshal(map[string]any{"type": "queue-operation", "operation": "enqueue",
			"content": "<task-notification>\n<task-id>a9</task-id>\n<tool-use-id>cc</tool-use-id>\n<status>completed</status>\n</task-notification>"})
		testutil.Write(t, path, launched+string(notice)+"\n")
		k.run("verify-claims", post).Allowed(t)
	})
	t.Run("a failed checker run gets the hook's own run, once per text", func(t *testing.T) {
		t.Parallel()
		notice, _ := json.Marshal(map[string]any{"type": "queue-operation", "operation": "enqueue",
			"content": "<task-notification>\n<tool-use-id>cc</tool-use-id>\n<status>failed</status>\n</task-notification>"})
		for name, failed := range map[string]func(id string) string{
			"in the foreground": func(id string) string { return checkerCall(id, "The agent stopped: API error.", true) },
			"in the background": func(id string) string {
				return checkerCall(id, "Async agent launched successfully.\nagentId: a9", false) + "\n" + string(notice)
			},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				k := newSandbox(t)
				k.verifier("", claim)
				post := toolCall("mcp__chat__send_message", map[string]any{"text": claim})
				id := checkID.FindString(k.run("verify-claims", post).Stdout)
				path := filepath.Join(t.TempDir(), "session.jsonl")
				testutil.Write(t, path, failed(id)+"\n")
				post["transcript_path"] = path
				r := k.run("verify-claims", post)
				r.Denied(t)
				r.Has(t, claim, "git log -S Foo")
				k.run("verify-claims", post).Allowed(t)
			})
		}
	})
	t.Run("a checker run whose every check errored gets the hook's own run", func(t *testing.T) {
		t.Parallel()
		// The checker's own transcript: a Skill load that worked, then each
		// of its checks, ok or denied.
		agentLog := func(checks ...bool) string {
			lines := []string{}
			call := func(id, name string, ok bool) {
				use, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
					map[string]any{"type": "tool_use", "id": id, "name": name, "input": map[string]any{}}}}})
				result, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"content": []any{
					map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": !ok, "content": "kit rules-gate: BLOCKED"}}}})
				lines = append(lines, string(use), string(result))
			}
			call("s1", "Skill", true)
			for i, ok := range checks {
				call(fmt.Sprintf("r%d", i), "Read", ok)
			}
			return strings.Join(lines, "\n") + "\n"
		}
		notice, _ := json.Marshal(map[string]any{"type": "queue-operation", "operation": "enqueue",
			"content": "<task-notification>\n<tool-use-id>cc</tool-use-id>\n<status>completed</status>\n</task-notification>"})
		runs := map[string]func(id string) string{
			"in the foreground": func(id string) string { return checkerCall(id, "No unconfirmed claims.\nagentId: a7", false) },
			"in the background": func(id string) string {
				return checkerCall(id, "Async agent launched successfully.\nagentId: a7", false) + "\n" + string(notice)
			},
		}
		for name, run := range runs {
			for _, tc := range []struct {
				name    string
				checks  []bool
				blinded bool
			}{
				{"every check denied", []bool{false, false}, true},
				{"one check went through", []bool{false, true}, false},
				{"no check needed", nil, false},
			} {
				t.Run(name+", "+tc.name, func(t *testing.T) {
					t.Parallel()
					k := newSandbox(t)
					calls := k.verifier("", claim)
					post := toolCall("mcp__chat__send_message", map[string]any{"text": claim})
					id := checkID.FindString(k.run("verify-claims", post).Stdout)
					dir := t.TempDir()
					path := filepath.Join(dir, "session.jsonl")
					testutil.Write(t, path, run(id)+"\n")
					testutil.Write(t, filepath.Join(dir, "session", "subagents", "agent-a7.jsonl"), agentLog(tc.checks...))
					post["transcript_path"] = path
					r := k.run("verify-claims", post)
					if !tc.blinded {
						r.Allowed(t)
						if got := testutil.Calls(t, calls); got != nil {
							t.Errorf("verifier ran: %q", got)
						}
						return
					}
					r.Denied(t)
					r.Has(t, claim, "git log -S Foo")
					k.run("verify-claims", post).Allowed(t)
				})
			}
		}
	})
	t.Run("a session without the checker agent gets the hook's own run, once per text", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("", claim)
		post := toolCall("mcp__chat__send_message", map[string]any{"text": claim})
		id := checkID.FindString(k.run("verify-claims", post).Stdout)
		path := filepath.Join(t.TempDir(), "session.jsonl")
		testutil.Write(t, path, checkerCall(id, "Agent type 'kit:claim-checker' not found. Available agents: Explore", true)+"\n")
		post["transcript_path"] = path
		r := k.run("verify-claims", post)
		r.Denied(t)
		r.Has(t, claim, "git log -S Foo")
		k.run("verify-claims", post).Allowed(t)
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
	t.Run("a text the session already checked gets its verdict again, with no run", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("", claim)
		k.run("verify-claims", handback("a1", claim)).Denied(t)
		again := k.run("verify-claims", handback("a2", claim))
		again.Denied(t)
		again.Has(t, claim, "git log -S Foo")
		k.run("verify-claims", handback("a3", "Deploys run at noon because the cron says 12.")).Denied(t)
		if got := len(testutil.Calls(t, calls)); got != 2 {
			t.Errorf("verifier ran %d times, want 2: once per distinct text", got)
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
	k.overlay(fmt.Sprintf("claims:\n  command: [%q]\n", stub))
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
		// clean is what a clean text gets: the hook's run's finding, or
		// the claim-checker gate; runs is how many hook runs that takes.
		clean string
		runs  int
	}{
		{"guard-dispatch", report, "kit:claim-checker", 0},
		{"handback-dispatch", func(_ *sandbox, text string) map[string]any { return handback("a1", text) }, "git log -S Foo", 1},
		{"connector-dispatch", func(_ *sandbox, text string) map[string]any {
			return toolCall("mcp__chat__send_message", map[string]any{"text": text})
		}, "kit:claim-checker", 0},
		{"subagent-dispatch", func(_ *sandbox, text string) map[string]any { return subagentStop("a1", text, false) }, "git log -S Foo", 1},
	} {
		t.Run(tc.hook, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			calls := k.verifier("", claim)
			r := k.run(tc.hook, tc.call(k, claim+" "+tell))
			r.Has(t, "I hope this helps")
			r.Lacks(t, "git log -S Foo", "kit:claim-checker")
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
	t.Run("a verifier that fails is logged and named to the user, never blocks", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		stub := filepath.Join(t.TempDir(), "verifier")
		testutil.FakeTool(t, stub, filepath.Join(t.TempDir(), "calls"), "cat >/dev/null\nexit 1")
		k.overlay(fmt.Sprintf("claims:\n  command: [%q]\n", stub))
		r := k.run("verify-claims", subagentStop("a1", claim, false))
		r.Want(t, 0)
		r.Has(t, "kit claims: NOT CHECKED subagent report, the verifier failed")
		if log := read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, "claims-error") {
			t.Errorf("guards log lacks the error:\n%s", log)
		}
	})
	t.Run("a verifier run logs what it cost", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		stub := filepath.Join(t.TempDir(), "verifier")
		testutil.FakeTool(t, stub, filepath.Join(t.TempDir(), "calls"),
			`cat >/dev/null; echo '{"is_error":false,"structured_output":{"ok":true,"unsupported":[]},"total_cost_usd":0.62,"duration_ms":9009}'`)
		k.overlay(fmt.Sprintf("claims:\n  command: [%q]\n", stub))
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
	t.Run("timeout 0 turns the verifier off", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("  timeout: 0\n", claim)
		r := k.run("verify-claims", subagentStop("a1", claim, false))
		r.Want(t, 0)
		r.Empty(t)
		if got := testutil.Calls(t, calls); got != nil {
			t.Errorf("verifier ran: %q", got)
		}
	})
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
