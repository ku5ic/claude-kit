// Package connector tells an MCP tool that writes to an outside system from
// one that only reads it, by the verbs in the tool's own name, so no
// connector needs naming anywhere.
package connector

import (
	"slices"
	"strings"
	"unicode"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

// IsWrite is true when tool is an MCP tool whose name holds a write verb.
func IsWrite(c config.Connectors, tool string) bool {
	return holds(words(tool), c.WriteVerbs)
}

// IsRead is true when tool is an MCP tool whose name holds a read verb and
// no write verb.
func IsRead(c config.Connectors, tool string) bool {
	w := words(tool)
	return holds(w, c.ReadVerbs) && !holds(w, c.WriteVerbs)
}

func holds(words, verbs []string) bool {
	return slices.ContainsFunc(words, func(w string) bool { return slices.Contains(verbs, w) })
}

// words is the lowercased words of the tool part of an
// mcp__<server>__<tool> name, split on any non-letter-or-digit and where
// lower case turns upper; nil for any other name.
func words(tool string) []string {
	rest, ok := strings.CutPrefix(tool, "mcp__")
	if !ok {
		return nil
	}
	i := strings.LastIndex(rest, "__")
	if i < 0 {
		return nil
	}
	var b strings.Builder
	prev := rune(0)
	for _, r := range rest[i+2:] {
		if unicode.IsUpper(r) && unicode.IsLower(prev) {
			b.WriteByte(' ')
		}
		b.WriteRune(unicode.ToLower(r))
		prev = r
	}
	return strings.FieldsFunc(b.String(), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}
