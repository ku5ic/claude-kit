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

func TestVerifyClaimsStop(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		payload     map[string]any
		unconfirmed []string
		status      int
		runs        int
	}{
		{"block: an unconfirmed claim", stop(claim, false), []string{claim}, 2, 1},
		{"allow: every claim confirmed", stop(claim, false), nil, 0, 1},
		{"allow: the stop after a block, unchecked", stop(claim, true), []string{claim}, 0, 0},
		{"allow: nothing stated, unchecked", stop("Which branch?", false), []string{claim}, 0, 0},
		{"block: a subagent's report", subagentStop("a1", claim, false), []string{claim}, 2, 1},
		{"allow: a subagent's second stop", subagentStop("a1", claim, true), []string{claim}, 0, 0},
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
	}{
		{"a subagent's handback", func(*sandbox) map[string]any { return handback("a1", claim) }, 1},
		{"an investigation report", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": report(k), "content": "# Debug\n\n" + claim})
		}, 1},
		{"a connector post", func(*sandbox) map[string]any {
			return toolCall("mcp__chat__send_message", map[string]any{"text": claim})
		}, 1},
		{"a connector read", func(*sandbox) map[string]any {
			return toolCall("mcp__chat__search_messages", map[string]any{"query": claim})
		}, 0},
		{"a source file", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", "README.md"), "content": claim})
		}, 0},
		{"another scratch file", func(k *sandbox) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "notes.md"), "content": claim})
		}, 0},
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
			// Sent back once: the retry goes through unchecked, and the
			// next attempt after it is checked again.
			k.run("verify-claims", tc.call(k)).Allowed(t)
			k.run("verify-claims", tc.call(k)).Denied(t)
			if got := len(testutil.Calls(t, calls)); got != 2 {
				t.Errorf("verifier ran %d times, want 2", got)
			}
		})
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
		k.run("verify-claims", stop(claim, false)).Want(t, 0)
		if log := read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, "claims-error") {
			t.Errorf("guards log lacks the error:\n%s", log)
		}
	})
	t.Run("claims in disabled_rules lets it through", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.verifier("disabled_rules: [claims]\n", claim)
		k.run("verify-claims", stop(claim, false)).Want(t, 0)
	})
	t.Run("timeout 0 turns the verifier off", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("  timeout: 0\n", claim)
		k.run("verify-claims", stop(claim, false)).Want(t, 0)
		if got := testutil.Calls(t, calls); got != nil {
			t.Errorf("verifier ran: %q", got)
		}
	})
}
