package hooks

import (
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
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
		r.Has(t, `The last reply held AI tells: \"I hope this helps\" -> cut it`, "A synonym keeps the tell", "slop-patterns")
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
		{"a dense tier 3 passage", dense},
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
		k.run("chat-slop", chatStop(dense+" "+tell))
		log := read(t, k.paths.LogFile(kitlog.Guards))
		for _, want := range []string{"slop-vocabulary", "slop-hope-helps", "chat reply"} {
			if !strings.Contains(log, want) {
				t.Errorf("guards log lacks %s:\n%s", want, log)
			}
		}
	})
}

// A reply is on screen before Stop runs, so a reference in it that doesn't
// resolve is corrected on the next prompt, in context the user never sees.
func TestChatClaims(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	dir := repo(t, t.TempDir())
	testutil.Put(t, dir, "src/stop.go", "a\nb\n")
	reply := func(text string) map[string]any {
		p := chatStop(text)
		p["cwd"] = dir
		return p
	}
	k.overlay("claims:\n  timeout: 240\n")
	k.run("chat-claims", reply("See `src/stop.go:40`.")).Empty(t)
	r := k.run("chat-claims", chatPrompt)
	r.Has(t, "src/stop.go:40", "Open your next reply with the corrected fact")
	r.Lacks(t, "systemMessage")
	k.run("chat-claims", chatPrompt).Empty(t)
	k.run("chat-claims", reply("See `src/stop.go:2`.")).Empty(t)
	k.run("chat-claims", chatPrompt).Empty(t)
}
