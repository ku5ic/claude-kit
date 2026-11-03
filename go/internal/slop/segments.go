package slop

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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
}

var (
	cLike  = syntax{line: []string{"//"}, block: [][2]string{{"/*", "*/"}}, quotes: "\"'`", multi: "`"}
	css    = syntax{block: [][2]string{{"/*", "*/"}}, quotes: `"'`}
	hash   = syntax{line: []string{"#"}, quotes: `"'`, heredoc: true}
	python = syntax{line: []string{"#"}, quotes: `"'`, doc: true}
	markup = syntax{line: []string{"//"}, block: [][2]string{{"<!--", "-->"}, {"/*", "*/"}}, quotes: "\"'`", multi: "`"}

	byExt = map[string]syntax{
		".go": cLike, ".js": cLike, ".jsx": cLike, ".mjs": cLike, ".cjs": cLike, ".ts": cLike, ".tsx": cLike,
		".mts": cLike, ".cts": cLike, ".java": cLike, ".kt": cLike, ".kts": cLike, ".swift": cLike, ".rs": cLike,
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
	ext := strings.ToLower(filepath.Ext(path))
	var segs []Segment
	syn, ok := byName[filepath.Base(path)]
	if ext == "" && strings.HasPrefix(after, "#!") {
		syn, ok = hash, true
	}
	switch {
	case slices.Contains(proseExts, ext) && !ok:
		segs = proseLines(after)
	case ok:
		segs = commentLines(after, syn)
	default:
		syn, ok := byExt[ext]
		if !ok {
			return nil, nil, false
		}
		segs = commentLines(after, syn)
		if slices.Contains(uiExts, ext) {
			segs = append(segs, rawLines(after)...)
			slices.SortStableFunc(segs, func(a, b Segment) int { return a.Line - b.Line })
		}
	}
	lines := strings.Split(after, "\n")
	segs, allows := finish(lines, segs)
	added := addedLines(before, lines)
	segs = slices.DeleteFunc(segs, func(s Segment) bool { return !added[s.Line] })
	allows = slices.DeleteFunc(allows, func(a Allow) bool { return !added[a.Line] })
	return segs, allows, true
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

func rawLines(text string) []Segment {
	var segs []Segment
	for i, line := range strings.Split(text, "\n") {
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

// commentLines is the comment text on each line of text, code spans taken
// out. shortcut: a line-at-a-time scan, not a lexer. A string or heredoc
// spanning lines is followed to its closer, but ${} inside a template
// literal isn't, and a Rust lifetime reads as an opening quote, which can
// hide the rest of its line. A tell wrapped across two comment lines is
// missed.
func commentLines(text string, syn syntax) []Segment {
	var segs []Segment
	var open span // the block comment, docstring, string, or heredoc a line starts inside
	for i, line := range strings.Split(text, "\n") {
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
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
		} else if strings.HasPrefix(s[i:], open.close) {
			return i
		}
	}
	return -1
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
