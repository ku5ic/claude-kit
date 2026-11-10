// Package weaken finds an edit that weakens a check: a test it skips or
// focuses, a linter or type checker it silences, or assertions it removes.
package weaken

import (
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/guard"
)

// allow is the marker that lets one weakening line through: `kit: allow
// <reason>` on the line above it or on the line itself.
var allow = regexp.MustCompile(`kit: allow\s+(.*?\S)\s*(\*/|-->)?\s*$`)

// Hit is one added line that skips a test or silences a check.
type Hit struct {
	Line   int    // 1-based, in the new text
	Text   string // the line, trimmed
	Kind   string // "skip" or "suppression"
	Reason string // its kit: allow reason; "" when it has none
}

// Result is what an edit weakens.
type Result struct {
	Hits []Hit
	// Lost is how many assertion lines a test file has fewer of after.
	Lost int
}

// Check compares path's text before and after an edit. Skips and lost
// assertions count only in a test file; suppressions in any file c
// doesn't skip.
func Check(c config.WeakenedChecks, path, before, after string) Result {
	if guard.GlobAny(c.Skip, path) {
		return Result{}
	}
	test := guard.GlobAny(c.TestFiles, path)
	var r Result
	if test {
		assertions := compile(c.Assertions)
		r.Lost = max(0, count(assertions, before)-count(assertions, after))
	}
	newLines := strings.Split(after, "\n")
	lines := added(before, newLines)
	if len(lines) == 0 {
		return r
	}
	skips, suppressions := compile(c.Skips), compile(c.Suppressions)
	for _, i := range lines {
		line := newLines[i]
		var kind string
		switch {
		case test && matches(skips, line):
			kind = "skip"
		case matches(suppressions, line):
			kind = "suppression"
		default:
			continue
		}
		hit := Hit{Line: i + 1, Text: strings.TrimSpace(line), Kind: kind}
		for _, at := range []string{line, above(newLines, i)} {
			if m := allow.FindStringSubmatch(at); m != nil {
				hit.Reason = m[1]
				break
			}
		}
		r.Hits = append(r.Hits, hit)
	}
	return r
}

// added is the index of each line in after that before doesn't have, line
// for line: a line moved or kept counts once against its old copy.
func added(before string, after []string) []int {
	old := map[string]int{}
	for _, l := range strings.Split(before, "\n") {
		old[strings.TrimSpace(l)]++
	}
	var out []int
	for i, l := range after {
		if key := strings.TrimSpace(l); old[key] > 0 {
			old[key]--
		} else {
			out = append(out, i)
		}
	}
	return out
}

func above(lines []string, i int) string {
	if i == 0 {
		return ""
	}
	return lines[i-1]
}

func compile(patterns []string) []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil { // config.validate names a bad one
			out = append(out, re)
		}
	}
	return out
}

func matches(res []*regexp.Regexp, line string) bool {
	return slices.ContainsFunc(res, func(re *regexp.Regexp) bool { return re.MatchString(line) })
}

func count(res []*regexp.Regexp, text string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		if matches(res, line) {
			n++
		}
	}
	return n
}
