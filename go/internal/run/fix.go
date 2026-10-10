package run

import (
	"fmt"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
)

// Fix runs p's gates, the fixers of a file just edited, one after another
// in their order, each killed with its children once timeout passes. A
// fixer's exit status is its own: it rewrote the file or left it. It
// returns a line per gate, for kit explain stop.
func Fix(p enforce.Plan, timeout time.Duration) string {
	var report strings.Builder
	for _, g := range p.Gates {
		why := g.Skip
		if why == "" {
			cmd := command(g, g.Command(), timeout)
			cmd.KillGroup()
			switch o := execute(cmd); {
			case o.timedOut:
				why = fmt.Sprintf("timed out after %s", timeout)
			case o.notFound():
				why = "not installed: " + lastLine(o.out)
			}
		}
		if why != "" {
			fmt.Fprintf(&report, "SKIP %s (%s)\n", g.Label, why)
			continue
		}
		fmt.Fprintf(&report, "RAN %s\n", g.Label)
	}
	return report.String()
}
