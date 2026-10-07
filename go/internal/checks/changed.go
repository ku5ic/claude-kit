package checks

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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
	git := func(args ...string) (string, error) {
		out, err := exec.Command("git", append([]string{"-C", root, "-c", "core.quotePath=false"}, args...)...).Output()
		return string(out), err
	}
	tracked, err := git(append([]string{"ls-tree", "-r", "--name-only", rev, "--"}, rel...)...)
	if err != nil {
		return nil
	}
	diff, err := git(append([]string{"diff", "-U0", "--no-color", "--no-ext-diff", "--no-renames", rev, "--"}, rel...)...)
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
	for line := range strings.SplitSeq(diff, "\n") {
		if name, ok := strings.CutPrefix(line, "+++ b/"); ok {
			current = changed[root+"/"+name]
			continue
		}
		m := hunk.FindStringSubmatch(line)
		if m == nil || current == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		if count == 0 {
			// A pure deletion: the lines on either side of it are touched.
			current[start], current[start+1] = true, true
		}
		for n := start; n < start+count; n++ {
			current[n] = true
		}
	}
	return changed
}

var hunk = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
