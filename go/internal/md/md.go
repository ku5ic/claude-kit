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
			case fence != "" && closes(trimmed, fence):
				fence, kind = "", FenceClose
			case fence != "":
				kind = Code
			default:
				if open := opener(trimmed); open != "" {
					fence, kind = open, FenceOpen
				}
			}
			first = false
			if !yield(Line{raw, kind}) {
				return
			}
		}
	}
}

// listMarker opens a list item, whose line a fence may follow.
var listMarker = regexp.MustCompile(`^(?:[-*+]|\d{1,9}[.)])[ \t]+`)

// fenceRun is the run of 3 or more backticks or tildes s opens with, or "".
func fenceRun(s string) string {
	if !strings.HasPrefix(s, "```") && !strings.HasPrefix(s, "~~~") {
		return ""
	}
	return s[:len(s)-len(strings.TrimLeft(s, s[:1]))]
}

// opener is the fence a trimmed line opens, after any list marker, or "".
// A backtick fence's info string holds no backtick (CommonMark 4.5).
func opener(trimmed string) string {
	line := listMarker.ReplaceAllString(trimmed, "")
	run := fenceRun(line)
	if run == "" || run[0] == '`' && strings.Contains(line[len(run):], "`") {
		return ""
	}
	return run
}

// closes reports whether a trimmed line closes fence: a run of its
// character at least as long, with nothing after it.
func closes(trimmed, fence string) bool {
	run := fenceRun(trimmed)
	return run != "" && run[0] == fence[0] && len(run) >= len(fence) && trimmed == run
}
