package connector

import (
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

func TestClassify(t *testing.T) {
	t.Parallel()
	verbs := config.Connectors{
		WriteVerbs: []string{"send", "create", "update", "comment", "post"},
		ReadVerbs:  []string{"get", "read", "search", "list"},
	}
	for _, tc := range []struct {
		tool        string
		write, read bool
	}{
		{"mcp__slack__slack_send_message", true, false},
		{"mcp__claude_ai_Notion__notion-create-pages", true, false},
		{"mcp__tracker__createIssue", true, false},
		{"mcp__tracker__get_task_comments", false, true}, // "comments" is no verb
		{"mcp__tracker__create_task_comment", true, false},
		{"mcp__docs__search", false, true},
		{"mcp__crm__get_or_create_contact", true, false}, // a write verb outranks a read verb
		{"mcp__browser__navigate", false, false},
		{"mcp__send__navigate", false, false}, // the server's name is no verb
		{"mcp__a__b__list_items", false, true},
		{"Write", false, false},
		{"SendMessage", false, false},
		{"mcp__noserver", false, false},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			t.Parallel()
			if got := IsWrite(verbs, tc.tool); got != tc.write {
				t.Errorf("IsWrite = %v, want %v", got, tc.write)
			}
			if got := IsRead(verbs, tc.tool); got != tc.read {
				t.Errorf("IsRead = %v, want %v", got, tc.read)
			}
		})
	}
}
