package hooks

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestCheckSize(t *testing.T) {
	t.Parallel()
	// Seven prose words against a budget of five, with fenced code that
	// would put a short text over if it counted.
	const long, short = "One two three four five six seven.", "One two three."
	const fenced = short + "\n\n```\nalpha beta gamma delta epsilon zeta\n```\n"
	budgets := "size_budgets: {plan: 5, report: 5, scratch: 5, post: 5}\n"
	handback := func(text string) map[string]any {
		return map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "agent_id": "a1", "tool_name": "SubagentHandback", "tool_input": map[string]any{"message": text}}
	}
	stop := func(text string, retry bool) map[string]any {
		return map[string]any{"hook_event_name": "SubagentStop", "session_id": "s", "agent_id": "a1", "last_assistant_message": text, "stop_hook_active": retry}
	}

	for _, tc := range []struct {
		name, where, then string
		call              func(k *sandbox, text string) map[string]any
	}{
		{"a handback", "your report", "hand back the shorter report", func(_ *sandbox, text string) map[string]any { return handback(text) }},
		{"a plan", "the plan p.md", "call ExitPlanMode again", func(k *sandbox, text string) map[string]any {
			return toolCall("ExitPlanMode", map[string]any{"plan": text, "planFilePath": filepath.Join(k.home, "proj", ".claude", "plans", "p.md")})
		}},
		{"an investigation report", "explore-x-20261111-1200.md", "write the report again", func(k *sandbox, text string) map[string]any {
			return toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "explore-x-20261111-1200.md"), "content": text})
		}},
		{"a connector post", "the mcp__chat__send_message input", "send it again", func(_ *sandbox, text string) map[string]any {
			return toolCall("mcp__chat__send_message", map[string]any{"text": text})
		}},
	} {
		t.Run(tc.name+" over budget is sent back once per text", func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			k.overlay(budgets)
			r := k.run("check-size", tc.call(k, long))
			r.Denied(t)
			r.Has(t, tc.where+" runs 7 words, over the 5-word budget", tc.then, "(size)")
			k.run("check-size", tc.call(k, long)).Allowed(t)
			k.run("check-size", tc.call(k, long+" Eight.")).Denied(t)
			k.run("check-size", tc.call(k, fenced)).Allowed(t)
		})
	}

	t.Run("a closing message over budget blocks once", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.overlay(budgets)
		k.run("check-size", stop(long, false)).Want(t, 2)
		k.run("check-size", stop(long, true)).Want(t, 0)
		k.run("check-size", stop(short, false)).Want(t, 0)
	})
	t.Run("the stop after a sent-back handback goes through", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.overlay(budgets)
		k.run("check-size", handback(long)).Denied(t)
		k.run("check-size", stop(long, false)).Want(t, 0)
	})
	t.Run("handback-dispatch sends it back before the claims run", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier(budgets, claim)
		r := k.run("handback-dispatch", handback(long))
		r.Denied(t)
		r.Has(t, "(size)")
		if got := testutil.Calls(t, calls); got != nil {
			t.Errorf("verifier ran: %q", got)
		}
	})
	for name, overlay := range map[string]string{
		"disabled_rules: [size] lets it through": budgets + "disabled_rules: [size]\n",
		"a budget of 0 is off":                   "size_budgets: {report: 0}\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			k.overlay(overlay)
			k.run("check-size", handback(long+strings.Repeat(" word", 900))).Allowed(t)
		})
	}
	t.Run("kit.yml's budgets let a short report through", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.run("check-size", handback(long)).Allowed(t)
	})
}
