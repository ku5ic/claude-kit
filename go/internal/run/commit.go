package run

import (
	"fmt"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
)

// Commit runs p's gates, a commit message's checks, one after another
// within budget, and returns each failure's line and the tail of its
// output; "" when none failed. One that can't run, or that the budget cuts
// short, doesn't fail: the commit's own hooks still run.
func Commit(p enforce.Plan, budget time.Duration) string {
	deadline := time.Now().Add(budget)
	var failures strings.Builder
	for _, g := range p.Gates {
		left := time.Until(deadline)
		if g.Skip != "" || left <= 0 {
			continue
		}
		cmd := command(g, g.Command(), left)
		cmd.KillGroup()
		if o := execute(cmd); o.err != nil && !o.timedOut && !o.notFound() {
			fmt.Fprintf(&failures, "FAIL %s\n%s", g.Label, tail(o.out))
		}
	}
	return failures.String()
}
