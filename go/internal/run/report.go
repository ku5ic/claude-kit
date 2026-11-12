package run

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/layout"
)

// Result counts a run's verdicts; Cut counts the skips the deadline cut
// short, which leave the run incomplete.
type Result struct{ Pass, Fail, Skip, Cut int }

// String is the summary line a run ends with.
func (res Result) String() string {
	return fmt.Sprintf("checks: %d passed, %d failed, %d skipped", res.Pass, res.Fail, res.Skip)
}

// reporter writes a run's PASS, FAIL, and SKIP lines to report and counts
// them. With failures set, each failure gets a short entry there, its
// first lines of detail under its FAIL line, and is described in full in
// detail instead of report; each skip is written to failures too.
type reporter struct {
	plan             enforce.Plan
	report           io.Writer
	failures, detail io.Writer
	res              Result
	verdicts         []string // each gate's receipt line, failures first once sorted
}

// shownDetail is how many lines of a failure's detail a block shows.
const shownDetail = 3

// Receipt is a run's verdicts for the user. By names the run; the zero
// rest is a run with no gates.
type Receipt struct {
	By    string
	res   Result
	lines []string
}

// String is the receipt: a headline with the counts, then one line per
// gate, failures first, then skips, then passes, never a failure's detail.
// extra lines (files nothing checked, overrides) go last.
func (rc Receipt) String(extra ...string) string {
	order := map[string]int{"FAIL": 0, "skip": 1, "pass": 2}
	lines := slices.Clone(rc.lines)
	slices.SortStableFunc(lines, func(a, b string) int { return order[strings.Fields(a)[0]] - order[strings.Fields(b)[0]] })
	head := fmt.Sprintf("kit %s: %d pass, %d skipped, %d failed", rc.By, rc.res.Pass, rc.res.Skip, rc.res.Fail)
	return strings.Join(append(append([]string{head}, lines...), extra...), "\n")
}

// Clean is true when every gate passed.
func (rc Receipt) Clean() bool { return rc.res.Fail == 0 && rc.res.Skip == 0 }

func (r *reporter) receipt(by string) Receipt { return Receipt{by, r.res, r.verdicts} }

// verdict writes line to the report and keeps kind and text for the receipt.
func (r *reporter) verdict(line, kind, text string) {
	fmt.Fprint(r.report, line)
	r.verdicts = append(r.verdicts, layout.KV(kind, text))
}

func (r *reporter) skip(label, why string) {
	r.verdict(fmt.Sprintf("SKIP %s (%s)\n", label, why), "skip", label+"  "+why)
	if r.failures != nil {
		// In a block's message too, so the skipped count has its reasons.
		fmt.Fprintln(r.failures, layout.KV("skip", label+" ("+why+")"))
	}
	r.res.Skip++
}

// cut skips label for the deadline.
func (r *reporter) cut(label string) {
	r.skip(label, "no time left in the hook's budget")
	r.res.Cut++
}

func (r *reporter) pass(label, note string) {
	if note != "" {
		label += " (" + note + ")"
	}
	r.verdict(fmt.Sprintf("PASS %s\n", label), "pass", label)
	r.res.Pass++
}

// fail writes g's FAIL line, then what it ran and detail under it.
func (r *reporter) fail(g enforce.Gate, detail string) {
	r.verdict(fmt.Sprintf("FAIL %s\n", g.Label), "FAIL", g.Label)
	described := r.report
	if r.failures != nil {
		described = r.detail
		fmt.Fprintf(described, "FAIL %s\n", g.Label)
		fmt.Fprintln(r.failures, layout.KV("FAIL", g.Label))
		lines := strings.Split(strings.TrimRight(detail, "\n"), "\n")
		lines = slices.DeleteFunc(lines, func(l string) bool { return strings.TrimSpace(l) == "" })
		for _, l := range lines[:min(len(lines), shownDetail)] {
			fmt.Fprintln(r.failures, layout.KV("", strings.TrimSpace(l)))
		}
		if len(lines) > shownDetail {
			fmt.Fprintln(r.failures, layout.KV("", fmt.Sprintf("+%d more lines", len(lines)-shownDetail)))
		}
	}
	r.plan.Describe(described, g)
	fmt.Fprint(described, detail)
	r.res.Fail++
}

// blocking is the text of each finding on a line inScope says changed.
func blocking(found []Finding, inScope func(Finding) bool) []string {
	var out []string
	for _, f := range found {
		if inScope(f) {
			out = append(out, f.Text)
		}
	}
	return out
}

// unchanged is the pass note for n findings that don't block.
func unchanged(n int) string {
	return fmt.Sprintf("%d finding%s on unchanged lines", n, plural(n))
}
