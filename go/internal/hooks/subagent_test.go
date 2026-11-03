package hooks

import (
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

func handback(agent, message string) map[string]any {
	return map[string]any{
		"hook_event_name": "PreToolUse", "session_id": "s", "agent_id": agent,
		"tool_name": "SubagentHandback", "tool_input": map[string]any{"message": message},
	}
}

func subagentStop(agent, message string, active bool) map[string]any {
	return map[string]any{
		"hook_event_name": "SubagentStop", "session_id": "s", "agent_id": agent,
		"last_assistant_message": message, "stop_hook_active": active,
	}
}

func TestGuardHandback(t *testing.T) {
	t.Parallel()
	t.Run("a tell is sent back once per agent, then goes through", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		first := k.run("guard-handback", handback("a1", "## Findings\n\n"+tell))
		first.Denied(t)
		first.Has(t, "your report", "hand back your full report again")
		k.run("guard-handback", handback("a1", "## Findings\n\n"+tell)).Allowed(t)
		k.run("guard-handback", handback("a2", tell)).Denied(t)
	})
	for _, tc := range []struct{ name, agent, message string }{
		{"a clean report goes through", "a1", "## Findings\n\nThe parser drops the last line."},
		{"a tell quoted in the report goes through", "a1", `Flagged "I hope this helps" in README.md.`},
		{"a dense tier 3 passage goes through", "a1", dense},
		{"no agent id: nothing to track, so nothing is sent back", "", tell},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			newSandbox(t).run("guard-handback", handback(tc.agent, tc.message)).Allowed(t)
		})
	}
	t.Run("a dense tier 3 passage is logged as a subagent report hit", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.run("guard-handback", handback("a1", dense)).Allowed(t)
		if log := read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, "slop-vocabulary") || !strings.Contains(log, "subagent report") {
			t.Errorf("guards log lacks the hit:\n%s", log)
		}
	})
}

func TestSubagentSlop(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		before map[string]any // a handback first, when set
		stop   map[string]any
		status int
	}{
		{"block: a tell in the closing message", nil, subagentStop("a1", "## Report\n\n"+tell, false), 2},
		{"allow: the second stop", nil, subagentStop("a1", tell, true), 0},
		{"allow: a clean closing message", nil, subagentStop("a1", "Report written to the scratch file.", false), 0},
		{"allow: an agent that handed back", handback("a1", "Clean report."), subagentStop("a1", tell, false), 0},
		{"block: another agent handed back", handback("a2", "Clean report."), subagentStop("a1", tell, false), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			if tc.before != nil {
				k.run("guard-handback", tc.before).Allowed(t)
			}
			r := k.run("subagent-slop", tc.stop)
			r.Want(t, tc.status)
			if tc.status == 2 {
				r.Has(t, "I hope this helps", "slop-patterns")
			}
		})
	}
}
