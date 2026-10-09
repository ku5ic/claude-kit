package e2e

import (
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
	t.Run("a trigger matches whole words only", func(t *testing.T) {
		k := New(t)
		prompt(k, "the reviewer said so").Has(t, "40 words")
	})
}
