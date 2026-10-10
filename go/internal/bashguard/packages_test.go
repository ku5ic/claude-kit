package bashguard

import (
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/gapfill"
)

func TestNearestPutsTheDeepestDirectoryFirst(t *testing.T) {
	t.Parallel()
	facts := []gapfill.Manager{{Dir: ".", Manager: "pnpm"}, {Dir: "y", Manager: "yarn"}, {Dir: "z", Manager: "bun"}}
	got := nearest("/r", "/r/y/src", facts)
	if len(got) != 2 || got[0].Manager != "yarn" || got[1].Manager != "pnpm" {
		t.Errorf("nearest: %+v", got)
	}
}
