// Package md reads Markdown a line at a time, knowing which lines are
// fenced code and which are YAML frontmatter. It imports nothing from the
// kit.
package md

import (
	"iter"
	"regexp"
	"strings"
)

// Kind is what a line is part of.
type Kind int

const (
	Prose      Kind = iota
	Front           // YAML frontmatter, its --- delimiters included
	FenceOpen       // a ``` or ~~~ line opening a fenced block
	Code            // a line inside a fenced block
	FenceClose      // the line closing it: the same marker
)

// Line is one line of a text, newline kept, and what it's part of.
type Line struct {
	Text string
	Kind Kind
}

var (
	frontDelim = regexp.MustCompile(`^---[[:space:]]*$`)
	frontClose = regexp.MustCompile(`(?m)^---[[:space:]]*$`)
)

func closesFront(rest string) bool { return frontClose.MatchString(rest) }

// Lines yields each line of text. Frontmatter is a --- first line through
// the next --- line; with no closing one, the first is prose. A fence opens
// on a line starting, after indentation, with ``` or ~~~, and closes only
// on its own marker; one never closed runs to the end.
func Lines(text string) iter.Seq[Line] {
	return func(yield func(Line) bool) {
		first, front, fence := true, false, ""
		for raw := range strings.Lines(text) {
			line := strings.TrimRight(raw, "\r\n")
			trimmed := strings.TrimSpace(line)
			kind := Prose
			switch {
			case first && frontDelim.MatchString(line) && closesFront(text[len(raw):]):
				front, kind = true, Front
			case front:
				front, kind = !frontDelim.MatchString(line), Front
			case fence != "" && strings.HasPrefix(trimmed, fence):
				fence, kind = "", FenceClose
			case fence != "":
				kind = Code
			case strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"):
				fence, kind = trimmed[:3], FenceOpen
			}
			first = false
			if !yield(Line{raw, kind}) {
				return
			}
		}
	}
}
