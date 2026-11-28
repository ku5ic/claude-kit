package bashguard

import (
	"strings"
)

// Word is one shell word: Value has quotes removed and escapes resolved,
// with expansions left as their source text ("$HOME/x", "$(kit scratch-dir)").
// Raw is the source.
type Word struct {
	Value string
	Raw   string
	end   int // source offset just past the word
	// literal is whether the shell passes the word as written: no
	// expansion, brace list, or glob character in any part.
	literal bool
}

// values is each word's Value.
func values(words []Word) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		out = append(out, w.Value)
	}
	return out
}

// Redir is an output redirect, its operator and target word.
type Redir struct {
	Op     string
	Target Word
}

// Call is one simple command of a pipeline.
type Call struct {
	Assigns []string // leading VAR=value words, the value's quotes removed
	// plainValues are the source spans of Assigns' values the shell takes
	// as written.
	plainValues [][2]int
	Words       []Word
	Redirs      []Redir
	Inputs      []string // < redirect sources, quotes removed
	// Heredocs are the raw bodies of this command's here-documents and
	// here-strings.
	Heredocs []string
	// Exports are the names an export declares, with a value or without.
	Exports []string
	// decl is the declaration keyword (export, declare, local, readonly,
	// typeset) this call came from, "" for a plain command.
	decl string
	// declOpaque is true for a declaration that can set a name its words
	// don't spell: a nameref (-n), an integer (-i), whose value is
	// arithmetic, or a word that isn't a plain flag, name, or NAME=value.
	declOpaque bool
	start      int // source offset of the first word
	end        int // source offset where the command and its redirects end
	// shellString is true when a POSIX shell's literal -c string was parsed
	// as commands (shellCString).
	shellString bool
}

// Segment is one pipeline: the commands between &&, ||, ;, &, or a newline.
type Segment struct {
	Calls []Call
	end   int // source offset where the pipeline ends
	src   string
	// subshell is a pipeline inside ( ), $( ), backticks, <( ), a
	// backgrounded statement, or a -c string: a cd there moves nothing after
	// the subshell. scopes are the ids of the subshells holding it,
	// outermost first, so a walk can tell where that subshell ends.
	subshell bool
	scopes   []int
	// conditional is a pipeline that may not run: on the right of && or
	// ||, or inside a compound command.
	conditional bool
	// loop is the outermost for, while, or until loop holding the
	// pipeline, 0 for none: its body reruns, so an assignment anywhere in
	// it may be in effect.
	loop int
}

// Rest is the normalized source text after call's word, to the end of that
// call and its redirects: what the per-command regexes match on, so a later
// command of the pipeline (| grep -- delete) never reads as this one's
// words. A word index past the end means the text after the whole call.
func (s Segment) Rest(call, word int) string {
	c := s.Calls[call]
	start := c.start
	switch {
	case word < len(c.Words):
		start = c.Words[word].end
	case len(c.Words) > 0:
		start = c.Words[len(c.Words)-1].end
	}
	if start >= c.end || c.end > len(s.src) {
		return ""
	}
	return normalize(s.src[start:c.end])
}

// normalize turns tabs into spaces and squeezes runs of spaces, so every
// rule matches one spacing.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

// shells are the POSIX-family shells: a heredoc body fed to one is a
// script, and -c runs its string.
var shells = func() map[string]bool {
	m := map[string]bool{}
	for _, name := range strings.Fields("sh bash rbash zsh dash ash hush ksh mksh yash") {
		m[name] = true
	}
	return m
}()

// runsC is true when name's -c runs a string: every shell, and fish, which
// isn't in shells because its heredoc bodies don't parse as POSIX.
func runsC(name string) bool { return shells[name] || name == "fish" }
