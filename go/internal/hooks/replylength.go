package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/hook"
)

// ReplyLength holds chat replies to the word ceilings in kit.yml
// reply_limits (rules/output.md section 0). On UserPromptSubmit it picks the
// turn's ceiling from the prompt, records it, and says it; at Stop it blocks
// a final reply over it, once.
func ReplyLength(h *hook.Hook) error {
	p := h.Payload
	cfg := h.Config()
	if p.Err != nil || cfg == nil {
		return nil
	}
	marker := h.Paths.SessionFile(config.ReplyLimit, p.SessionID())
	switch p.String("hook_event_name") {
	case "UserPromptSubmit":
		limit := turnLimit(cfg.ReplyLimits, p.String("prompt"))
		if marker != "" && os.MkdirAll(filepath.Dir(marker), 0o755) == nil {
			_ = os.WriteFile(marker, []byte(strconv.Itoa(limit)), 0o644)
		}
		if limit > 0 {
			hook.AddContext(h.Stdout, "UserPromptSubmit", "", fmt.Sprintf("Reply ceiling this turn: %d words, fenced code excluded (rules/output.md section 0).", limit))
		}
	case "Stop":
		if p.Bool("stop_hook_active") {
			return nil
		}
		limit := cfg.ReplyLimits.Chat
		if data, err := os.ReadFile(marker); err == nil {
			limit, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
		if n := proseWords(p.String("last_assistant_message")); limit > 0 && n > limit {
			return h.Block(fmt.Sprintf("your reply was %d words; this turn's ceiling is %d (rules/output.md section 0). "+
				"Send only the cut version, at most %d words: the answer or next action first, nothing the reader already saw.", n, limit, limit), "reply-length")
		}
	}
	return nil
}

// turnLimit is the ceiling a prompt sets: none after a detail trigger, the
// explain ceiling after an explain trigger, else the chat one.
func turnLimit(l config.ReplyLimits, prompt string) int {
	switch {
	case hasTrigger(prompt, l.DetailTriggers):
		return 0
	case hasTrigger(prompt, l.ExplainTriggers):
		return l.Explain
	}
	return l.Chat
}

func hasTrigger(prompt string, triggers []string) bool {
	for _, t := range triggers {
		if regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(t) + `\b`).MatchString(prompt) {
			return true
		}
	}
	return false
}

// proseWords counts the words outside fenced code blocks. A token with no
// letter or digit (a table's pipes, a bullet, a rule) isn't a word.
func proseWords(text string) int {
	n, fenced := 0, false
	for line := range strings.Lines(text) {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, f := range strings.Fields(line) {
			if strings.IndexFunc(f, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
				n++
			}
		}
	}
	return n
}
