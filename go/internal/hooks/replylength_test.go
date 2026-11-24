package hooks

import (
	"strconv"
	"strings"
	"testing"
)

// reply-length on the real kit.yml: terse, chat 40 and explain 150. capped
// takes its 50% slack away, so a reply one word over is flagged.
func TestReplyLength(t *testing.T) {
	t.Parallel()
	prompt := func(k *sandbox, text string) result {
		return k.run("reply-length", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "prompt": text})
	}
	// stop never blocks and never prints: the reply is already on screen.
	stop := func(t *testing.T, k *sandbox, reply string, active bool) {
		t.Helper()
		r := k.run("reply-length", map[string]any{"hook_event_name": "Stop", "session_id": "s1",
			"last_assistant_message": reply, "stop_hook_active": active})
		r.Want(t, 0)
		r.Empty(t)
	}
	over := func(n, limit int) string {
		return "The last reply ran " + strconv.Itoa(n) + " words against a " + strconv.Itoa(limit) + "-word target"
	}
	words := func(n int) string { return strings.TrimSpace(strings.Repeat("word ", n)) }
	capped := func(t *testing.T) *sandbox {
		k := newSandbox(t)
		k.overlay("reply_slack: none\n")
		return k
	}

	t.Run("a reply within the slack isn't flagged, and one past it is", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		prompt(k, "go").Has(t, "Reply target this turn: 40 words (terse)", "clearer or a fact correct, up to 60")
		stop(t, k, words(60), false)
		prompt(k, "go").Lacks(t, "The last reply")
		stop(t, k, words(61), false)
		// The note names the allowance Stop judged by, not one set since.
		k.overlay("reply_slack: 100%\n")
		prompt(k, "go").Has(t, over(61, 40)+", past its 60-word allowance")
	})

	t.Run("an explain trigger raises the target; a detail trigger lifts it", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		prompt(k, "Why does this fail?").Has(t, "150 words")
		stop(t, k, words(151), false)
		prompt(k, "walk me through it").Has(t, over(151, 150))
		stop(t, k, words(500), false)
		prompt(k, "go").Lacks(t, "The last reply")
	})
	t.Run("asking to understand is an explain trigger", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		for _, p := range []string{"Can you elaborate on those?", "What's your honest take?"} {
			prompt(k, p).Has(t, "150 words")
		}
		r := prompt(k, "I understand, ship it.")
		r.Has(t, "Reply target this turn: 40 words")
		r.Lacks(t, "150 words")
	})
	t.Run("asking an opinion raises the target, asking for depth or !long lifts it, a longer word holding a trigger does neither", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		for p, want := range map[string]string{
			"What's your opinion on this?":    "150 words",
			"Give me a deep dive on the gate": "No reply target this turn",
			"!long what changed":              "No reply target this turn",
			"thoroughness matters here":       "Reply target this turn: 40 words",
			"improve this function":           "Reply target this turn: 40 words",
		} {
			prompt(k, p).Has(t, want)
		}
	})
	t.Run("fenced code and table pipes don't count", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		prompt(k, "go")
		stop(t, k, words(30)+"\n| a | b |\n|---|---|\n```go\n"+words(200)+"\n```\n", false)
		prompt(k, "go").Lacks(t, "The last reply")
	})
	t.Run("a write-* skill sets its target, code included", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		prompt(k, "/write-commit").Has(t, "40 words, code included")
		stop(t, k, "```text\n"+words(41)+"\n```", false)
		prompt(k, "go").Has(t, over(41, 40))
	})
	t.Run("an overrun before another hook's block still counts after a short continuation", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		prompt(k, "go")
		stop(t, k, words(60), false)
		stop(t, k, words(10), true)
		prompt(k, "go").Has(t, over(60, 40))
	})
	t.Run("a long continuation after another Stop hook's block counts", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		prompt(k, "go")
		stop(t, k, words(10), false)
		stop(t, k, words(41), true)
		prompt(k, "go").Has(t, over(41, 40))
	})
	t.Run("a turn with no prompt of its own gets the chat target", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		prompt(k, "/write-commit")
		stop(t, k, words(30), false)
		// A task notification's turn: no UserPromptSubmit, code not counted.
		stop(t, k, words(30)+"\n```\n"+words(50)+"\n```", false)
		prompt(k, "go").Lacks(t, "The last reply")
		stop(t, k, words(30), false)
		stop(t, k, words(41), false)
		prompt(k, "go").Has(t, over(41, 40))
	})
	t.Run("a prompt queued before the last turn's Stop keeps its own target", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		prompt(k, "go")
		prompt(k, "why did it stall?").Has(t, "150 words")
		// The previous turn's continuation Stop arrives after the new prompt.
		stop(t, k, words(300), true)
		stop(t, k, words(151), false)
		prompt(k, "go").Has(t, over(151, 150))
	})
	t.Run("a detail trigger lifts a write-* target too", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		prompt(k, "/write-explainer the cache in detail").Has(t, "No reply target this turn")
		stop(t, k, words(500), false)
		prompt(k, "go").Lacks(t, "The last reply")
	})
	t.Run("with no session id it does nothing", func(t *testing.T) {
		t.Parallel()
		r := capped(t).run("reply-length", map[string]any{"hook_event_name": "Stop", "last_assistant_message": words(500)})
		r.Want(t, 0)
		r.Empty(t)
	})
	t.Run("an uncapped command's turn has no target", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		for _, p := range []string{"/verify this branch", "/code-review high", "/audit-perf", "/kit:audit-perf src", "/kit:deps"} {
			prompt(k, p).Has(t, "No reply target this turn")
			stop(t, k, words(500), false)
		}
	})
	t.Run("every prompt asks for checked claims", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		for _, p := range []string{"go", "/verify this branch", "explain the cache in detail"} {
			prompt(k, p).Has(t, "In this project, a claim stands only when you verified it against the code or source itself, so what you read this session shows it", "running a tool isn't verification, and anything assumed is cut", "skills/kit-rules/reference/evidence.md section 1")
		}
	})
	t.Run("claims in disabled_rules drops the claims line", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		k.overlay("disabled_rules: [claims]\n")
		prompt(k, "/verify this branch").Lacks(t, "a claim stands")
		r := prompt(k, "go")
		r.Has(t, "Reply target this turn: 40 words")
		r.Lacks(t, "a claim stands")
	})
	t.Run("an unclosed fence doesn't hide the prose after it; a tilde fence is code", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		prompt(k, "go")
		stop(t, k, words(30)+"\n~~~\n"+words(200)+"\n~~~\n", false)
		prompt(k, "go").Lacks(t, "The last reply")
		stop(t, k, words(30)+"\n```\n"+words(20), false)
		prompt(k, "go").Has(t, over(50, 40))
	})
	t.Run("every prompt states the chat target, an overrun's prompt too", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		r := prompt(k, "go")
		r.Has(t, "Reply target this turn: 40 words (terse), fenced code excluded")
		r.Lacks(t, "No reply target")
		stop(t, k, words(41), false)
		prompt(k, "go").Has(t, over(41, 40), "Reply target this turn: 40 words")
	})
	t.Run("explanatory mode states no target, a why question included", func(t *testing.T) {
		t.Parallel()
		k := capped(t)
		k.overlay("reply_length: explanatory\n")
		prompt(k, "go").Lacks(t, "Reply target", "No reply target")
		prompt(k, "Why does this fail?").Lacks(t, "Reply target", "No reply target")
	})
	// The chat target holds when what looks like a trigger isn't one.
	for name, text := range map[string]string{
		"a slash command's name isn't a trigger": "/security-review high",
		"a hyphenated compound isn't a trigger":  "is it a why-not case",
		"a trigger matches whole words only":     "the reviewer said so",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := prompt(capped(t), text)
			r.Has(t, "Reply target this turn: 40 words")
			r.Lacks(t, "150 words")
		})
	}
}
