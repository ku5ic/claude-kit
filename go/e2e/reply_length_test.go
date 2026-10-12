package e2e

import (
	"strings"
	"testing"
)

// `kit hook reply-length` wiring, on the real kit.yml (chat 40): the
// ceiling set on UserPromptSubmit, the reply measured at Stop without
// printing, and an overrun corrected in the next prompt's context. The
// ceilings and word counts are tested in process, in internal/hooks.
func TestReplyLength(t *testing.T) {
	t.Parallel()
	t.Run("a reply at the ceiling passes; one over is corrected on the next prompt, once", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		prompt := func(text string) Result {
			return k.Hook("reply-length", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "prompt": text})
		}
		stop := func(words int) {
			t.Helper()
			r := k.Hook("reply-length", map[string]any{"hook_event_name": "Stop", "session_id": "s1",
				"last_assistant_message": strings.TrimSpace(strings.Repeat("word ", words)), "stop_hook_active": false})
			r.Want(t, 0)
			r.Empty(t)
		}
		prompt("fix the bug").Has(t, "Reply ceiling this turn: 40 words")
		stop(40)
		prompt("next").Lacks(t, "Your last reply")
		stop(41)
		prompt("next").Has(t, "Your last reply ran 41 words against a 40-word ceiling", "Reply ceiling this turn: 40 words")
		prompt("again").Lacks(t, "Your last reply")
	})
}
