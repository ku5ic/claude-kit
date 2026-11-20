package run

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/layout"
)

// Result counts a run's verdicts; Unchecked counts the skips that leave
// code unchecked, the deadline's cuts among them, which make the run
// incomplete.
type Result struct{ Pass, Fail, Skip, Unchecked int }

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
	verdicts         []verdict // each gate's receipt row
}

// storePrefix is a path's way into node_modules, a package manager's store
// included (.pnpm/<id>/node_modules/), so an error path starts at its
// package.
var storePrefix = regexp.MustCompile(`(?:[^\s(]*/)?node_modules/(?:\.pnpm/[^/\s]+/node_modules/)?`)

// shownDetail is how many lines of a failure's detail a block shows.
const shownDetail = 3

// Receipt is a run's verdicts for the user. By names the run; the zero
// rest is a run with no gates.
type Receipt struct {
	By       string
	res      Result
	verdicts []verdict
}

// verdict is one gate's receipt row: its kind (FAIL, skip, pass), its
// label, and a skip's reason.
type verdict struct{ kind, label, why string }

// String is the receipt: a headline with the counts, then one row per
// failure and skip, then the passes on one line, never a failure's
// detail. extra lines (files nothing checked, overrides) go last. A blank
// line sets each group apart.
func (rc Receipt) String(extra ...string) string {
	head := fmt.Sprintf("kit %s: %d failed, %d skipped, %d passed", rc.By, rc.res.Fail, rc.res.Skip, rc.res.Pass)
	var rows, passes []string
	for _, kind := range []string{"FAIL", "skip"} {
		for _, v := range rc.verdicts {
			if v.kind != kind {
				continue
			}
			text := v.label
			if v.why != "" {
				text += "  " + layout.Clip(v.why)
			}
			rows = append(rows, layout.KV(kind, text))
		}
	}
	for _, v := range rc.verdicts {
		if v.kind == "pass" {
			passes = append(passes, v.label)
		}
	}
	if len(passes) > 0 {
		rows = append(rows, layout.KV("pass", passLine(passes)))
	}
	groups := []string{head}
	for _, g := range [][]string{rows, extra} {
		if len(g) > 0 {
			groups = append(groups, "  "+strings.Join(g, "\n  "))
		}
	}
	return strings.Join(groups, "\n\n")
}

// PassesOnly is rc without its failure and skip rows, for a receipt shown
// beside a block that names them with their detail.
func (rc Receipt) PassesOnly() Receipt {
	rc.verdicts = slices.DeleteFunc(slices.Clone(rc.verdicts), func(v verdict) bool { return v.kind != "pass" })
	return rc
}

// Failures is the label of each gate that failed, in run order.
func (rc Receipt) Failures() []string {
	var labels []string
	for _, v := range rc.verdicts {
		if v.kind == "FAIL" {
			labels = append(labels, v.label)
		}
	}
	return labels
}

// subLabel is a label's trailing subproject, " [web]".
var subLabel = regexp.MustCompile(` \[([^\]]+)\]`)

// passLine is labels as their kinds, grouped by subproject in first-seen
// order, a repeated kind counted: "web: lint, test; api: test x2". A
// label's origin, the first parenthesis, is left to the report.
func passLine(labels []string) string {
	var subs []string
	kinds := map[string][]string{}
	for _, l := range labels {
		sub := ""
		if m := subLabel.FindStringSubmatch(l); m != nil {
			sub, l = m[1], strings.Replace(l, m[0], "", 1)
		}
		if open := strings.Index(l, " ("); open >= 0 {
			if end := strings.Index(l[open:], ")"); end >= 0 {
				l = l[:open] + l[open+end+1:]
			}
		}
		if _, seen := kinds[sub]; !seen {
			subs = append(subs, sub)
		}
		kinds[sub] = append(kinds[sub], l)
	}
	var parts []string
	for _, sub := range subs {
		var names []string
		count := map[string]int{}
		for _, name := range kinds[sub] {
			if count[name]++; count[name] == 1 {
				names = append(names, name)
			}
		}
		for i, name := range names {
			if n := count[name]; n > 1 {
				names[i] = fmt.Sprintf("%s x%d", name, n)
			}
		}
		part := strings.Join(names, ", ")
		if sub != "" {
			part = sub + ": " + part
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

// Passed is true when no gate failed.
func (rc Receipt) Passed() bool { return rc.res.Fail == 0 }

// Clean is true when every gate passed.
func (rc Receipt) Clean() bool { return rc.res.Fail == 0 && rc.res.Skip == 0 }

func (r *reporter) receipt(by string) Receipt { return Receipt{by, r.res, r.verdicts} }

// verdict writes line to the report and keeps v for the receipt.
func (r *reporter) verdict(line string, v verdict) {
	fmt.Fprint(r.report, line)
	r.verdicts = append(r.verdicts, v)
}

// skip reports label skipped, leaving code unchecked.
func (r *reporter) skip(label, why string) {
	r.skipped(label, why)
	r.res.Unchecked++
}

// skipped reports label skipped for why.
func (r *reporter) skipped(label, why string) {
	r.verdict(fmt.Sprintf("SKIP %s (%s)\n", label, why), verdict{"skip", label, why})
	if r.failures != nil {
		// In a block's message too, so the skipped count has its reasons,
		// clipped there and whole in the detail it points at.
		fmt.Fprintln(r.failures, layout.KV("skip", label+"  "+layout.Clip(why)))
		fmt.Fprintf(r.detail, "SKIP %s (%s)\n", label, why)
	}
	r.res.Skip++
}

// cut skips label for the deadline.
func (r *reporter) cut(label string) {
	r.skip(label, "no time left in the hook's budget")
}

func (r *reporter) pass(label, note string) {
	if note != "" {
		label += " (" + note + ")"
	}
	r.verdict(fmt.Sprintf("PASS %s\n", label), verdict{"pass", label, ""})
	r.res.Pass++
}

// fail writes g's FAIL line, then what it ran and detail under it.
func (r *reporter) fail(g enforce.Gate, detail string) {
	r.verdict(fmt.Sprintf("FAIL %s\n", g.Label), verdict{"FAIL", g.Label, ""})
	described := r.report
	if r.failures != nil {
		described = r.detail
		fmt.Fprintf(described, "FAIL %s\n", g.Label)
		fmt.Fprintln(r.failures, layout.KV("FAIL", g.Label))
		lines := strings.Split(strings.TrimRight(detail, "\n"), "\n")
		lines = slices.DeleteFunc(lines, func(l string) bool { return strings.TrimSpace(l) == "" })
		for _, l := range lines[:min(len(lines), shownDetail)] {
			fmt.Fprintln(r.failures, layout.KV("", storePrefix.ReplaceAllString(strings.TrimSpace(l), "")))
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
