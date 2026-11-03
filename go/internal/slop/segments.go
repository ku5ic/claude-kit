package slop

import (
	"path/filepath"
	"slices"
	"strings"
)

// syntax is how a file type writes comments: line-comment openers, block
// comment pairs, the characters that open a string (a comment marker inside
// one isn't one), and whether a line opening with triple quotes is a
// docstring.
type syntax struct {
	line   []string
	block  [][2]string
	quotes string
	doc    bool
}

var (
	cLike  = syntax{line: []string{"//"}, block: [][2]string{{"/*", "*/"}}, quotes: "\"'`"}
	css    = syntax{block: [][2]string{{"/*", "*/"}}, quotes: `"'`}
	hash   = syntax{line: []string{"#"}, quotes: `"'`}
	python = syntax{line: []string{"#"}, quotes: `"'`, doc: true}
	markup = syntax{line: []string{"//"}, block: [][2]string{{"<!--", "-->"}, {"/*", "*/"}}, quotes: "\"'`"}

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

// commentLines is the comment text on each line of text, code spans taken
// out. shortcut: a line-at-a-time scan, not a lexer. A string spanning lines
// (a template literal, a Go raw string) isn't tracked, and a Rust lifetime
// reads as an opening quote; both can hide a comment, never invent one. A
// tell wrapped across two comment lines is missed.
func commentLines(text string, syn syntax) []Segment {
	var segs []Segment
	closer := "" // set while inside a block comment or docstring
	for i, line := range strings.Split(text, "\n") {
		var got []string
		rest := line
		if t := strings.TrimSpace(line); closer == "" && syn.doc && (strings.HasPrefix(t, `"""`) || strings.HasPrefix(t, `'''`)) {
			closer, rest = t[:3], t[3:]
		}
		for rest != "" {
			if closer != "" {
				end := strings.Index(rest, closer)
				if end < 0 {
					got = append(got, rest)
					break
				}
				got = append(got, rest[:end])
				rest, closer = rest[end+len(closer):], ""
				continue
			}
			at, open, close := nextComment(rest, syn)
			if at < 0 {
				break
			}
			if close == "" {
				got = append(got, rest[at+len(open):])
				break
			}
			rest, closer = rest[at+len(open):], close
		}
		if t := strings.TrimSpace(strings.Join(got, " ")); t != "" {
			segs = append(segs, Segment{Line: i + 1, Text: stripCode(t)})
		}
	}
	return segs
}

// nextComment is where the first comment outside a string opens in s, its
// opener, and its closer ("" for a line comment); -1 when none does. A "#"
// opens one only at the start or after a blank, a "//" not after a ":", so
// $#, ${#x}, and a URL aren't comments.
func nextComment(s string, syn syntax) (int, string, string) {
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
			return i, open, ""
		}
		for _, b := range syn.block {
			if strings.HasPrefix(s[i:], b[0]) {
				return i, b[0], b[1]
			}
		}
	}
	return -1, "", ""
}
