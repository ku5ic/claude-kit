// Package stats measures the kit from its own logs: how often each rule
// blocks, or only would have (shadow_rules), and what became of each claim
// the claim checker flagged in the text that went out after it.
package stats

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

// bullet is a list marker a reply line may open with: "- ", "* ", "1. ".
var bullet = regexp.MustCompile(`^\s*(?:[-*]|\d+[.)])\s+`)

// Flagged is each claim a claim-checker reply flags, one `claim -> check`
// line per claim it found unsupported; none for `ok`.
func Flagged(reply string) []string {
	var out []string
	for line := range strings.Lines(reply) {
		claim, _, ok := strings.Cut(bullet.ReplaceAllString(line, ""), " -> ")
		if claim = strings.TrimSpace(claim); ok && claim != "" {
			out = append(out, claim)
		}
	}
	return out
}

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

// ruleRow is one rule's counts.
type ruleRow struct {
	block, shadow, disabled int
	sessions                map[string]bool
}

// claimRow is one source and model's flags.
type claimRow struct{ kept, dropped int }

// Report writes the rule and claim tables from the guards and claims logs
// at those paths.
func Report(guards, claims string, o Options, w io.Writer) error {
	rules := map[string]*ruleRow{}
	var hookRuns int
	var hookCost float64
	err := each(guards, o, func(l line) {
		switch l.Event {
		case "block", "shadow", "disabled":
			if o.Rule != "" && l.Rule != o.Rule {
				return
			}
			r := rules[l.Rule]
			if r == nil {
				r = &ruleRow{sessions: map[string]bool{}}
				rules[l.Rule] = r
			}
			r.sessions[l.Session] = true
			switch l.Event {
			case "block":
				r.block++
			case "shadow":
				r.shadow++
			default:
				r.disabled++
			}
		case "claims", "claims-error":
			var cost float64
			if _, err := fmt.Sscan(l.Cost, &cost); err == nil {
				hookRuns++
				hookCost += cost
			}
		}
	})
	if err != nil {
		return err
	}
	flags := map[[2]string]*claimRow{}
	err = each(claims, o, func(l line) {
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
	fmt.Fprintln(tw, "rule\tblocks\tshadow\tdisabled\tsessions")
	for _, name := range sortedKeys(rules) {
		r := rules[name]
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\n", name, r.block, r.shadow, r.disabled, len(r.sessions))
	}
	if o.Rule == "" || o.Rule == "claims" {
		fmt.Fprintln(tw, "\nclaims\tmodel\tflagged\tdropped\tkept")
		for _, key := range sortedKeys(flags) {
			f := flags[key]
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\n", key[0], key[1], f.kept+f.dropped, f.dropped, f.kept)
		}
		fmt.Fprintf(tw, "\nhook runs\t%d\t$%.2f\n", hookRuns, hookCost)
	}
	return tw.Flush()
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

// sortedKeys is m's keys in order, for a stable report.
func sortedKeys[K interface{ ~string | ~[2]string }, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b K) int { return strings.Compare(fmt.Sprint(a), fmt.Sprint(b)) })
	return keys
}

// Run is `kit stats [--rule r] [--since days]`: the report over the last
// days, 30 unless --since says otherwise, from the logs under logDir.
func Run(logDir string, now time.Time, args []string, stdout, stderr io.Writer) int {
	o, days := Options{}, 30
	for i := 0; i < len(args); i++ {
		switch flag := args[i]; {
		case (flag == "--rule" || flag == "--since") && i+1 < len(args):
			i++
			if flag == "--rule" {
				o.Rule = args[i]
			} else if n, err := strconv.Atoi(args[i]); err == nil && n > 0 {
				days = n
			} else {
				fmt.Fprintf(stderr, "stats: --since takes a number of days, got %q\n", args[i])
				return 2
			}
		default:
			fmt.Fprintf(stderr, "stats: unknown argument %q; usage: kit stats [--rule r] [--since days]\n", flag)
			return 2
		}
	}
	o.Since = now.AddDate(0, 0, -days)
	if err := Report(filepath.Join(logDir, kitlog.Guards+".jsonl"), filepath.Join(logDir, kitlog.Claims+".jsonl"), o, stdout); err != nil {
		fmt.Fprintln(stderr, "stats:", err)
		return 1
	}
	return 0
}
