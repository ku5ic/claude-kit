package bashguard

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// replayRow is one command from the false-positive mining of 2026-11-27:
// the decision it should get, and, until the plan step that fixes it
// lands, the decision it gets now.
type replayRow struct {
	Class string `json:"class"`
	Step  int    `json:"step"`
	Shell string `json:"shell"`
	Cwd   string `json:"cwd"`
	Cmd   string `json:"cmd"`
	Now   string `json:"now"`
	Want  string `json:"want"`
}

// Each mined false positive, and the true positive beside it, gets the
// decision its row names, so a fixed false positive can't come back. A row
// still carrying "now" pins today's decision: the step that fixes it drops
// the field, and a change that flips it early shows here too.
func TestFalsePositivesReplay(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	root := filepath.Dir(kitYML)
	fill := strings.NewReplacer("{root}", root, "{home}", k.home).Replace
	for i, raw := range strings.Split(strings.TrimSpace(testutil.Read(t, "testdata/false-positives.jsonl")), "\n") {
		var row replayRow
		if err := json.Unmarshal([]byte(raw), &row); err != nil {
			t.Fatalf("row %d: %v", i+1, err)
		}
		want, why := row.Want, "fixed"
		if row.Now != "" {
			want, why = row.Now, "before its step"
		}
		cwd := root
		if row.Cwd != "" {
			cwd = fill(row.Cwd)
		}
		k.shell = row.Shell
		if got := k.hook("", cwd, fill(row.Cmd)).verdict(); got != want {
			t.Errorf("row %d (%s, step %d, %s): got %s want %s: %s", i+1, row.Class, row.Step, why, got, want, row.Cmd)
		}
	}
}
