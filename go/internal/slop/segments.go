package slop

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// syntax is how a file type writes comments: line-comment openers, block
// comment pairs, the characters that open a string (a comment marker inside
// one isn't one), those of them a string may span lines in, whether triple
// quotes open a string (a docstring when they open the line), and whether
// << opens a heredoc.
type syntax struct {
	line    []string
	block   [][2]string
	quotes  string
	multi   string
	doc     bool
	heredoc bool
	// lifetimes: a ' opens a char literal ('x', '\n') or nothing, as
	// Rust's 'a does; it never opens a string.
	lifetimes bool
}

var (
	cLike  = syntax{line: []string{"//"}, block: [][2]string{{"/*", "*/"}}, quotes: "\"'`", multi: "`"}
	css    = syntax{block: [][2]string{{"/*", "*/"}}, quotes: `"'`}
	rust   = syntax{line: []string{"//"}, block: [][2]string{{"/*", "*/"}}, quotes: `"`, lifetimes: true}
	hash   = syntax{line: []string{"#"}, quotes: `"'`, heredoc: true}
	python = syntax{line: []string{"#"}, quotes: `"'`, doc: true}
	markup = syntax{line: []string{"//"}, block: [][2]string{{"<!--", "-->"}, {"/*", "*/"}}, quotes: "\"'`", multi: "`"}

	byExt = map[string]syntax{
		".go": cLike, ".js": cLike, ".jsx": cLike, ".mjs": cLike, ".cjs": cLike, ".ts": cLike, ".tsx": cLike,
		".mts": cLike, ".cts": cLike, ".java": cLike, ".kt": cLike, ".kts": cLike, ".swift": cLike, ".rs": rust,
		".c": cLike, ".h": cLike, ".cc": cLike, ".cpp": cLike, ".hpp": cLike, ".cs": cLike, ".scala": cLike,
		".dart": cLike, ".php": cLike, ".scss": cLike, ".less": cLike,
		".css": css,
		".py":  python, ".pyi": python,
		".rb": hash, ".sh": hash, ".bash": hash, ".zsh": hash, ".yml": hash, ".yaml": hash, ".toml": hash,
		".tf": hash, ".tfvars": hash, ".r": hash, ".pl": hash, ".ex": hash, ".exs": hash, ".nix": hash,
		".html": markup, ".htm": markup, ".vue": markup, ".svelte": markup, ".xml": markup,
	}
	byName = map[string]syntax{"Dockerfile": hash, "Makefile": hash, "justfile": hash, "Brewfile": hash}

	proseExts = []string{"", ".md", ".markdown", ".mdx", ".txt"}
	// uiExts also get every line scanned as written, for the on: [ui] rules.
	uiExts = []string{".tsx", ".jsx", ".vue", ".svelte", ".html", ".htm", ".css", ".scss"}
)

// Segments is what to scan in after, the whole text of a file at path once
// written, and the allow markers in it: prose for Markdown and text, the
// comments of code and config, and for UI files every line too. Only lines
// after adds to before, the file as it was, count: old text never blocks
// new work. The state a line depends on (a fence, a block comment, a marker
// above it) comes from all of after. False for a type with no reader.
func Segments(path, before, after string) ([]Segment, []Allow, bool) {
	syn, code, ok := reader(path, after)
	if !ok {
		return nil, nil, false
	}
	lines := strings.Split(after, "\n")
	var segs []Segment
	if !code {
		segs = proseLines(after)
	} else if segs = commentLines(lines, syn); slices.Contains(uiExts, strings.ToLower(filepath.Ext(path))) {
		segs = append(segs, rawLines(lines)...)
	}
	segs, allows := finish(lines, segs)
	added := addedLines(before, lines)
	segs = slices.DeleteFunc(segs, func(s Segment) bool { return !added[s.Line] })
	allows = slices.DeleteFunc(allows, func(a Allow) bool { return !added[a.Line] })
	return segs, allows, true
}

