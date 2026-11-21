package claims

import (
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

var (
	// evidenceField is a finding's Evidence line, as skills/report-format
	// writes it.
	evidenceField = regexp.MustCompile(`(?m)^\s*(?:[-*]\s*)?\*\*Evidence:\*\*\s*(.+)$`)
	lineSuffix    = regexp.MustCompile(`:\d+(-\d+)?$`)
)

// ungrounded is each Evidence field of text naming nothing the session
// touched: no backticked file:line, path, command, or value, or one that no
// tool call's input or result holds. A value counts when a result showed it.
func ungrounded(entries []transcript.Entry, text string) []Unsupported {
	fields := evidenceField.FindAllStringSubmatch(fence.ReplaceAllString(text, ""), -1)
	if len(fields) == 0 {
		return nil
	}
	var seen []string
	for _, b := range transcript.Blocks(entries) {
		switch b.Type {
		case "tool_use":
			seen = append(seen, b.Input.Command+"\n"+string(b.RawInput))
		case "tool_result":
			seen = append(seen, b.ResultText())
		}
	}
	var out []Unsupported
	for _, f := range fields {
		names := backticked.FindAllStringSubmatch(f[1], -1)
		if len(names) == 0 {
			out = append(out, Unsupported{Claim: f[1], Check: "name the file:line read or the command run, in backticks"})
			continue
		}
		for _, n := range names {
			core := lineSuffix.ReplaceAllString(n[1], "")
			if !slices.ContainsFunc(seen, func(s string) bool { return strings.Contains(s, core) }) {
				out = append(out, Unsupported{Claim: f[1], Check: "`" + n[1] + "` never ran this session: run it, or cut the finding"})
				break
			}
		}
	}
	return out
}
