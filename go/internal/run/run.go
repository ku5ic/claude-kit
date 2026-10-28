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

// Gates runs p's gates in order, covered ones last, writing a PASS, FAIL, or SKIP line for
// each, then a blank line and the summary. A failure's line has its command,
// where it ran, and its binaries under it, then the first lines of its
// output. cacheDir holds the marks of gates that changed files. A gate gets
// what's left before deadline, when one is set, and one starting past it is
// skipped. It returns the counts and the run's notice for the user.
func Gates(p enforce.Plan, cacheDir string, w io.Writer, deadline time.Time) (Result, string) {
	r := &runner{reporter: reporter{plan: p, report: w}, cacheDir: cacheDir, deadline: deadline, ran: map[string]bool{},
		base:    sync.OnceValue(func() string { base, _ := git.Base(p.Root, ""); return base }),
		changes: sync.OnceValues(func() (git.Since, bool) { return git.ChangedSince(p.Root) }),
	}
	// A scoped plan already asked git.
	if s := p.Since; s != nil {
		r.base = func() string { return s.Base }
		r.changes = func() (git.Since, bool) { return *s, true }
	}
	if p.Discovery != "" {
		r.skip("everything", p.Discovery)
	}
	var covered []enforce.Gate
	for _, g := range p.Gates {
		if g.Covered() {
			covered = append(covered, g)
			continue
		}
		r.gate(g)
	}
	// Last, so each coverer has had its turn: one that didn't run covers
	// nothing.
	for _, g := range covered {
		if !r.ran[g.CoveredBy] {
			g.Skip = ""
		}
		r.gate(g)
	}
	fmt.Fprintf(w, "\n%s\n", r.res)
	return r.res, r.notice("run-checks")
}

type runner struct {
	reporter
	cacheDir string
	deadline time.Time // zero: no budget past each gate's own timeout
	started  bool
	tree     string          // the work tree's snapshot after the last gate; "" outside a repo
	ref      string          // the restore point taken before the first gate
	ran      map[string]bool // the gap-fill keys of the entries whose gates ran
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
	// limit is the time a Stop gate had; outOfTime is true for one that
	// never started, the batch's deadline already past.
	limit     time.Duration
	outOfTime bool
}

// notFound is true when the shell couldn't find a command the gate runs.
func (o outcome) notFound() bool {
	exit, ok := errors.AsType[*exec.ExitError](o.err)
	return ok && exit.ExitCode() == 127
}

// gate runs g and reports it. A dead-code gate is judged by its findings
// on lines changed since the git base; it doesn't run with no base, and
// passes unrun with nothing changed.
func (r *runner) gate(g enforce.Gate) {
	if g.Skip != "" {
		r.skip(g.Label, g.Skip)
		return
	}
	if g.Unchanged != "" {
		r.pass(g.Label, "nothing changed since "+g.Unchanged)
		return
	}
	body := g.Command()
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
	timeout, ok := limit(gateTimeout, r.deadline)
	if !ok {
		r.cut(g.Label)
		return
	}
	o := r.exec(g, body, timeout)
	// Killed for the budget, not its own timeout: it never got to finish.
	cut := o.timedOut && timeout < gateTimeout
	// Only a run of its whole or affected body covers another gate.
	if g.Entry != "" && len(g.Files) == 0 && !cut && !o.notFound() {
		r.ran[g.Entry] = true
	}
	// A failure describes what ran: the body with its files and {base}
	// filled in.
	ran := g
	ran.Body, ran.Files = body, nil
	switch {
	case len(o.changed) > 0:
		_ = enforce.AddMark(r.cacheDir, r.plan.Root, g, enforce.Mark{Label: g.Label, Paths: o.changed, Ref: r.ref})
		detail := changedLine(o.changed)
		if o.err != nil {
			detail += head(o.out)
		}
		r.fail(ran, detail)
	case o.notFound():
		r.skip(g.Label, "not installed: "+lastLine(o.out))
	case cut:
		r.cut(g.Label)
	case g.Scoped():
		r.judge(ran, o)
	case o.err != nil:
		r.fail(ran, head(o.out))
	default:
		r.pass(g.Label, "")
	}
}

// limit is timeout cut to what's left before deadline, when one is set;
// false when nothing is left.
func limit(timeout time.Duration, deadline time.Time) (time.Duration, bool) {
	if !deadline.IsZero() {
		timeout = min(timeout, time.Until(deadline))
	}
	return timeout, timeout > 0
}

// exec runs body as g says, killed after timeout, with the snapshots around
// it. The first snapshot is kept as the run's restore point.
func (r *runner) exec(g enforce.Gate, body string, timeout time.Duration) outcome {
	root := r.plan.Root
	if !r.started {
		r.started = true
		if r.tree, _ = git.Snapshot(root); r.tree != "" {
			r.ref = keep(root, r.tree, time.Now())
		}
	}
	cmd := command(g, body, timeout)
	cmd.Interruptible()
	o := execute(cmd)
	if o.timedOut {
		o.out += fmt.Sprintf("\ntimed out after %s\n", timeout.Round(time.Second))
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
	// A timeout kills the workers a test runner spawned, not only bash.
	cmd.KillGroup()
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

// changedLine is the detail under a gate that changed paths.
func changedLine(paths []string) string {
	return "  changed: " + strings.Join(paths, ", ") + "; it won't run again until kit gates reset, and kit gates reset --restore puts them back\n"
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
