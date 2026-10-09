package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/hook"
)

// writeCommand is a typed /write and its kind; command is any leading
// slash command and its name, which isn't free text for the triggers.
var (
	writeCommand = regexp.MustCompile(`^\s*/(?:kit:)?write\s+([a-z-]+)`)
	command      = regexp.MustCompile(`^\s*/(\S+)`)
)

// turn is what UserPromptSubmit records for the Stops that follow: the
// ceiling, whether code counts, and how far Stop has judged it.
type turn struct {
	Limit int    `json:"limit"`
	All   bool   `json:"all"`
	State string `json:"state"` // "", then "passed" or "blocked", then "released"
}

func writeTurn(marker string, t turn) {
	if data, err := json.Marshal(t); err == nil && marker != "" && os.MkdirAll(filepath.Dir(marker), 0o755) == nil {
		_ = os.WriteFile(marker, data, 0o644)
	}
}

// ReplyLength holds chat replies to the word ceilings in kit.yml
// reply_limits (rules/output.md section 0). On UserPromptSubmit it picks the
// turn's ceiling from the prompt, records it, and says it; at Stop it blocks
// a final reply over it, once per turn.
func ReplyLength(h *hook.Hook) error {
	p := h.Payload
	cfg := h.Config()
	if p.Err != nil || cfg == nil {
		return nil
	}
	l := cfg.ReplyLimits
	marker := h.Paths.SessionFile(config.ReplyLimit, p.SessionID())
	switch p.String("hook_event_name") {
	case "UserPromptSubmit":
		t := turnLimit(l, p.String("prompt"))
		writeTurn(marker, t)
		if t.Limit > 0 {
			counted := "fenced code excluded"
			if t.All {
				counted = "code included"
			}
			hook.AddContext(h.Stdout, "UserPromptSubmit", "", fmt.Sprintf("Reply ceiling this turn: %d words, %s (rules/output.md section 0).", t.Limit, counted))
		}
	case "Stop":
		// Its own state, not stop_hook_active: another Stop hook's block sets
		// that too, and a long reply after it still counts. After its own
		// block the turn is released; a turn with no prompt of its own (a task
		// notification) gets the chat ceiling.
		t := turn{Limit: l.Chat}
		if data, err := os.ReadFile(marker); err == nil {
			_ = json.Unmarshal(data, &t)
		}
		active := p.Bool("stop_hook_active")
		switch {
		case marker == "":
			// No session to keep "once per turn" in: never block, never loop.
			return nil
		case t.State == "" && active:
			// A continuation before this turn's first Stop belongs to the
			// turn before a queued prompt: leave the new turn's marker alone.
			return nil
		case t.State == "blocked":
			t.State = "released"
			writeTurn(marker, t)
			return nil
		case t.State == "released" && active:
			return nil
		case t.State != "" && !active:
			t = turn{Limit: l.Chat}
		}
		n := countWords(p.String("last_assistant_message"), t.All)
		t.State = "passed"
		if t.Limit > 0 && n > t.Limit {
			t.State = "blocked"
		}
		writeTurn(marker, t)
		if t.State == "blocked" {
			return h.Block(fmt.Sprintf("your reply was %d words; this turn's ceiling is %d (rules/output.md section 0). "+
				"Send only the cut version, at most %d words: the answer or next action first, nothing the reader already saw.", n, t.Limit, t.Limit), "reply-length")
		}
	}
	return nil
}

// turnLimit is what a prompt sets: none for an uncapped command or after a
// detail trigger; a /write kind's ceiling, code included; the explain
// ceiling after an explain trigger; else the chat one. A leading slash
// command's name isn't a trigger.
func turnLimit(l config.ReplyLimits, prompt string) turn {
	if m := command.FindStringSubmatch(prompt); m != nil && slices.Contains(l.UncappedCommands, m[1]) {
		return turn{}
	}
	text := command.ReplaceAllString(prompt, "")
	if hasTrigger(text, l.DetailTriggers) {
		return turn{}
	}
	if m := writeCommand.FindStringSubmatch(prompt); m != nil {
		if n, ok := l.Write[m[1]]; ok {
			return turn{Limit: n, All: true}
		}
	}
	if hasTrigger(text, l.ExplainTriggers) {
		return turn{Limit: l.Explain}
	}
	return turn{Limit: l.Chat}
}

func hasTrigger(text string, triggers []string) bool {
	if len(triggers) == 0 {
		return false
	}
	quoted := make([]string, len(triggers))
	for i, t := range triggers {
		quoted[i] = regexp.QuoteMeta(t)
	}
	// A hyphen joins a compound ("why-not"), so it bounds a trigger like a letter.
	return regexp.MustCompile(`(?i)(?:^|[^\w-])(?:` + strings.Join(quoted, "|") + `)(?:$|[^\w-])`).MatchString(text)
}

// countWords counts the words in text, fenced code only when all. A token
// with no letter or digit (a table's pipes, a bullet, a rule) isn't a word.
// A fence closes only on its own marker; one never closed was prose.
func countWords(text string, all bool) int {
	n, inFence := 0, 0
	fence := ""
	for line := range strings.Lines(text) {
		if trimmed := strings.TrimSpace(line); fence == "" && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
			fence = trimmed[:3]
			continue
		} else if fence != "" && strings.HasPrefix(trimmed, fence) {
			fence, inFence = "", 0
			continue
		}
		words := 0
		for _, f := range strings.Fields(line) {
			if strings.IndexFunc(f, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
				words++
			}
		}
		if fence != "" && !all {
			inFence += words
			continue
		}
		n += words
	}
	return n + inFence
}
