package run

import (
	"fmt"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
)

// Fix runs p's gates, the fixers of a file just edited, one after another
// in their order, each killed with its children at deadline; one left no
// time doesn't start. A fixer's exit status is its own: it rewrote the file
// or left it. It returns a line per gate, for kit explain stop.
func Fix(p enforce.Plan, deadline time.Time) string {
	var report strings.Builder
	for _, g := range p.Gates {
		why := g.Skip
		if why == "" {
			why = fix(g, deadline)
		}
		if why != "" {
			fmt.Fprintf(&report, "SKIP %s (%s)\n", g.Label, why)
			continue
		}
		fmt.Fprintf(&report, "RAN %s\n", g.Label)
	}
	return report.String()
}

// fix runs g by deadline, and says why it didn't, "" when it did.
func fix(g enforce.Gate, deadline time.Time) string {
	left := time.Until(deadline)
	if left <= 0 {
		return "out of time"
	}
	cmd := command(g, g.Command(), left)
	cmd.KillGroup()
	switch o := execute(cmd); {
	case o.timedOut:
		return "out of time"
	case o.notFound():
		return "not installed: " + lastLine(o.out)
	}
	return ""
}