// reader is how Segments reads path: its comment syntax, or code false for
// prose; ok false for a type with no reader. With no extension, a shebang
// opening after makes it a script.
func reader(path, after string) (syn syntax, code, ok bool) {
	ext := strings.ToLower(filepath.Ext(path))
	if syn, ok := byName[filepath.Base(path)]; ok {
		return syn, true, true
	}
	if ext == "" && strings.HasPrefix(after, "#!") {
		return hash, true, true
	}
	if slices.Contains(proseExts, ext) {
		return syntax{}, false, true
	}
	syn, ok = byExt[ext]
	return syn, true, ok
}

// Reads reports whether Segments has a reader for path's type, which the
// path alone decides.
func Reads(path string) bool {
	_, _, ok := reader(path, "")
	return ok
}

// addedLines is the 1-based lines of after that before doesn't hold, each
// line of before matching at most one of after.
func addedLines(before string, after []string) map[int]bool {
	count := map[string]int{}
	if before != "" {
		for _, l := range strings.Split(before, "\n") {
			count[strings.TrimRight(l, "\r")]++
		}
	}
	added := map[int]bool{}
	for i, l := range after {
		if k := strings.TrimRight(l, "\r"); count[k] > 0 {
			count[k]--
			continue
		}
		added[i+1] = true
	}
	return added
}

func rawLines(lines []string) []Segment {
	var segs []Segment
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			segs = append(segs, Segment{Line: i + 1, Text: line, Raw: true})
		}
	}
	return segs
}

// The kinds of span commentLines tracks: a comment to the end of the line, a
// block comment or docstring, a string, and a heredoc.
const (
	lineComment = iota + 1
	blockComment
	str
	heredoc
)

// span is an opened comment, string, or heredoc: its opener and closer (a
// heredoc's closer is its delimiter word).
type span struct {
	kind        int
	open, close string
}

