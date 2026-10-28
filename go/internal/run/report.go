package run

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
)

// verdict matches a report's PASS, FAIL, or SKIP line.
var verdict = regexp.MustCompile(`^(PASS|FAIL|SKIP) `)

// Notice is report for the user: its summary, then each verdict line under
// it, indented; a failure's detail is left out. "" when report has no
// summary, so it ran no gate.
func Notice(report string) string {
	summary := Summary.FindString(report)
	if summary == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(summary)
	for line := range strings.Lines(report) {
		if verdict.MatchString(line) {
			b.WriteString("\n  " + strings.TrimSuffix(line, "\n"))
		}
	}
	return b.String()
}

// Result counts a run's verdicts.
type Result struct{ Pass, Fail, Skip int }

// String is the summary line Summary matches.
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
}

func (r *reporter) skip(label, why string) {
	line := fmt.Sprintf("SKIP %s (%s)\n", label, why)
	fmt.Fprint(r.report, line)
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
	fmt.Fprintf(r.report, "PASS %s\n", label)
	r.res.Pass++
}

// fail writes g's FAIL line, then what it ran and detail under it.
func (r *reporter) fail(g enforce.Gate, detail string) {
	fmt.Fprintf(r.report, "FAIL %s\n", g.Label)
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
