package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
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

// writeCommand is a typed write-* skill and its kind; command is any
// leading slash command and its name, which isn't free text for the
// triggers.
var (
	writeCommand = regexp.MustCompile(`^\s*/(?:kit:)?write-([a-z-]+)`)
	command      = regexp.MustCompile(`^\s*/(\S+)`)
)

// turn is what UserPromptSubmit records for the Stops that follow: the
// target and whether code counts; Stop adds whether it judged the turn and,
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

// passTarget is how far a reply may run past its target, and why.
const passTarget = "Pass it only when the extra words make the reply clearer or a fact correct, up to %d"

// checkClaims stands in for verify-claims on chat replies: said before the
// reply, since a check at Stop would show the reply twice.
const checkClaims = "In this repo, a claim stands only when a tool checked it this session; any other is cut, never labelled (skills/kit-rules/reference/evidence.md section 1)."

// ReplyLength holds chat replies to the reply_length mode's word targets
// (skills/kit-rules/reference/output.md section 0), silently: on UserPromptSubmit it
// picks the turn's target from the prompt and records it. In context the
// user never sees, it says the turn's target and its allowance, the chat one
// too, and a correction when the last reply ran past its allowance. It also carries checkClaims, unless claims is in
// disabled_rules. At Stop it only measures: a Stop runs after the reply is
// on screen, so a block would show it twice.
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
	switch p.String(hook.KeyHookEventName) {
	case "UserPromptSubmit":
		last := readTurn(marker, turn{})
		t := turnLimit(l, p.String(hook.KeyPrompt))
		writeTurn(marker, t)
		var lines []string
		if last.OverWords > 0 {
			lines = append(lines, fmt.Sprintf("The last reply ran %d words against a %d-word target, past its %d-word allowance (skills/kit-rules/reference/output.md section 0).", last.OverWords, last.OverLimit, l.Allowance(last.OverLimit)))
		}
		// The chat ceiling too, though SessionStart states it: deep in a
		// session that line is far from the reply.
		switch {
		case t.Limit > 0:
			counted, mode := "fenced code excluded", " ("+l.Mode+")"
			if t.All {
				counted, mode = "code included", ""
			}
			lines = append(lines, fmt.Sprintf("Reply target this turn: %d words%s, %s. "+passTarget+" (skills/kit-rules/reference/output.md section 0).", t.Limit, mode, counted, l.Allowance(t.Limit)))
		case l.Chat > 0:
			lines = append(lines, "No reply target this turn.")
		}
		if !h.RuleDisabled("claims") {
			lines = append(lines, checkClaims)
		}
		// Each line is its own context, so each gets the label.
		for _, line := range lines {
			h.AddContext("UserPromptSubmit", line)
		}
	case "Stop":
		t := readTurn(marker, turn{Limit: l.Chat})
		active := p.Bool(hook.KeyStopHookActive)
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
		if n := countWords(p.String(hook.KeyLastAssistantMessage), t.All); t.Limit > 0 && n > l.Allowance(t.Limit) {
			t.OverWords, t.OverLimit = n, t.Limit
		}
		writeTurn(marker, t)
	}
	return nil
}

// chatCeiling is the target a turn has unless its prompt sets another,
// said once at SessionStart; "" when chat replies have none.
func chatCeiling(l config.ReplyLimits) string {
	if l.Chat <= 0 {
		return ""
	}
	return fmt.Sprintf("Reply target: %d words (%s), fenced code excluded, on every turn whose context names no other. "+passTarget+" (skills/kit-rules/reference/output.md section 0).\n", l.Chat, l.Mode, l.Allowance(l.Chat))
}

// turnLimit is what a prompt sets: none for an uncapped command or after a
// detail trigger; a write-* skill's ceiling, code included; the explain
// ceiling after an explain trigger; else the chat one. A leading slash
// command's name isn't a trigger.
func turnLimit(l config.ReplyLimits, prompt string) turn {
	if m := command.FindStringSubmatch(prompt); m != nil && uncapped(l.UncappedCommands, m[1]) {
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

// uncapped is whether name, as typed or without its kit: prefix, matches
// one of the patterns, each a name or a glob such as audit-*.
func uncapped(patterns []string, name string) bool {
	bare := strings.TrimPrefix(name, "kit:")
	return slices.ContainsFunc(patterns, func(p string) bool {
		typed, _ := path.Match(p, name)
		short, _ := path.Match(p, bare)
		return typed || short
	})
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
