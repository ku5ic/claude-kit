package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/md"
)

// writeCommand is a typed /write and its kind; command is any leading
// slash command and its name, which isn't free text for the triggers.
var (
	writeCommand = regexp.MustCompile(`^\s*/(?:kit:)?write\s+([a-z-]+)`)
	command      = regexp.MustCompile(`^\s*/(\S+)`)
)

// turn is what UserPromptSubmit records for the Stops that follow: the
// ceiling and whether code counts; Stop adds whether it judged the turn and,
// when the reply ran over, by how much, for the next prompt to correct.
type turn struct {
	Limit     int  `json:"limit"`
	All       bool `json:"all"`
	Judged    bool `json:"judged"`
	OverWords int  `json:"over_words"`
	OverLimit int  `json:"over_limit"`
}

func readTurn(marker string, fallback turn) turn {
	if data, err := os.ReadFile(marker); err == nil {
		_ = json.Unmarshal(data, &fallback)
	}
	return fallback
}

func writeTurn(marker string, t turn) {
	if data, err := json.Marshal(t); err == nil && marker != "" {
		_ = fsx.WriteAtomic(marker, data, 0o644)
	}
}

// ReplyLength holds chat replies to the word ceilings in kit.yml
// reply_limits (rules/output.md section 0), silently: on UserPromptSubmit it
// picks the turn's ceiling from the prompt, records it, and says it in
// context the user never sees, with a correction when the last reply ran
// over. At Stop it only measures: a Stop runs after the reply is on screen,
// so a block would show it twice.
func ReplyLength(h *hook.Hook) error {
	p := h.Payload
	cfg := h.Config()
	if p.Err != nil || cfg == nil {
		return nil
	}
	l := cfg.ReplyLimits
	marker := h.Paths.SessionFile(cache.ReplyLimit, p.SessionID())
	if marker == "" {
		return nil
	}
	switch p.String("hook_event_name") {
	case "UserPromptSubmit":
		last := readTurn(marker, turn{})
		t := turnLimit(l, p.String("prompt"))
		writeTurn(marker, t)
		var lines []string
		if last.OverWords > 0 {
			lines = append(lines, fmt.Sprintf("Your last reply ran %d words against a %d-word ceiling: run section 0's cut test before sending this one.", last.OverWords, last.OverLimit))
		}
		if t.Limit > 0 {
			counted := "fenced code excluded"
			if t.All {
				counted = "code included"
			}
			lines = append(lines, fmt.Sprintf("Reply ceiling this turn: %d words, %s (rules/output.md section 0).", t.Limit, counted))
		}
		if len(lines) > 0 {
			h.AddContext("UserPromptSubmit", strings.Join(lines, "\n"))
		}
	case "Stop":
		t := readTurn(marker, turn{Limit: l.Chat})
		active := p.Bool("stop_hook_active")
		switch {
		case !t.Judged && active:
			// A continuation before this turn's first Stop belongs to the
			// turn before a queued prompt: leave the new turn's marker alone.
			return nil
		case t.Judged && !active:
			// A turn with no prompt of its own (a task notification).
			t.Limit, t.All = l.Chat, false
		}
		// Every reply was shown, so any one over counts; the next prompt
		// reports and clears it.
		t.Judged = true
		if n := countWords(p.String("last_assistant_message"), t.All); t.Limit > 0 && n > t.Limit {
			t.OverWords, t.OverLimit = n, t.Limit
		}
		writeTurn(marker, t)
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
	for l := range md.Lines(text) {
		switch l.Kind {
		case md.FenceOpen:
			continue
		case md.FenceClose:
			inFence = 0
			continue
		}
		words := 0
		for _, f := range strings.Fields(l.Text) {
			if strings.IndexFunc(f, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
				words++
			}
		}
		if l.Kind == md.Code && !all {
			inFence += words
			continue
		}
		n += words
	}
	return n + inFence
}
