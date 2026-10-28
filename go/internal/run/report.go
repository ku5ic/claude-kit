package run

import (
	"fmt"
	"io"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
)

// Result counts a run's verdicts.
type Result struct{ Pass, Fail, Skip int }

// String is the summary line a run ends with.
func (res Result) String() string {
	return fmt.Sprintf("checks: %d passed, %d failed, %d skipped", res.Pass, res.Fail, res.Skip)
}

// reporter writes a run's PASS, FAIL, and SKIP lines to report and counts
// them. With failures set, a failure is described there instead, under its
// own FAIL line, and each skip is written there too.
type reporter struct {
	plan     enforce.Plan
	report   io.Writer
	failures io.Writer
	res      Result
	verdicts []string // each gate's verdict line, for the user's notice
}

// notice is the run for the user: what ran it, the summary, then each
// gate's verdict line under it, never a failure's detail.
func (r *reporter) notice(by string) string {
	return by + " " + r.res.String() + strings.Join(r.verdicts, "")
}

// verdict writes line to the report and keeps it for the notice.
func (r *reporter) verdict(line string) {
	fmt.Fprint(r.report, line)
	r.verdicts = append(r.verdicts, "\n  "+strings.TrimSuffix(line, "\n"))
}

func (r *reporter) skip(label, why string) {
	line := fmt.Sprintf("SKIP %s (%s)\n", label, why)
	r.verdict(line)
	if r.failures != nil {
		// In a block's message too, so the skipped count has its reasons.
		fmt.Fprint(r.failures, line)
	}
	r.res.Skip++
}

func (r *reporter) pass(label, note string) {
	if note != "" {
		label += " (" + note + ")"
	}
	r.verdict(fmt.Sprintf("PASS %s\n", label))
	r.res.Pass++
}

// fail writes g's FAIL line, then what it ran and detail under it.
func (r *reporter) fail(g enforce.Gate, detail string) {
	r.verdict(fmt.Sprintf("FAIL %s\n", g.Label))
	described := r.report
	if r.failures != nil {
		described = r.failures
		fmt.Fprintf(described, "FAIL %s\n", g.Label)
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
