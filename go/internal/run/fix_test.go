package run

import (
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
)

// An edit's fixers run inside one hook's timeout: they share its deadline,
// and a fixer the ones before it left no time for doesn't start.
func TestFixersShareOneDeadline(t *testing.T) {
	t.Parallel()
	r := newRepo(t, "repo")
	p := enforce.Plan{Root: r.root, Gates: []enforce.Gate{r.gate("fix (a)", "sleep 0.2"), r.gate("fix (b)", "sleep 5"), r.gate("fix (c)", "true")}}
	start := time.Now()
	got := Fix(p, start.Add(time.Second))
	if want := "RAN fix (a)\nSKIP fix (b) (out of time)\nSKIP fix (c) (out of time)\n"; got != want {
		t.Errorf("report:\n%s\nwant:\n%s", got, want)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("took %s past a 1s deadline", took)
	}
}
