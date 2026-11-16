// Package attribution finds an AI tool's attribution in text bound for git
// or GitHub, by the patterns in kit.yml attribution:.
package attribution

import (
	"regexp"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/layout"
)

// Detector holds the compiled patterns.
type Detector struct{ patterns []*regexp.Regexp }

// New compiles c's patterns; one that doesn't compile is skipped, since
// config's validation names it.
func New(c config.Attribution) *Detector {
	d := &Detector{}
	for _, p := range c.Expanded() {
		if re, err := regexp.Compile(p); err == nil {
			d.patterns = append(d.patterns, re)
		}
	}
	return d
}

// Find is the first line of text a pattern matches, trimmed, and whether
// one did. Line by line, so a trailer pattern can anchor at ^.
func (d *Detector) Find(text string) (string, bool) {
	for line := range strings.SplitSeq(text, "\n") {
		for _, re := range d.patterns {
			if re.MatchString(line) {
				return strings.TrimSpace(line), true
			}
		}
	}
	return "", false
}

// Message is the block reason for line, found in where.
func Message(where, line string) string {
	return "AI attribution in " + where + "\n" + layout.KV("line", line) + "\n" +
		layout.KV("do", "drop it: git and code-host text carries no AI attribution")
}
