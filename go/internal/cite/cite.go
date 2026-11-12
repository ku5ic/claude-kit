// Package cite finds file:line citations in a text that don't resolve in a
// project: a file that isn't there, or a line past its end. It reads files
// only, no model.
package cite

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// citation is path:line or path:line-line, after a space, quote, bracket,
// backtick, or the start of a line, so a URL's host:port never matches.
var citation = regexp.MustCompile("(?:^|[\\s`(\"'\\[])((?:\\.{1,2}/|/)?[\\w.-]+(?:/[\\w.-]+)*\\.[A-Za-z0-9]+):([0-9]+)(?:-([0-9]+))?\\b")

// Problem is one citation that doesn't resolve, and why.
type Problem struct {
	Cite, Reason string
}

// Broken is every citation in text that doesn't resolve under root, in the
// order they appear, each once. A citation whose first path segment isn't
// in root points at another tree, and is skipped, as is a bare file name
// not at root, which could live in any directory.
func Broken(root, text string) []Problem {
	var out []Problem
	seen := map[string]bool{}
	lines := map[string]int{}
	for _, m := range citation.FindAllStringSubmatch(text, -1) {
		cite := m[1] + ":" + m[2]
		if m[3] != "" {
			cite += "-" + m[3]
		}
		if seen[cite] {
			continue
		}
		seen[cite] = true
		path, ok := inRoot(root, m[1])
		if !ok {
			continue
		}
		n, known := lines[path]
		if !known {
			n = lineCount(path)
			lines[path] = n
		}
		if reason := check(n, m[2], m[3]); reason != "" {
			out = append(out, Problem{cite, reason})
		}
	}
	return out
}

// inRoot is the file a cited path names under root, and whether the
// citation is root's to check.
func inRoot(root, cited string) (string, bool) {
	path := cited
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, cited)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	first, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
	if _, err := os.Stat(filepath.Join(root, first)); err != nil {
		return "", false
	}
	return path, true
}

// lineCount is the file's line count, or -1 when it isn't a readable file.
func lineCount(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	n := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		n++
	}
	return n
}

// check says why line (through end, when set) isn't in a file of n lines.
func check(n int, line, end string) string {
	if n < 0 {
		return "no such file"
	}
	last := line
	if end != "" {
		last = end
	}
	from, _ := strconv.Atoi(line)
	to, _ := strconv.Atoi(last)
	switch {
	case from < 1:
		return "lines start at 1"
	case to < from:
		return "the range ends before it starts"
	case to > n:
		return fmt.Sprintf("the file has %d lines", n)
	}
	return ""
}
