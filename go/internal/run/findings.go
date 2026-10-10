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
// whatever the tool: a path:line[:col] anywhere in a line; a line led by a
// file's path, then the indented line:col lines under it, up to a blank
// line (eslint's stylish); or, with none under it, that line alone, a
// finding without a line (an unused file, a file to format). A path counts
// only when it names an existing file.
func Findings(out, dir string) []Finding {
	var found []Finding
	var led *Finding // the last line led by a file's path
	listed := false  // indented findings followed it
	end := func() {
		if led != nil && !listed {
			found = append(found, *led)
		}
		led = nil
	}
	for line := range strings.Lines(out) {
		line = strings.TrimRight(line, "\r\n")
		if m := indented.FindStringSubmatch(line); m != nil && led != nil {
			n, _ := strconv.Atoi(m[1])
			found = append(found, Finding{File: led.File, Line: n, Text: led.File + ":" + strings.TrimSpace(line)})
			listed = true
			continue
		}
		if f, ok := located(line, dir); ok {
			end()
			found = append(found, f)
		} else if file := leadingFile(line, dir); file != "" {
			end()
			led, listed = &Finding{File: file, Text: strings.TrimSpace(line)}, false
		} else if strings.TrimSpace(line) == "" {
			end()
		}
	}
	end()
	return found
}

// leadingFile is the existing file whose path leads line, the longest run
// of its words that names one; "" when none does.
func leadingFile(line, dir string) string {
	line = strings.TrimSpace(line)
	for end := len(line); end > 0; end-- {
		if end < len(line) && !unicode.IsSpace(rune(line[end])) {
			continue
		}
		if path := fsx.Abs(dir, line[:end]); fsx.IsFile(path) {
			return path
		}
	}
	return ""
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
