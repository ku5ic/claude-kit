// Package cite finds file:line citations in a text that don't resolve in a
// project: a file that isn't there, or a line past its end. It reads files,
// and git's file list for a shortened path, no model.
package cite

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/git"
)

// citation is path:line or path:line-line, after a space, quote, bracket,
// backtick, or the start of a line, so a URL's host:port never matches.
var citation = regexp.MustCompile("(?:^|[\\s`(\"'\\[])((?:\\.{1,2}/|/)?[\\w.-]+(?:/[\\w.-]+)*\\.[A-Za-z0-9]+):([0-9]+)(?:-([0-9]+))?\\b")

// Problem is one citation that doesn't resolve, and why.
type Problem struct {
	Cite, Reason string
}

// hexWord is a word that may be a commit hash.
var hexWord = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)

// namesRevision is whether text names a commit, a hex word with a digit
// and a letter.
func namesRevision(text string) bool {
	return slices.ContainsFunc(hexWord.FindAllString(text, -1), func(w string) bool {
		return strings.ContainsAny(w, "0123456789") && strings.ContainsAny(w, "abcdef")
	})
}

// matches is every citation match in text but one followed, before the
// next citation or the sentence's end, by a commit it points at.
func matches(text string) [][]string {
	var out [][]string
	subs, at := citation.FindAllStringSubmatch(text, -1), citation.FindAllStringIndex(text, -1)
	for i, m := range at {
		end := len(text)
		if i+1 < len(at) {
			end = at[i+1][0]
		}
		after := text[m[1]:end]
		if j := strings.IndexAny(after, "\n;"); j >= 0 {
			after = after[:j]
		}
		if j := strings.Index(after, ". "); j >= 0 {
			after = after[:j]
		}
		if !namesRevision(after) {
			out = append(out, subs[i])
		}
	}
	return out
}

// Broken is every citation in text that doesn't resolve under root, in the
// order they appear, each once. A citation whose first path segment isn't
// in root points at another tree, and is skipped, as is a bare file name
// not at root, which could live in any directory, and one that names a
// commit after it.
func Broken(root, text string) []Problem {
	var out []Problem
	seen := map[string]bool{}
	lines := map[string]int{}
	for _, m := range matches(text) {
		cite := m[1] + ":" + m[2]
		if m[3] != "" {
			cite += "-" + m[3]
		}
		if seen[cite] {
			continue
		}
		seen[cite] = true
		path, ok := inRoot(root, m[1], false)
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
// citation is root's to check. A relative path that names no file there
// is read as a shortened one, naming the one tracked file it ends. One whose
// first segment isn't at root points at another tree, unless anyFirst and
// it ends a tracked file.
func inRoot(root, cited string, anyFirst bool) (string, bool) {
	path := cited
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, cited)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	first, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
	_, firstErr := os.Stat(filepath.Join(root, first))
	if firstErr != nil && !anyFirst {
		return "", false
	}
	if _, err := os.Stat(path); err != nil && !filepath.IsAbs(cited) {
		if ends, _ := git.Lines(root, "ls-files", "--", ":(glob)**/"+filepath.ToSlash(cited)); len(ends) == 1 {
			return filepath.Join(root, ends[0]), true
		}
	}
	return path, firstErr == nil
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

// Files is each file a citation in text names under root, absolute, in
// the order they appear, each once; citations Broken skips are skipped
// here too, except a shortened path whose first segment isn't at root.
func Files(root, text string) []string {
	var out []string
	for _, m := range matches(text) {
		if path, ok := inRoot(root, m[1], true); ok && !slices.Contains(out, path) {
			out = append(out, path)
		}
	}
	return out
}
