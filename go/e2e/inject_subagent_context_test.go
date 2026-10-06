package e2e

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectSubagentContext(t *testing.T) {
	t.Run("wraps the context in hookSpecificOutput, the only shape SubagentStart reads", func(t *testing.T) {
		k := New(t)
		proj := filepath.Join(t.TempDir(), "testproject")
		Mkdir(t, proj)
		k.Git(proj, "init", "-q")

		r := k.Hook("inject-subagent-context", map[string]any{"session_id": "s1", "cwd": proj})
		r.Want(t, 0)

		var out struct {
			HookSpecificOutput struct {
				HookEventName     string `json:"hookEventName"`
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, r.Stdout)
		}
		if got := out.HookSpecificOutput.HookEventName; got != "SubagentStart" {
			t.Errorf("hookEventName = %q, want SubagentStart\n%s", got, r.Stdout)
		}
		if ctx := out.HookSpecificOutput.AdditionalContext; !strings.Contains(ctx, "<scratch>") {
			t.Errorf("additionalContext lacks the <scratch> block\n%s", r.Stdout)
		}
	})
}
