// Package stats measures the kit from its own logs: how often each rule
// blocks, or only would have (shadow_rules), what became of each claim the
// claim checker flagged in the text that went out after it, and how many
// calls auto mode denied.
package stats

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

// Kept is true when one of statements still states claim: it holds 70% or
// more of the claim's words of three letters or more. A flag the writer cut,
// or rewrote around what the check found, reads as dropped.
func Kept(claim string, statements []string) bool {
	want := words(claim)
	if len(want) == 0 {
		return false
	}
	return slices.ContainsFunc(statements, func(s string) bool {
		have := words(s)
		n := 0
		for _, w := range want {
			if slices.Contains(have, w) {
				n++
			}
		}
		return n*10 >= len(want)*7
	})
}

// words is s's distinct lowercased words of three letters or more.
func words(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(w) >= 3 && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}

// Options narrows a report: lines from Since on, and one rule's alone.
type Options struct {
	Since time.Time
	Rule  string
}

// line is the fields a report reads from either log.
type line struct {
	TS      string `json:"ts"`
	Event   string `json:"event"`
	Session string `json:"session_id"`
	Rule    string `json:"rule"`
	Cost    string `json:"cost_usd"`
	Source  string `json:"source"`
	Model   string `json:"model"`
	Outcome string `json:"outcome"`
}

// ruleRow is one rule's counts: an ask is a permission prompt the user
// saw, and a skip is a marker that waived the rule, such as a plan's
// critic-skip line.
type ruleRow struct {
	block, ask, shadow, disabled, skipped int
	sessions                              map[string]bool
}

// claimRow is one source and model's flags.
type claimRow struct{ kept, dropped int }

// Report writes the rule and claim tables from the guards and claims logs
// at those paths.
func Report(guards, claims string, o Options, w io.Writer) error {
	g := guardTally{rules: map[string]*ruleRow{}}
	if err := each(guards, o, func(l line) { g.add(l, o.Rule) }); err != nil {
		return err
	}
	flags := map[[2]string]*claimRow{}
	err := each(claims, o, func(l line) {
		key := [2]string{l.Source, l.Model}
		if flags[key] == nil {
			flags[key] = &claimRow{}
		}
		if l.Outcome == "kept" {
			flags[key].kept++
		} else {
			flags[key].dropped++
		}
	})
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "rule\tblocks\tasks\tshadow\tdisabled\tskipped\tsessions")
	for _, name := range slices.Sorted(maps.Keys(g.rules)) {
		r := g.rules[name]
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%d\n", name, r.block, r.ask, r.shadow, r.disabled, r.skipped, len(r.sessions))
	}
	if o.Rule == "" || o.Rule == "claims" {
		fmt.Fprintln(tw, "\nclaims\tmodel\tflagged\tdropped\tkept")
		keys := slices.Collect(maps.Keys(flags))
		slices.SortFunc(keys, func(a, b [2]string) int { return slices.Compare(a[:], b[:]) })
		for _, key := range keys {
			f := flags[key]
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\n", key[0], key[1], f.kept+f.dropped, f.dropped, f.kept)
		}
		fmt.Fprintf(tw, "\nhook runs\t%d\t$%.2f\n", g.hookRuns, g.hookCost)
	}
	if o.Rule == "" {
		fmt.Fprintf(tw, "\nauto-mode denials\t%d\n", g.denials)
	}
	return tw.Flush()
}

// guardTally is the guards log's counts: per rule, auto-mode denials, and
// the claims hook's runs and cost.
type guardTally struct {
	rules             map[string]*ruleRow
	hookRuns, denials int
	hookCost          float64
}

// add counts l, a rule's event only when only is "" or names its rule.
func (g *guardTally) add(l line, only string) {
	switch l.Event {
	case "block", "ask", "shadow", "disabled", "critic-skip":
		// An ask logged before asks carried a rule has none to count under.
		if only != "" && l.Rule != only || l.Event == "ask" && l.Rule == "" {
			return
		}
		g.rule(l.Rule).count(l)
	case kitlog.EventAutoDenied:
		g.denials++
	case "claims", "claims-error":
		var cost float64
		if _, err := fmt.Sscan(l.Cost, &cost); err == nil {
			g.hookRuns++
			g.hookCost += cost
		}
	}
}

// rule is name's row, added on first use.
func (g *guardTally) rule(name string) *ruleRow {
	r := g.rules[name]
	if r == nil {
		r = &ruleRow{sessions: map[string]bool{}}
		g.rules[name] = r
	}
	return r
}

// count adds l's event and session to r.
func (r *ruleRow) count(l line) {
	r.sessions[l.Session] = true
	switch l.Event {
	case "block":
		r.block++
	case "ask":
		r.ask++
	case "shadow":
		r.shadow++
	case "critic-skip":
		r.skipped++
	default:
		r.disabled++
	}
}

// Unused writes each of rules with no line in the guards log, as blocked,
// shadowed, disabled, or skipped: one that never fires may catch nothing, or
// nothing has come up for it to catch.
func Unused(guards string, rules []string, o Options, w io.Writer) error {
	fired := map[string]bool{}
	if err := each(guards, o, func(l line) { fired[l.Rule] = true }); err != nil {
		return err
	}
	for _, rule := range rules {
		if !fired[rule] && (o.Rule == "" || o.Rule == rule) {
			fmt.Fprintln(w, rule)
		}
	}
	return nil
}

// each calls fn with every line of the log at path from o.Since on; a line
// that isn't JSON is skipped.
func each(path string, o Options, fn func(line)) error {
	return kitlog.Each(path, func(raw []byte) {
		var l line
		if json.Unmarshal(raw, &l) != nil {
			return
		}
		if ts, err := time.Parse(kitlog.TimeLayout, l.TS); err == nil && ts.Before(o.Since) {
			return
		}
		fn(l)
	})
}

// Run is `kit stats [--rule r] [--since days] [--unused]`: the report over
// the last days, 30 unless --since says otherwise, from the logs under
// logDir, or with --unused which of rules it has no event for.
func Run(logDir string, rules []string, now time.Time, args []string, stdout, stderr io.Writer) int {
	o, days, unused := Options{}, 30, false
	for i := 0; i < len(args); i++ {
		switch flag := args[i]; {
		case flag == "--unused":
			unused = true
		case flag == "--rule" && i+1 < len(args):
			i++
			o.Rule = args[i]
		case flag == "--since" && i+1 < len(args):
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintf(stderr, "stats: --since takes a number of days, got %q\n", args[i])
				return 2
			}
			days = n
		default:
			fmt.Fprintf(stderr, "stats: unknown argument %q; usage: kit stats [--rule r] [--since days] [--unused]\n", flag)
			return 2
		}
	}
	o.Since = now.AddDate(0, 0, -days)
	guards := filepath.Join(logDir, kitlog.Guards+".jsonl")
	var err error
	if unused {
		fmt.Fprintf(stdout, "no event in %d days\n", days)
		err = Unused(guards, rules, o, stdout)
	} else {
		err = Report(guards, filepath.Join(logDir, kitlog.Claims+".jsonl"), o, stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, "stats:", err)
		return 1
	}
	return 0
}
