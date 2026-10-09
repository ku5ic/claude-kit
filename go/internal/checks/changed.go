package checks

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/git"
)

// changedLines maps each of files (absolute, under root) to the lines the
// working tree changed against rev; a nil entry means every line (a file
// rev doesn't have). A file tracked and unchanged maps to no lines. The
// whole map is nil, every line of every file counting, when git can't say.
func changedLines(root, rev string, files []string) map[string]map[int]bool {
	var rel []string
	for _, f := range files {
		rel = append(rel, strings.TrimPrefix(f, root+"/"))
	}
	run := func(args ...string) (string, error) {
		return git.Output(root, append([]string{"-c", "core.quotePath=false"}, args...)...)
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
	changed := map[string]map[int]bool{}
	for _, f := range files {
		changed[f] = nil
	}
	for name := range strings.SplitSeq(tracked, "\n") {
		if name != "" {
			changed[root+"/"+name] = map[int]bool{}
		}
	}
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
				return nil
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
