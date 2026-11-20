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
				r.Has(t, "kit connector: BLOCKED ", "AI tell(s) in the mcp__", "input\n", "rule  slop")
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

// A GitHub tool that doesn't only read blocks on an AI tool's attribution
// in any of its input, code included; other servers' tools don't.
func TestGuardConnectorBlocksAttributionOnGitHub(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		call   map[string]any
		status int
	}{
		{"block: a PR footer", toolCall("mcp__github__create_pull_request", map[string]any{"title": "fix: x", "body": "Fixes it.\n\n\U0001F916 Generated with [Claude Code](https://claude.com/claude-code)"}), 2},
		{"block: a session link in a comment", toolCall("mcp__plugin_github_github__add_issue_comment", map[string]any{"body": "https://claude.ai/code/session_x"}), 2},
		{"block: a trailer in a commit message pushed with files", toolCall("mcp__claude_ai_GitHub__push_files", map[string]any{"message": "fix: x\n\nCo-Authored-By: Claude <noreply@anthropic.com>", "files": []any{map[string]any{"path": "a.go", "content": "package a"}}}), 2},
		{"block: a trailer in a code field", toolCall("mcp__github__create_or_update_file", map[string]any{"message": "fix: x", "code": "Claude-Session: x"}), 2},
		{"allow: a clean PR", toolCall("mcp__github__create_pull_request", map[string]any{"title": "fix: x", "body": "Reads the body from disk."}), 0},
		{"allow: a GitHub read", toolCall("mcp__github__search_issues", map[string]any{"query": "Co-Authored-By: Claude"}), 0},
		{"allow: another server", toolCall("mcp__chat__send_message", map[string]any{"text": "see https://claude.ai/code/session_x"}), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newSandbox(t).run("guard-connector", tc.call)
			r.Want(t, tc.status)
			if tc.status == 2 {
				r.Has(t, "kit connector: BLOCKED AI attribution in the mcp__", "rule  ai-attribution")
			}
		})
	}
}

// A connector's path fields get guard-edit's path rules: a read tool's as
// reads, any other's as writes.
func TestGuardConnectorRunsThePathRules(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, want string
		call       map[string]any
	}{
		{"an MCP read of a key", "reading a credential or key file", toolCall("mcp__fs__read_file", map[string]any{"path": "~/.ssh/id_rsa"})},
		{"an MCP read of a credential among several", "reading a credential or key file", toolCall("mcp__fs__read_multiple_files", map[string]any{"paths": []any{"/tmp/p/a.txt", "/tmp/p/.env"}})},
		{"an MCP write of a credential file", "writing a credential or key file", toolCall("mcp__fs__write_file", map[string]any{"path": "/tmp/p/.env", "content": "K=1"})},
		{"an MCP move onto authorized_keys", "writing a credential or key file", toolCall("mcp__fs__move_file", map[string]any{"source": "/tmp/p/k", "destination": "~/.ssh/authorized_keys"})},
		{"an MCP write into .git", "edit inside .git/", toolCall("mcp__fs__write_file", map[string]any{"path": "/tmp/p/.git/hooks/pre-commit", "content": "exit 0"})},
		{"an MCP read of a plain file", "", toolCall("mcp__fs__read_file", map[string]any{"path": "/tmp/p/README.md"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newSandbox(t).run("guard-connector", tc.call)
			if tc.want == "" {
				r.Want(t, 0)
				return
			}
			r.Want(t, 2)
			r.Has(t, tc.want)
		})
	}
}