// heredocOpener is << (not <<<) with an optional - or ~, then the delimiter,
// quoted or not; a YAML merge key (<<:) has none.
var heredocOpener = regexp.MustCompile(`^<<[-~]?[ \t]*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)

// commentLines is the comment text on each of lines, code spans taken
// out. shortcut: a line-at-a-time scan, not a lexer. A string or heredoc
// spanning lines is followed to its closer, and a template literal's ${}
// to its brace, but only within one line. A tell wrapped across two
// comment lines is missed.
func commentLines(lines []string, syn syntax) []Segment {
	var segs []Segment
	var open span // the block comment, docstring, string, or heredoc a line starts inside
	for i, line := range lines {
		if open.kind == heredoc {
			if strings.TrimSpace(line) == open.close {
				open = span{}
			}
			continue
		}
		var got []string
		rest, pending := line, "" // pending: a heredoc this line opens
		if t := strings.TrimSpace(line); open.kind == 0 && syn.doc && (strings.HasPrefix(t, `"""`) || strings.HasPrefix(t, `'''`)) {
			open, rest = span{blockComment, t[:3], t[:3]}, t[3:]
		}
		for rest != "" {
			if open.kind != 0 {
				end := closeAt(rest, open)
				if open.kind == blockComment {
					if end < 0 {
						got = append(got, rest)
					} else {
						got = append(got, rest[:end])
					}
				}
				if end < 0 {
					break
				}
				rest, open = rest[end+len(open.close):], span{}
				continue
			}
			at, next := nextSpan(rest, syn)
			switch {
			case at < 0:
				rest = ""
			case next.kind == lineComment:
				got, rest = append(got, rest[at+len(next.open):]), ""
			case next.kind == heredoc:
				pending, rest = next.close, rest[at+len(next.open):]
			default:
				open, rest = next, rest[at+len(next.open):]
			}
		}
		if pending != "" && open.kind == 0 {
			open = span{kind: heredoc, close: pending}
		}
		if t := strings.TrimSpace(strings.Join(got, " ")); t != "" {
			segs = append(segs, Segment{Line: i + 1, Text: stripCode(t)})
		}
	}
	return segs
}

// closeAt is where open's closer is in s, -1 when it isn't. Inside a string
// a backslash escapes the next character; inside a comment nothing does.
func closeAt(s string, open span) int {
	if open.kind == blockComment {
		return strings.Index(s, open.close)
	}
	if open.close == "`" {
		return closeTemplate(s)
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
		} else if strings.HasPrefix(s[i:], open.close) {
			return i
		}
	}
	return -1
}

// closeTemplate is where the backtick closing a template literal is in s,
// past any ${} and the templates and strings nested in it; -1 when none is.
func closeTemplate(s string) int {
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\':
			i++
		case s[i] == '`':
			return i
		case strings.HasPrefix(s[i:], "${"):
			end := closeBrace(s[i+2:])
			if end < 0 {
				return -1
			}
			i += 2 + end
		}
	}
	return -1
}

// closeBrace is where the } closing a ${ is in s, past nested braces,
// templates, and strings; -1 when none is.
func closeBrace(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '`':
			end := closeTemplate(s[i+1:])
			if end < 0 {
				return -1
			}
			i += 1 + end
		case '"', '\'':
			end := closeAt(s[i+1:], span{str, s[i : i+1], s[i : i+1]})
			if end < 0 {
				return -1
			}
			i += 1 + end
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

// charLiteral is the length of the char literal s starts with ('x', '\n',
// '\u{1F600}'), or 0 when its ' opens none, as a Rust lifetime's doesn't.
func charLiteral(s string) int {
	if strings.HasPrefix(s, `'\`) {
		if end := strings.IndexByte(s[min(3, len(s)):], '\''); end >= 0 {
			return 3 + end + 1
		}
		return 0
	}
	_, size := utf8.DecodeRuneInString(s[1:])
	if len(s) > 1+size && s[1+size] == '\'' {
		return size + 2
	}
	return 0
}

// nextSpan is where the first comment, heredoc, or string the line doesn't
// close opens in s, and that span; -1 when none does. A "#" opens a comment
// only at the start or after a blank, a "//" not after a ":", so $#, ${#x},
// and a URL aren't comments.
func nextSpan(s string, syn syntax) (int, span) {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
			continue
		}
		if syn.doc && (strings.HasPrefix(s[i:], `"""`) || strings.HasPrefix(s[i:], `'''`)) {
			triple := span{str, s[i : i+3], s[i : i+3]}
			end := closeAt(s[i+3:], triple)
			if end < 0 {
				return i, triple
			}
			i += 3 + end + 2
			continue
		}
		if strings.IndexByte(syn.multi, c) >= 0 {
			multi := span{str, s[i : i+1], s[i : i+1]}
			end := closeAt(s[i+1:], multi)
			if end < 0 {
				return i, multi
			}
			i += 1 + end
			continue
		}
		if syn.lifetimes && c == '\'' {
			if n := charLiteral(s[i:]); n > 0 {
				i += n - 1
			}
			continue
		}
		if strings.IndexByte(syn.quotes, c) >= 0 {
			quote = c
			continue
		}
		for _, open := range syn.line {
			if !strings.HasPrefix(s[i:], open) {
				continue
			}
			if open == "#" && i > 0 && s[i-1] != ' ' && s[i-1] != '\t' || open == "//" && i > 0 && s[i-1] == ':' {
				continue
			}
			return i, span{kind: lineComment, open: open}
		}
		for _, b := range syn.block {
			if strings.HasPrefix(s[i:], b[0]) {
				return i, span{blockComment, b[0], b[1]}
			}
		}
		if syn.heredoc && c == '<' && (i == 0 || s[i-1] != '<') {
			if m := heredocOpener.FindStringSubmatch(s[i:]); m != nil {
				return i, span{heredoc, m[0], m[1]}
			}
		}
	}
	return -1, span{}
}
