package e2e

import (
	"strings"
	"testing"
)

// kit statusline reads the statusLine JSON payload from stdin and prints a
// two-row status. The status line is Go: an empty PATH still renders. Runs
// the built binary rather than bin/kit, which would pick the installed one.
func TestStatusline(t *testing.T) {
	t.Parallel()
	k := New(t)
	k.Setenv("PATH", t.TempDir())
	r := k.Run(`{"model":{"display_name":"Opus"}}`, "statusline")
	r.Want(t, 0)
	if rows := Lines(r.Stdout); len(rows) != 2 || !strings.Contains(rows[0], "Opus") || !strings.Contains(rows[1], "0%") {
		t.Errorf("want the model row then the usage row, got:\n%s", r.Output)
	}
}
