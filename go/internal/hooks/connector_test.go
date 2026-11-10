package hooks

import (
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

func TestGuardConnector(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		call   map[string]any
		status int
	}{
		{"block: a tell in a message sent", toolCall("mcp__chat__send_message", map[string]any{"channel": "C1", "text": tell}), 2},
		{"block: a tell nested in a page's blocks", toolCall("mcp__wiki__create-pages", map[string]any{
			"pages": []any{map[string]any{"title": "Notes", "body": "Intro.\n\n" + tell}},
		}), 2},
		{"block: a pr-surface rule", toolCall("mcp__tracker__createComment", map[string]any{"body": "Addressed feedback from review."}), 2},
		{"allow: a clean message", toolCall("mcp__chat__send_message", map[string]any{"text": "The deploy failed at the migration step."}), 0},
		{"allow: a read with a tell in its query", toolCall("mcp__chat__search_messages", map[string]any{"query": tell}), 0},
		{"allow: a tool that is neither", toolCall("mcp__browser__navigate", map[string]any{"url": "https://example.com", "note": tell}), 0},
		{"allow: a quoted tell", toolCall("mcp__chat__send_message", map[string]any{"text": `Flagged "I hope this helps" in the README.`}), 0},
		{"allow: no text", toolCall("mcp__tracker__delete_task", map[string]any{"id": "123"}), 0},
		{"allow: a tell in a code field", toolCall("mcp__claude_ai_Figma__use_figma", map[string]any{"code": "// " + tell, "description": "Rename the frame."}), 0},
		{"block: a tell beside a code field", toolCall("mcp__claude_ai_Figma__use_figma", map[string]any{"code": "figma.root", "description": tell}), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newSandbox(t).run("guard-connector", tc.call)
			r.Want(t, tc.status)
			if tc.status == 2 {
				r.Has(t, "unslop:", "input")
			}
		})
	}
	t.Run("a dense tier 3 message goes through and is logged", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.run("guard-connector", toolCall("mcp__chat__send_message", map[string]any{"text": dense})).Want(t, 0)
		if log := read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, "slop-vocabulary") || !strings.Contains(log, "mcp__chat__send_message") {
			t.Errorf("guards log lacks the hit:\n%s", log)
		}
	})
	t.Run("slop in disabled_rules lets it through", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.overlay("disabled_rules: [slop]\n")
		k.run("guard-connector", toolCall("mcp__chat__send_message", map[string]any{"text": tell})).Want(t, 0)
	})
}
