package tools

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Findings is how a tool's output names what it found, so the Stop hook
// can block on changed lines only. Item matches one finding and holds a
// line group and, unless Header carries the file for output grouped under
// file headers (eslint's stylish), a file group. A missing line means the
// whole file. Hidden matches output saying findings were left out.
type Findings struct {
	Header, Item, Hidden *regexp.Regexp
}

// Finding is one parsed finding. File is as the tool printed it, relative
// to where it ran or absolute.
type Finding struct {
	File string
	Line int
	Text string
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Parse lists out's findings; ok is false when it says some weren't shown.
func (f *Findings) Parse(out string) ([]Finding, bool) {
	out = ansi.ReplaceAllString(out, "")
	if f.Hidden != nil && f.Hidden.MatchString(out) {
		return nil, false
	}
	var found []Finding
	header := ""
	names := f.Item.SubexpNames()
	for raw := range strings.SplitSeq(out, "\n") {
		if f.Header != nil {
			if m := f.Header.FindStringSubmatch(raw); m != nil {
				header = m[f.Header.SubexpIndex("file")]
				continue
			}
		}
		m := f.Item.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		finding := Finding{File: header, Text: strings.TrimSpace(raw)}
		if slices.Contains(names, "file") {
			finding.File = group(names, m, "file")
		} else {
			finding.Text = header + ":" + finding.Text
		}
		if finding.File == "" {
			continue
		}
		finding.Line, _ = strconv.Atoi(group(names, m, "line"))
		found = append(found, finding)
	}
	return found, true
}

// group is the first non-empty submatch named name: a pattern may repeat a
// name across alternatives (knip prints "sym  file:line:col" and bare paths).
func group(names, m []string, name string) string {
	for i, n := range names {
		if n == name && m[i] != "" {
			return m[i]
		}
	}
	return ""
}
