package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// writeCommand is a typed /write and its kind; command is any leading
// slash command, whose name isn't free text for the triggers.
var (
	writeCommand = regexp.MustCompile(`^\s*/(?:kit:)?write\s+([a-z-]+)`)
	command      = regexp.MustCompile(`^\s*/\S+`)
)

// turn is what UserPromptSubmit records for the Stop and PostToolUse that
// follow: the ceiling, whether code counts, and the /write kind, if any.
type turn struct {
	Limit int    `json:"limit"`
	All   bool   `json:"all"`
	Kind  string `json:"kind"`
	State string `json:"state"` // "", "passed", or "blocked" once Stop has judged it
}

func writeTurn(marker string, t turn) {
	if data, err := json.Marshal(t); err == nil && marker != "" && os.MkdirAll(filepath.Dir(marker), 0o755) == nil {
		_ = os.WriteFile(marker, data, 0o644)
	}
}

// ReplyLength holds output to the word ceilings in kit.yml reply_limits
// (rules/output.md section 0). On UserPromptSubmit it picks the turn's
// ceiling from the prompt, records it, and says it; at Stop it blocks a
// final reply over it, once per turn; after a write to scratch it sends back
// a /write turn's .md file or a report finding over its ceiling.
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
		// that too, and a long reply after it still counts. A blocked turn's
		// next Stop goes through; a turn with no prompt of its own (a task
		// notification) gets the chat ceiling.
		t := turn{Limit: l.Chat}
		if data, err := os.ReadFile(marker); err == nil {
			_ = json.Unmarshal(data, &t)
		}
		switch {
		case t.State == "blocked":
			os.Remove(marker)
			return nil
		case t.State == "passed" && !p.Bool("stop_hook_active"):
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
	case "PostToolUse":
		path := p.FilePath()
		if !project.IsScratch(h.Paths, path) || filepath.Ext(path) != ".md" {
			return nil
		}
		var t turn
		if data, err := os.ReadFile(marker); err == nil {
			_ = json.Unmarshal(data, &t)
		}
		if reason := fileOverLimit(l, t, path); reason != "" {
			return h.Block(reason+" (kit.yml reply_limits, rules/output.md section 0). Rewrite it shorter: one idea per line, nothing the reader already knows.", "reply-length")
		}
	}
	return nil
}

// turnLimit is what a prompt sets: a /write kind's ceiling, code included;
// none after a detail trigger; the explain ceiling after an explain trigger;
// else the chat one. A leading slash command's name isn't a trigger.
func turnLimit(l config.ReplyLimits, prompt string) turn {
	if m := writeCommand.FindStringSubmatch(prompt); m != nil {
		if n, ok := l.Write[m[1]]; ok {
			return turn{Limit: n, All: true, Kind: m[1]}
		}
	}
	text := command.ReplaceAllString(prompt, "")
	switch {
	case hasTrigger(text, l.DetailTriggers):
		return turn{}
	case hasTrigger(text, l.ExplainTriggers):
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
	return regexp.MustCompile(`(?i)\b(?:` + strings.Join(quoted, "|") + `)\b`).MatchString(text)
}

// fileOverLimit says how a scratch .md file breaks its ceiling, or "". In a
// /write turn the file is that kind's output, held whole to its ceiling; else
// a report-format file has each finding held to report_finding.
func fileOverLimit(l config.ReplyLimits, t turn, path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	base := filepath.Base(path)
	if t.Kind != "" {
		if n := countWords(string(data), true); t.Limit > 0 && n > t.Limit {
			return fmt.Sprintf("%s is %d words; the /write %s ceiling is %d", base, n, t.Kind, t.Limit)
		}
		return ""
	}
	if l.ReportFinding <= 0 {
		return ""
	}
	var over []string
	for _, f := range findings(string(data)) {
		title, _, _ := strings.Cut(f, "\n")
		if n := countWords(f, false); n > l.ReportFinding {
			over = append(over, fmt.Sprintf("%q (%d)", strings.TrimSpace(strings.TrimPrefix(title, "### ")), n))
		}
	}
	if len(over) == 0 {
		return ""
	}
	return fmt.Sprintf("%s has findings over %d words: %s", base, l.ReportFinding, strings.Join(over, ", "))
}

// findings is each "### " section under a report's "## Findings", up to the
// next "## ". Headings inside fenced code are text, not structure.
func findings(report string) []string {
	var out []string
	var cur strings.Builder
	in, fenced := false, false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for line := range strings.Lines(report) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			fenced = !fenced
		}
		switch {
		case fenced || strings.HasPrefix(trimmed, "```"):
		case strings.HasPrefix(line, "## "):
			flush()
			in = trimmed == "## Findings"
			continue
		case in && strings.HasPrefix(line, "### "):
			flush()
		}
		if in && (cur.Len() > 0 || strings.HasPrefix(line, "### ")) {
			cur.WriteString(line)
		}
	}
	flush()
	return out
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
