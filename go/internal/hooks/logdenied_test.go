package hooks

import (
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestLogDenied(t *testing.T) {
	t.Parallel()
	t.Run("a denial is logged by its tool, its input left out, with no answer", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		r := k.run("log-denied", map[string]any{"hook_event_name": "PermissionDenied", "session_id": "s1", "agent_id": "a1", "agent_type": "kit:tester",
			"tool_name": "Bash", "tool_input": map[string]any{"command": "rm -rf /tmp/secret-name"}, "reason": "[Irreversible Local Destruction]"})
		r.Want(t, 0)
		r.Empty(t)
		lines := testutil.JSONLines(t, k.paths.LogFile(kitlog.Guards))
		if len(lines) != 1 {
			t.Fatalf("%d log lines, want 1", len(lines))
		}
		for key, want := range map[string]any{"event": "auto-denied", "session_id": "s1", "agent_id": "a1", "agent_type": "kit:tester", "tool": "Bash"} {
			if lines[0][key] != want {
				t.Errorf("%s = %v, want %v", key, lines[0][key], want)
			}
		}
		if log := read(t, k.paths.LogFile(kitlog.Guards)); strings.Contains(log, "secret-name") {
			t.Errorf("the log holds the command:\n%s", log)
		}
	})
	t.Run("a payload that can't be read, or names no tool, logs nothing", func(t *testing.T) {
		t.Parallel()
		for _, payload := range []any{"{not json", map[string]any{"hook_event_name": "PermissionDenied", "session_id": "s1"}} {
			k := newSandbox(t)
			r := k.run("log-denied", payload)
			r.Want(t, 0)
			r.Empty(t)
			if lines := testutil.JSONLines(t, k.paths.LogFile(kitlog.Guards)); lines != nil {
				t.Errorf("logged %v", lines)
			}
		}
	})
}
