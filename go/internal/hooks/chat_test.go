package hooks

import (
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

func chatStop(reply string) map[string]any {
	return map[string]any{"hook_event_name": "Stop", "session_id": "s", "last_assistant_message": reply}
}

var chatPrompt = map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt": "next"}

func TestChatSlop(t *testing.T) {
	t.Parallel()
	t.Run("a tell in a reply is named on the next prompt, once", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.run("chat-slop", chatStop("Done. "+tell)).Empty(t)
		r := k.run("chat-slop", chatPrompt)
		r.Want(t, 0)
		r.Has(t, `\"I hope this helps\" -> cut it`, "slop-patterns")
		k.run("chat-slop", chatPrompt).Empty(t)
	})
	t.Run("tells from every Stop before the prompt are named, each once", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.run("chat-slop", chatStop(tell))
		k.run("chat-slop", chatStop("You're absolutely right. "+tell))
		r := k.run("chat-slop", chatPrompt)
		r.Has(t, "absolutely right")
		if n := strings.Count(r.Output, "I hope this helps"); n != 1 {
			t.Errorf("named %d times, want 1:\n%s", n, r.Output)
		}
	})
	for _, tc := range []struct{ name, reply string }{
		{"a clean reply", "The parser drops the last line; fixed in parse.go."},
		{"a quoted tell", `The rule bans "I hope this helps".`},
		{"a tell in a code span", "Matches `I hope this helps`."},
		{"a tier 2 tell", "It serves as the entry point."},
	} {
		t.Run("nothing to correct after "+tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			k.run("chat-slop", chatStop(tc.reply)).Empty(t)
			k.run("chat-slop", chatPrompt).Empty(t)
		})
	}
	t.Run("every hit is logged", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.run("chat-slop", chatStop("It serves as the entry point. "+tell))
		log := read(t, k.paths.LogFile(kitlog.Guards))
		for _, want := range []string{"slop-copula", "slop-hope-helps", "chat reply"} {
			if !strings.Contains(log, want) {
				t.Errorf("guards log lacks %s:\n%s", want, log)
			}
		}
	})
}
