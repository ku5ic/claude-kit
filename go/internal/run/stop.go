package run

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/git"
)

// Checked is one Stop run: a line per gate for kit explain stop, the
// message a block shows, and whether to block.
type Checked struct {
	Report, Failures string
	Failed           bool
	lines, summary   string
}

// Notice is the report for the user: the summary, then each gate's line
// under it.
func (c Checked) Notice() string {
	indented := strings.ReplaceAll(strings.TrimSuffix(c.lines, "\n"), "\n", "\n  ")
	return "stop " + strings.TrimSuffix(c.summary, "\n") + "\n  " + indented
}

// Files runs p's gates, the Stop hook's checks of the turn's edits, all at
// once, each killed with its children once timeout passes. A gate of a
// lint-like kind fails only on findings on lines changed since HEAD in the
// files it checked; any other on any failure. A snapshot around the batch
// catches a gate that changed files: the block names them, and the gates
// that checked them are marked so they never run again.
func Files(p enforce.Plan, cacheDir string, timeout time.Duration) Checked {
	before, _ := git.Snapshot(p.Root)
	results := make([]outcome, len(p.Gates))
	var wg sync.WaitGroup
	for i, g := range p.Gates {
		if g.Skip == "" {
			wg.Go(func() {
				// A panic here would exit 2, which Claude Code reads as a
				// block; the hook's recover can't reach this goroutine.
				defer func() {
					if recover() != nil {
						results[i] = outcome{crashed: true}
					}
				}()
				cmd := command(g, g.Command(), timeout)
				cmd.KillGroup()
				results[i] = execute(cmd)
			})
		}
	}
	wg.Wait()
	var suspects map[int]bool
	var changed []string
	if before != "" {
		if after, err := git.Snapshot(p.Root); err == nil && after != before {
			suspects, changed = mutators(p, before, after, cacheDir)
		}
	}
	t := tally{plan: p, lines: sync.OnceValue(func() git.ChangedLines { return git.LinesChanged(p.Root, "HEAD", edited(p.Gates)) })}
	for i, g := range p.Gates {
		if !suspects[i] {
			t.record(g, results[i], timeout)
			continue
		}
		// As in the full gate: a gate that changed files fails as that.
		detail := changedLine(changed)
		if results[i].err != nil {
			detail += tail(results[i].out)
		}
		t.fail(g, detail)
	}
	summary := fmt.Sprintf("checks: %d passed, %d failed, %d skipped\n", t.res.Pass, t.res.Fail, t.res.Skip)
	return Checked{Report: t.report.String() + summary, Failures: t.failures.String() + summary, Failed: t.res.Fail > 0, lines: t.report.String(), summary: summary}
}

// tally builds a Stop run's report and block message.
type tally struct {
	plan             enforce.Plan
	lines            func() git.ChangedLines
	res              Result
	report, failures strings.Builder
}

func (t *tally) record(g enforce.Gate, o outcome, timeout time.Duration) {
	switch {
	case g.Skip != "":
		t.skip(g.Label, g.Skip)
	case o.crashed:
		t.skip(g.Label, "crashed; failing open")
	case o.timedOut:
		t.skip(g.Label, fmt.Sprintf("timed out after %s", timeout))
	case o.notFound():
		t.skip(g.Label, "not installed: "+lastLine(o.out))
	case o.err == nil:
		t.pass(g.Label, "")
	case g.LintLike():
		t.findings(g, o)
	default:
		t.fail(g, tail(o.out))
	}
}

// findings judges a failed lint-like gate by what it found: a finding on a
// changed line of a file it checked fails it, the rest are old; output with
// no finding in it fails it whole.
func (t *tally) findings(g enforce.Gate, o outcome) {
	found := Findings(o.out, g.Dir)
	if len(found) == 0 {
		t.fail(g, tail(o.out))
		return
	}
	var blocking []string
	for _, f := range found {
		if slices.Contains(g.Files, f.File) && t.lines().Touches(f.File, f.Line) {
			blocking = append(blocking, f.Text)
		}
	}
	old := len(found) - len(blocking)
	switch {
	case len(blocking) == 0:
		t.pass(g.Label, fmt.Sprintf("%d finding%s on unchanged lines", old, plural(old)))
	case old > 0:
		t.fail(g, head(strings.Join(blocking, "\n"))+fmt.Sprintf("(%d more on unchanged lines don't block)\n", old))
	default:
		t.fail(g, head(strings.Join(blocking, "\n")))
	}
}

func (t *tally) skip(label, why string) {
	line := fmt.Sprintf("SKIP %s (%s)\n", label, why)
	// In a block's message too, so the skipped count has its reasons.
	t.report.WriteString(line)
	t.failures.WriteString(line)
	t.res.Skip++
}

func (t *tally) pass(label, note string) {
	if note != "" {
		label += " (" + note + ")"
	}
	fmt.Fprintf(&t.report, "PASS %s\n", label)
	t.res.Pass++
}

func (t *tally) fail(g enforce.Gate, detail string) {
	fmt.Fprintf(&t.report, "FAIL %s\n", g.Label)
	fmt.Fprintf(&t.failures, "FAIL %s\n", g.Label)
	t.plan.Describe(&t.failures, g)
	t.failures.WriteString(detail)
	t.res.Fail++
}

// mutators are the gates of a batch that changed files, by index: those
// that checked a changed file, or, when none did, every gate that ran. Each
// is marked, the snapshot before the batch its restore point; changed is
// the files.
func mutators(p enforce.Plan, before, after, cacheDir string) (suspects map[int]bool, changed []string) {
	changed, _ = git.TreeDiff(p.Root, before, after)
	if len(changed) == 0 {
		return nil, nil
	}
	suspects, ran := map[int]bool{}, map[int]bool{}
	for i, g := range p.Gates {
		if g.Skip != "" {
			continue
		}
		ran[i] = true
		if slices.ContainsFunc(changed, func(path string) bool { return slices.Contains(g.Files, p.Root+"/"+path) }) {
			suspects[i] = true
		}
	}
	if len(suspects) == 0 {
		suspects = ran
	}
	ref := keep(p.Root, before, time.Now())
	for i := range suspects {
		g := p.Gates[i]
		_ = enforce.AddMark(cacheDir, p.Root, g, enforce.Mark{Label: g.Label, Paths: changed, Ref: ref})
	}
	return suspects, changed
}

// edited is every file p's gates check.
func edited(gates []enforce.Gate) []string {
	var files []string
	for _, g := range gates {
		for _, f := range g.Files {
			if !slices.Contains(files, f) {
				files = append(files, f)
			}
		}
	}
	return files
}

// tail is out's last maxOutputLines lines: linters print findings and the
// summary last, after preambles.
func tail(out string) string {
	if out = strings.TrimRight(out, "\n"); out == "" {
		return ""
	}
	lines := strings.Split(out, "\n")
	return strings.Join(lines[max(0, len(lines)-maxOutputLines):], "\n") + "\n"
}
