package hooks

import (
	"strconv"
	"strings"
	"testing"
)

// reply-length on the real kit.yml: chat 40, explain 80.
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
		return "Your last reply ran " + strconv.Itoa(n) + " words against a " + strconv.Itoa(limit) + "-word ceiling"
	}
	words := func(n int) string { return strings.TrimSpace(strings.Repeat("word ", n)) }

	t.Run("an explain trigger raises the ceiling; a detail trigger lifts it", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		prompt(k, "Why does this fail?").Has(t, "80 words")
		stop(t, k, words(81), false)
		prompt(k, "walk me through it").Has(t, over(81, 80))
		stop(t, k, words(500), false)
		prompt(k, "go").Lacks(t, "Your last reply")
	})
	t.Run("fenced code and table pipes don't count", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		prompt(k, "go")
		stop(t, k, words(30)+"\n| a | b |\n|---|---|\n```go\n"+words(200)+"\n```\n", false)
		prompt(k, "go").Lacks(t, "Your last reply")
	})
	t.Run("/write sets its kind's ceiling, code included", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		prompt(k, "/write commit").Has(t, "40 words, code included")
		stop(t, k, "```text\n"+words(41)+"\n```", false)
		prompt(k, "go").Has(t, over(41, 40))
	})
	t.Run("an overrun before another hook's block still counts after a short continuation", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		prompt(k, "go")
		stop(t, k, words(60), false)
		stop(t, k, words(10), true)
		prompt(k, "go").Has(t, over(60, 40))
	})
	t.Run("a long continuation after another Stop hook's block counts", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		prompt(k, "go")
		stop(t, k, words(10), false)
		stop(t, k, words(41), true)
		prompt(k, "go").Has(t, over(41, 40))
	})
	t.Run("a turn with no prompt of its own gets the chat ceiling", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		prompt(k, "/write commit")
		stop(t, k, words(30), false)
		// A task notification's turn: no UserPromptSubmit, code not counted.
		stop(t, k, words(30)+"\n```\n"+words(50)+"\n```", false)
		prompt(k, "go").Lacks(t, "Your last reply")
		stop(t, k, words(30), false)
		stop(t, k, words(41), false)
		prompt(k, "go").Has(t, over(41, 40))
	})
	t.Run("a prompt queued before the last turn's Stop keeps its own ceiling", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		prompt(k, "go")
		prompt(k, "why did it stall?").Has(t, "80 words")
		// The previous turn's continuation Stop arrives after the new prompt.
		stop(t, k, words(300), true)
		stop(t, k, words(81), false)
		prompt(k, "go").Has(t, over(81, 80))
	})
	t.Run("a detail trigger lifts a /write ceiling too", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		prompt(k, "/write explainer the cache in detail").Empty(t)
		stop(t, k, words(500), false)
		prompt(k, "go").Lacks(t, "Your last reply")
	})
	t.Run("with no session id it does nothing", func(t *testing.T) {
		t.Parallel()
		r := newSandbox(t).run("reply-length", map[string]any{"hook_event_name": "Stop", "last_assistant_message": words(500)})
		r.Want(t, 0)
		r.Empty(t)
	})
	t.Run("an uncapped command's turn has no ceiling", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		for _, p := range []string{"/verify this branch", "/code-review high", "/kit:audit perf"} {
			prompt(k, p).Empty(t)
			stop(t, k, words(500), false)
		}
	})
	t.Run("an unclosed fence doesn't hide the prose after it; a tilde fence is code", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		prompt(k, "go")
		stop(t, k, words(30)+"\n~~~\n"+words(200)+"\n~~~\n", false)
		prompt(k, "go").Lacks(t, "Your last reply")
		stop(t, k, words(30)+"\n```\n"+words(20), false)
		prompt(k, "go").Has(t, over(50, 40))
	})
	// The chat ceiling holds when what looks like a trigger isn't one.
	for name, text := range map[string]string{
		"a slash command's name isn't a trigger": "/security-review high",
		"a hyphenated compound isn't a trigger":  "is it a why-not case",
		"a trigger matches whole words only":     "the reviewer said so",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			prompt(newSandbox(t), text).Has(t, "40 words")
		})
	}
}
