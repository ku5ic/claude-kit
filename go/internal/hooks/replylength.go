package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// writeCommand is a typed /write and its kind.
var writeCommand = regexp.MustCompile(`^\s*/(?:kit:)?write\s+([a-z-]+)`)

// ReplyLength holds output to the word ceilings in kit.yml reply_limits
// (rules/output.md section 0). On UserPromptSubmit it picks the turn's
// ceiling from the prompt, records it, and says it; at Stop it blocks a
// final reply over it, once; after a write to scratch it sends back a /write
// file or a report finding over its ceiling.
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
		limit, all := turnLimit(l, p.String("prompt"))
		if marker != "" && os.MkdirAll(filepath.Dir(marker), 0o755) == nil {
			_ = os.WriteFile(marker, []byte(fmt.Sprintf("%d %t", limit, all)), 0o644)
		}
		if limit > 0 {
			counted := "fenced code excluded"
			if all {
				counted = "code included"
			}
			hook.AddContext(h.Stdout, "UserPromptSubmit", "", fmt.Sprintf("Reply ceiling this turn: %d words, %s (rules/output.md section 0).", limit, counted))
		}
	case "Stop":
		if p.Bool("stop_hook_active") {
			return nil
		}
		limit, all := l.Chat, false
		if data, err := os.ReadFile(marker); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) == 2 {
				limit, _ = strconv.Atoi(fields[0])
				all = fields[1] == "true"
			}
		}
		if n := countWords(p.String("last_assistant_message"), all); limit > 0 && n > limit {
			return h.Block(fmt.Sprintf("your reply was %d words; this turn's ceiling is %d (rules/output.md section 0). "+
				"Send only the cut version, at most %d words: the answer or next action first, nothing the reader already saw.", n, limit, limit), "reply-length")
		}
	case "PostToolUse":
		if path := p.FilePath(); project.IsScratch(h.Paths, path) {
			if reason := fileOverLimit(l, path); reason != "" {
				return h.Block(reason+" (kit.yml reply_limits, rules/output.md section 0). Rewrite it shorter: one idea per line, nothing the reader already knows.", "reply-length")
			}
		}
	}
	return nil
}

// turnLimit is the ceiling a prompt sets, and whether code counts toward
// it: a /write kind's, whole output; none after a detail trigger; the
// explain ceiling after an explain trigger; else the chat one.
func turnLimit(l config.ReplyLimits, prompt string) (limit int, all bool) {
	if m := writeCommand.FindStringSubmatch(prompt); m != nil {
		if n, ok := l.Write[m[1]]; ok {
			return n, true
		}
	}
	switch {
	case hasTrigger(prompt, l.DetailTriggers):
		return 0, false
	case hasTrigger(prompt, l.ExplainTriggers):
		return l.Explain, false
	}
	return l.Chat, false
}

func hasTrigger(prompt string, triggers []string) bool {
	for _, t := range triggers {
		if regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(t) + `\b`).MatchString(prompt) {
			return true
		}
	}
	return false
}

// fileOverLimit says how a scratch file breaks its ceiling, or "". A file
// named for a /write kind (`kit scratch-dir <kind> <slug>`) is held whole to
// that kind's ceiling; a report-format file, each finding to report_finding.
func fileOverLimit(l config.ReplyLimits, path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	base := filepath.Base(path)
	kinds := make([]string, 0, len(l.Write))
	for k := range l.Write {
		kinds = append(kinds, k)
	}
	// Longest first: review-comment, not a shorter kind it starts with.
	slices.SortFunc(kinds, func(a, b string) int { return len(b) - len(a) })
	for _, kind := range kinds {
		if strings.HasPrefix(base, kind+"-") {
			if n := countWords(string(data), true); l.Write[kind] > 0 && n > l.Write[kind] {
				return fmt.Sprintf("%s is %d words; the /write %s ceiling is %d", base, n, kind, l.Write[kind])
			}
			return ""
		}
	}
	_, findings, ok := strings.Cut(string(data), "\n## Findings")
	if !ok || l.ReportFinding <= 0 {
		return ""
	}
	findings, _, _ = strings.Cut(findings, "\n## ")
	var over []string
	for _, f := range strings.Split(findings, "\n### ")[1:] {
		title, _, _ := strings.Cut(f, "\n")
		if n := countWords(f, false); n > l.ReportFinding {
			over = append(over, fmt.Sprintf("%q (%d)", strings.TrimSpace(title), n))
		}
	}
	if len(over) == 0 {
		return ""
	}
	return fmt.Sprintf("%s has findings over %d words: %s", base, l.ReportFinding, strings.Join(over, ", "))
}

// countWords counts the words in text, fenced code only when all. A token
// with no letter or digit (a table's pipes, a bullet, a rule) isn't a word.
func countWords(text string, all bool) int {
	n, fenced := 0, false
	for line := range strings.Lines(text) {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced && !all {
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
