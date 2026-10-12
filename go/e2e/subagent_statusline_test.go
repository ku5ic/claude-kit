package e2e

import (
	"strings"
	"testing"
)

// kit subagent-statusline reads the subagentStatusLine payload from stdin
// and prints one {"id","content"} JSON object per task. An empty PATH still
// renders. Runs the built binary rather than bin/kit, which would pick the
// installed one.
func TestSubagentStatusline(t *testing.T) {
	t.Parallel()
	k := New(t)
	k.Setenv("PATH", t.TempDir())
	r := k.Run(`{"columns":120,"tasks":[{"id":"t1","name":"scout"}]}`, "subagent-statusline")
	r.Want(t, 0)
	if got := strings.TrimRight(r.Output, "\n"); got != `{"id":"t1","content":"scout"}` {
		t.Errorf("output %q", got)
	}
}
