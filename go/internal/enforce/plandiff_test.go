package enforce

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	catalog "github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

const fixtures = "../gapfill/testdata"

// TestPlanDiff writes the old engine's and this one's --plan for each
// gap-fill fixture into the directory KIT_PLAN_DIFF names, with the
// recorded verdicts, for the tagged diff in .claude/tasks/plan-diff.md. It
// checks nothing: both plans depend on what this machine has installed.
func TestPlanDiff(t *testing.T) {
	out := os.Getenv("KIT_PLAN_DIFF")
	if out == "" {
		t.Skip("KIT_PLAN_DIFF names the directory the plans go to")
	}
	for name := range testutil.Fixtures(t, fixtures) {
		root := testutil.Fixture(t, fixtures, name)
		cfg := testutil.KitConfig(t)
		var old strings.Builder
		catalog.PrintPlan(cfg, root, nil, &old)
		recording, _ := filepath.Abs(filepath.Join(fixtures, name+".golden.json"))
		cfg.Classifier = testutil.Replayer(t, recording)
		var plan strings.Builder
		Build(cfg, Options{Root: root, CacheDir: t.TempDir(), Ask: true, Timeout: time.Minute}).Print(&plan)
		testutil.Put(t, out, name+".old", strings.ReplaceAll(old.String(), root, "<root>"))
		testutil.Put(t, out, name+".new", strings.ReplaceAll(plan.String(), root, "<root>"))
	}
}
