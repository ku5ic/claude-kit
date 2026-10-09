package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// reply-length sets each turn's word ceiling from kit.yml reply_limits on
// UserPromptSubmit and blocks a longer final reply at Stop. These run on the
// real kit.yml: chat 40, explain 80.
func TestReplyLength(t *testing.T) {
	prompt := func(k *Kit, text string) Result {
		return k.Hook("reply-length", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "prompt": text})
	}
	stop := func(k *Kit, reply string, active bool) Result {
		return k.Hook("reply-length", map[string]any{"hook_event_name": "Stop", "session_id": "s1",
			"last_assistant_message": reply, "stop_hook_active": active})
	}
	words := func(n int) string { return strings.TrimSpace(strings.Repeat("word ", n)) }

	t.Run("a plain prompt gets the chat ceiling, and a longer reply is blocked once", func(t *testing.T) {
		k := New(t)
		prompt(k, "fix the bug").Has(t, "Reply ceiling this turn: 40 words")
		stop(k, words(40), false).Want(t, 0)
		r := stop(k, words(41), false)
		r.Want(t, 2)
		r.Has(t, "your reply was 41 words; this turn's ceiling is 40")
		stop(k, words(41), true).Want(t, 0)
	})
	t.Run("an explain trigger raises the ceiling; a detail trigger lifts it", func(t *testing.T) {
		k := New(t)
		prompt(k, "Why does this fail?").Has(t, "80 words")
		stop(k, words(80), false).Want(t, 0)
		stop(k, words(81), false).Want(t, 2)
		prompt(k, "walk me through it").Empty(t)
		stop(k, words(500), false).Want(t, 0)
	})
	t.Run("fenced code and table pipes don't count", func(t *testing.T) {
		k := New(t)
		prompt(k, "go")
		reply := words(30) + "\n| a | b |\n|---|---|\n```go\n" + words(200) + "\n```\n"
		stop(k, reply, false).Want(t, 0)
	})
	t.Run("/write sets its kind's ceiling, code included", func(t *testing.T) {
		k := New(t)
		prompt(k, "/write commit").Has(t, "40 words, code included")
		stop(k, "```text\n"+words(41)+"\n```", false).Want(t, 2)
	})
	written := func(k *Kit, name, body string) Result {
		path := filepath.Join(k.Home, "proj/.claude/scratch", name)
		Write(t, path, body)
		return k.Hook("reply-length", map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1",
			"tool_name": "Write", "tool_input": map[string]any{"file_path": path}})
	}
	t.Run("a /write file over its kind's ceiling is sent back", func(t *testing.T) {
		k := New(t)
		written(k, "pr-x-20261009-1200.md", words(80)).Want(t, 0)
		r := written(k, "pr-x-20261009-1200.md", words(81))
		r.Want(t, 2)
		r.Has(t, "pr-x-20261009-1200.md is 81 words; the /write pr ceiling is 80")
		r = written(k, "review-comment-y-20261009-1200.md", words(41))
		r.Has(t, "the /write review-comment ceiling is 40")
	})
	t.Run("a report finding over report_finding is named; short ones and other files pass", func(t *testing.T) {
		k := New(t)
		report := "# R\n\n## Summary\n\n" + words(100) + "\n\n## Findings\n\n### Short\n\n" + words(20) +
			"\n\n### Long one\n\n" + words(51) + "\n```go\n" + words(100) + "\n```\n\n## Out of scope\n\n" + words(100) + "\n"
		r := written(k, "audit-x-20261009-1200.md", report)
		r.Want(t, 2)
		r.Has(t, `"Long one" (53)`)
		if strings.Contains(r.Output, "Short") {
			t.Errorf("short finding named:\n%s", r.Output)
		}
		written(k, "notes.md", words(500)).Want(t, 0)
	})
	t.Run("a file outside scratch is never checked", func(t *testing.T) {
		k := New(t)
		path := filepath.Join(k.Home, "proj/pr-x.md")
		Write(t, path, words(500))
		k.Hook("reply-length", map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1",
			"tool_name": "Write", "tool_input": map[string]any{"file_path": path}}).Want(t, 0)
	})
	t.Run("a trigger matches whole words only", func(t *testing.T) {
		k := New(t)
		prompt(k, "the reviewer said so").Has(t, "40 words")
	})
}
