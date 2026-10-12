package enforce

import (
	"strconv"
	"sync"
	"testing"
)

// Two Stop hooks on one root, or Stop plus kit run-checks, mark at once;
// neither may lose the other's mark.
func TestConcurrentAddMarkKeepsEveryMark(t *testing.T) {
	t.Parallel()
	cacheDir, root := t.TempDir(), t.TempDir()
	const n = 32
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			g := Gate{Dir: root, Body: "fmt " + strconv.Itoa(i)}
			if err := AddMark(cacheDir, root, g, Mark{Label: g.Body}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if got := len(Marks(cacheDir, root)); got != n {
		t.Errorf("%d marks, want %d", got, n)
	}
}
