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
// on a line starting, after indentation, with ``` or ~~~, and closes on
// its own marker. One never closed runs to the end, or, opened on a list
// item's line, to the first line indented less than the item's content.
func Lines(text string) iter.Seq[Line] {
	return func(yield func(Line) bool) {
		first, front, fence, within := true, false, "", 0
		for raw := range strings.Lines(text) {
			line := strings.TrimRight(raw, "\r\n")
			trimmed := strings.TrimSpace(line)
			// The list item, and the fence in it, ended. Its own marker,
			// even unindented, still closes it: CommonMark would open a new
			// fence there that runs to the end.
			if fence != "" && trimmed != "" && !closes(trimmed, fence) && len(line)-len(strings.TrimLeft(line, " \t")) < within {
				fence = ""
			}
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
				if open, content := opener(line); open != "" {
					fence, within, kind = open, content, FenceOpen
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

// opener is the fence line opens, after indentation and any list marker,
// or "", and the column the list item's content starts at, 0 outside one.
// A backtick fence's info string holds no backtick (CommonMark 4.5).
func opener(line string) (fence string, content int) {
	rest := strings.TrimLeft(line, " \t")
	if marker := listMarker.FindString(rest); marker != "" {
		content = len(line) - len(rest) + len(marker)
		rest = rest[len(marker):]
	}
	rest = strings.TrimSpace(rest)
	run := fenceRun(rest)
	if run == "" || run[0] == '`' && strings.Contains(rest[len(run):], "`") {
		return "", 0
	}
	return run, content
}

// closes reports whether a trimmed line closes fence: a run of its
// character at least as long, with nothing after it.
func closes(trimmed, fence string) bool {
	run := fenceRun(trimmed)
	return run != "" && run[0] == fence[0] && len(run) >= len(fence) && trimmed == run
}
