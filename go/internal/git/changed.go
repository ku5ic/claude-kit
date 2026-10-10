package git

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// Since is what changed in a repo since its git base: the base, its
// merge-base with HEAD, and the files changed since that merge-base, by
// absolute path: committed, staged, unstaged, and untracked, deletions left
// out.
type Since struct {
	Base, MergeBase string
	Files           map[string]bool
}

// ChangedSince is what changed in root's repo since its git base; false
// when no base resolves.
func ChangedSince(root string) (Since, bool) {
	base, ok := Base(root, "")
	if !ok {
		return Since{}, false
	}
	run := func(args ...string) []string {
		lines, err := Lines(root, append([]string{"-c", "core.quotePath=false"}, args...)...)
		if err != nil {
			return nil
		}
		return lines
	}
	mb := run("merge-base", base, "HEAD")
	if len(mb) != 1 {
		return Since{}, false
	}
	s := Since{Base: base, MergeBase: mb[0], Files: map[string]bool{}}
	for _, f := range append(run("diff", "--name-only", "--diff-filter=d", mb[0]), run("ls-files", "--others", "--exclude-standard")...) {
		s.Files[filepath.Join(root, f)] = true
	}
	return s, true
}

// ChangedLines maps files (absolute) to the lines the working tree changed
// against a rev; a nil entry means every line (a file the rev doesn't have).
// A whole nil map, every line of every file counting, means git couldn't
// say.
type ChangedLines map[string]map[int]bool

// Touches is true for a finding ChangedLines can't rule out: one without a
// line, or in a file whose every line counts.
func (c ChangedLines) Touches(file string, line int) bool {
	return line == 0 || c[file] == nil || c[file][line]
}

// LinesChanged maps each of files (absolute, under root) to the lines the
// working tree changed against rev. A file tracked and unchanged maps to no
// lines.
func LinesChanged(root, rev string, files []string) ChangedLines {
	var rel []string
	for _, f := range files {
		rel = append(rel, fsx.Rel(root, f))
	}
	run := func(args ...string) (string, error) {
		return Output(root, append([]string{"-c", "core.quotePath=false"}, args...)...)
	}
	tracked, err := run(append([]string{"ls-tree", "-r", "--name-only", rev, "--"}, rel...)...)
	if err != nil {
		return nil
	}
	// Fixed prefixes: diff.noprefix or diff.mnemonicPrefix would change them.
	diff, err := run(append([]string{"diff", "-U0", "--no-color", "--no-ext-diff", "--no-renames", "--src-prefix=a/", "--dst-prefix=b/", rev, "--"}, rel...)...)
	if err != nil {
		return nil
	}
	changed := ChangedLines{}
	for _, f := range files {
		changed[f] = nil
	}
	for name := range strings.SplitSeq(tracked, "\n") {
		if name != "" {
			changed[root+"/"+name] = map[int]bool{}
		}
	}
	if !markDiff(diff, root, changed) {
		return nil
	}
	return changed
}

// hunk is a hunk header: the old line count, the new start, the new count.
var hunk = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// hunkCount is a hunk header's line count; one left out is 1.
func hunkCount(s string) int {
	if s == "" {
		return 1
	}
	n, _ := strconv.Atoi(s)
	return n
}

// markDiff marks each file's lines a -U0 diff adds or touches in changed;
// false when a file's name doesn't map back, and git can't say.
func markDiff(diff, root string, changed ChangedLines) bool {
	var current map[int]bool
	body := 0 // lines of the hunk still to come: content, whatever they start with
	for line := range strings.SplitSeq(diff, "\n") {
		if body > 0 {
			if !strings.HasPrefix(line, `\`) { // "\ No newline at end of file" isn't counted
				body--
			}
			continue
		}
		if header, ok := strings.CutPrefix(line, "+++ "); ok {
			if header == "/dev/null" {
				current = nil
				continue
			}
			// A path with a space ends in a tab; one git quotes ("b/a\"b")
			// doesn't map, and then git can't say: every line counts.
			name, ok := strings.CutPrefix(strings.TrimSuffix(header, "\t"), "b/")
			lines, known := changed[root+"/"+name]
			if !ok || !known {
				return false
			}
			current = lines
			continue
		}
		m := hunk.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[2])
		removed, added := hunkCount(m[1]), hunkCount(m[3])
		body = removed + added
		if current == nil {
			continue
		}
		if added == 0 {
			// A pure deletion: the lines on either side of it are touched.
			current[start], current[start+1] = true, true
		}
		for n := start; n < start+added; n++ {
			current[n] = true
		}
	}
	return true
}
