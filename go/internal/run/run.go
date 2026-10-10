// Package run executes the full gate enforce plans, one gate at a time,
// and reports each as PASS, FAIL, or SKIP. A content snapshot of the work
// tree before and after each gate catches one that changes files: it fails,
// and is marked so it never runs again. The snapshot taken before the first
// gate is kept as a restore point.
package run

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/proc"
)

// gateTimeout bounds one gate: the Bash tool's own ceiling, past which the
// whole run would be killed anyway.
const gateTimeout = 10 * time.Minute

// maxOutputLines bounds the output a failing gate shows.
const maxOutputLines = 30

// Summary matches the line a run ends with; review-checks reads it as proof
// the reviewer ran the full gate.
var Summary = regexp.MustCompile(`checks: \d+ passed, \d+ failed, \d+ skipped`)

// Result counts a run's verdicts.
type Result struct{ Pass, Fail, Skip int }

// Gates runs p's gates in order, writing a PASS, FAIL, or SKIP line for
// each, then a blank line and the summary. A failure's line has its command,
// where it ran, and its binaries under it, then the first lines of its
// output. cacheDir holds the marks of gates that changed files.
func Gates(p enforce.Plan, cacheDir string, w io.Writer) Result {
	r := &runner{plan: p, cacheDir: cacheDir, w: w,
		base:    sync.OnceValue(func() string { base, _ := git.Base(p.Root, ""); return base }),
		changes: sync.OnceValues(func() (git.Since, bool) { return git.ChangedSince(p.Root) }),
	}
	if p.Discovery != "" {
		r.skip("everything", p.Discovery)
	}
	for _, g := range p.Gates {
		r.gate(g)
	}
	fmt.Fprintf(w, "\nchecks: %d passed, %d failed, %d skipped\n", r.res.Pass, r.res.Fail, r.res.Skip)
	return r.res
}

type runner struct {
	plan     enforce.Plan
	cacheDir string
	w        io.Writer
	res      Result
	started  bool
	tree     string // the work tree's snapshot after the last gate; "" outside a repo
	ref      string // the restore point taken before the first gate
	base     func() string
	changes  func() (git.Since, bool)
}

// outcome is one gate's run: its combined output, its error, whether its
// deadline killed it, and the paths, from the root, whose content it
// changed.
type outcome struct {
	out      string
	err      error
	timedOut bool
	crashed  bool
	changed  []string
}

// notFound is true when the shell couldn't find a command the gate runs.
func (o outcome) notFound() bool {
	exit, ok := errors.AsType[*exec.ExitError](o.err)
	return ok && exit.ExitCode() == 127
}

func (r *runner) skip(label, why string) {
	fmt.Fprintf(r.w, "SKIP %s (%s)\n", label, why)
	r.res.Skip++
}

func (r *runner) pass(label, note string) {
	if note != "" {
		label += " (" + note + ")"
	}
	fmt.Fprintf(r.w, "PASS %s\n", label)
	r.res.Pass++
}

// fail writes g's FAIL line, what it ran under it, and detail.
func (r *runner) fail(g enforce.Gate, body, detail string) {
	fmt.Fprintf(r.w, "FAIL %s\n", g.Label)
	g.Body = body
	r.plan.Describe(r.w, g)
	fmt.Fprint(r.w, detail)
	r.res.Fail++
}

// gate runs g and reports it. A dead-code gate is judged by its findings
// on lines changed since the git base; it doesn't run with no base, and
// passes unrun with nothing changed.
func (r *runner) gate(g enforce.Gate) {
	if g.Skip != "" {
		r.skip(g.Label, g.Skip)
		return
	}
	body := g.Body
	if strings.Contains(body, "{base}") {
		if r.base() == "" {
			r.skip(g.Label, "no git base")
			return
		}
		body = strings.ReplaceAll(body, "{base}", r.base())
	}
	if g.Scoped() {
		switch since, ok := r.changes(); {
		case !ok:
			r.skip(g.Label, "no git base")
			return
		case len(since.Files) == 0:
			r.pass(g.Label, "nothing changed since "+since.Base)
			return
		}
	}
	o := r.exec(g, body)
	switch {
	case len(o.changed) > 0:
		_ = enforce.AddMark(r.cacheDir, r.plan.Root, g, enforce.Mark{Label: g.Label, Paths: o.changed, Ref: r.ref})
		detail := "  changed: " + strings.Join(o.changed, ", ") + "; it won't run again until kit gates reset, and kit gates reset --restore puts them back\n"
		if o.err != nil {
			detail += head(o.out)
		}
		r.fail(g, body, detail)
	case o.notFound():
		r.skip(g.Label, "not installed: "+lastLine(o.out))
	case g.Scoped():
		r.judge(g, body, o)
	case o.err != nil:
		r.fail(g, body, head(o.out))
	default:
		r.pass(g.Label, "")
	}
}

// exec runs body as g says, with the snapshots around it. The first
// snapshot is kept as the run's restore point.
func (r *runner) exec(g enforce.Gate, body string) outcome {
	root := r.plan.Root
	if !r.started {
		r.started = true
		if r.tree, _ = git.Snapshot(root); r.tree != "" {
			r.ref = keep(root, r.tree, time.Now())
		}
	}
	o := execute(command(g, body, gateTimeout))
	if o.timedOut {
		o.out += fmt.Sprintf("\ntimed out after %s\n", gateTimeout)
	}
	if r.tree != "" {
		if after, err := git.Snapshot(root); err == nil {
			o.changed, _ = git.TreeDiff(root, r.tree, after)
			r.tree = after
		}
	}
	return o
}

// command is body run as g says, in its directory with its env, killed
// once timeout passes.
func command(g enforce.Gate, body string, timeout time.Duration) *proc.Cmd {
	// GitHub Actions' default shell for a run step.
	cmd := proc.Command(timeout, "bash", "-e", "-o", "pipefail", "-c", body)
	cmd.Dir = g.Dir
	cmd.Env = append(append(os.Environ(), g.Env...), "PATH="+searchPath(g))
	return cmd
}

// execute runs cmd for its combined output.
func execute(cmd *proc.Cmd) outcome {
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	return outcome{out: buf.String(), err: err, timedOut: cmd.TimedOut()}
}

// searchPath is PATH with the directory of each binary g resolved ahead of
// it, unless PATH already finds that copy first.
func searchPath(g enforce.Gate) string {
	var dirs []string
	for _, bin := range g.Bins {
		if found, err := exec.LookPath(filepath.Base(bin.Path)); err != nil || found != bin.Path {
			dirs = append(dirs, filepath.Dir(bin.Path))
		}
	}
	if path := os.Getenv("PATH"); path != "" {
		dirs = append(dirs, path)
	}
	return strings.Join(dirs, string(os.PathListSeparator))
}

// head is out's first maxOutputLines lines, each ending in a newline.
func head(out string) string {
	if out = strings.TrimRight(out, "\n"); out == "" {
		return ""
	}
	lines := strings.SplitAfter(out, "\n")
	return strings.Join(lines[:min(len(lines), maxOutputLines)], "") + "\n"
}

// lastLine is out's last non-empty line.
func lastLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
