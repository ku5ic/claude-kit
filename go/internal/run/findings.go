package run

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
)

// Finding is one line of a check's output that names a file: the file,
// absolute, the line it names (0 for none), and the text.
type Finding struct {
	File string
	Line int
	Text string
}

var (
	// position is a :line or :line:col after a path, then a colon, a space,
	// or the end.
	position = regexp.MustCompile(`:(\d+)(?::\d+)?(?::|\s|$)`)
	// indented is a line:col under a file's own line (eslint's stylish).
	indented = regexp.MustCompile(`^\s+(\d+):\d+\s`)
)

// Findings reads a check's output, run in dir, for the files it names,
// whatever the tool: a path:line[:col] anywhere in a line, a file's line
// followed by indented line:col lines, or a line that is only a file's
// path. A path counts only when it names an existing file.
func Findings(out, dir string) []Finding {
	var found []Finding
	var listed *Finding // a line that is only a file's path
	listedUsed := false
	flush := func() {
		if listed != nil && !listedUsed {
			found = append(found, *listed)
		}
		listed = nil
	}
	for line := range strings.Lines(out) {
		line = strings.TrimRight(line, "\r\n")
		if m := indented.FindStringSubmatch(line); m != nil && listed != nil {
			n, _ := strconv.Atoi(m[1])
			found = append(found, Finding{File: listed.File, Line: n, Text: listed.Text + " " + strings.TrimSpace(line)})
			listedUsed = true
			continue
		}
		flush()
		if f, ok := located(line, dir); ok {
			found = append(found, f)
		} else if path := strings.TrimSpace(line); path != "" && fsx.IsFile(fsx.Abs(dir, path)) {
			listed, listedUsed = &Finding{File: fsx.Abs(dir, path), Text: path}, false
		}
	}
	flush()
	return found
}

// located is the finding a path:line[:col] in line names: the longest
// run of words before it that names a file, so a path with a space counts.
func located(line, dir string) (Finding, bool) {
	for _, m := range position.FindAllStringSubmatchIndex(line, -1) {
		n, _ := strconv.Atoi(line[m[2]:m[3]])
		for start := 0; start < m[0]; start++ {
			if start > 0 && !unicode.IsSpace(rune(line[start-1])) {
				continue
			}
			if path := strings.TrimSpace(line[start:m[0]]); path != "" && fsx.IsFile(fsx.Abs(dir, path)) {
				return Finding{File: fsx.Abs(dir, path), Line: n, Text: line}, true
			}
		}
	}
	return Finding{}, false
}

// judge reports dead-code gate g by its findings, not its exit code
// (vulture exits 3 and deadcode 0 with findings alike): one on a line
// changed since the merge-base, in a file that changed, fails it, so
// touching a file doesn't inherit its old dead code; a failure with none
// parsed is a tool error.
func (r *runner) judge(g enforce.Gate, body string, o outcome) {
	since, _ := r.changes()
	found := Findings(o.out, g.Dir)
	var touched []string
	for _, f := range found {
		if since.Files[f.File] {
			touched = append(touched, f.File)
		}
	}
	var lines git.ChangedLines
	if len(touched) > 0 {
		// Each file once: thousands of findings in one file mustn't swell git's argv.
		slices.Sort(touched)
		lines = git.LinesChanged(r.plan.Root, since.MergeBase, slices.Compact(touched))
	}
	var blocking []string
	for _, f := range found {
		if since.Files[f.File] && lines.Touches(f.File, f.Line) {
			blocking = append(blocking, f.Text)
		}
	}
	switch {
	case len(blocking) > 0:
		r.fail(g, body, head(strings.Join(blocking, "\n")))
	case o.err != nil && len(found) == 0:
		r.fail(g, body, head(o.out))
	case len(found) > 0:
		r.pass(g.Label, fmt.Sprintf("%d finding%s on unchanged lines", len(found), plural(len(found))))
	default:
		r.pass(g.Label, "")
	}
}
