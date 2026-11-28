package hooks

import (
	"maps"
	"path/filepath"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// hookBudgets is each budget a hook's work runs inside, by the hook in
// hooks.json that runs it: formatBudget bounds one edit's formatting
// under post-edit-dispatch and under bash-edits alike.
var hookBudgets = []struct {
	hook   string
	name   string
	budget time.Duration
}{
	{"stop-dispatch", "stopBudget", stopBudget},
	{"review-checks", "reviewBudget", reviewBudget},
	{"post-edit-dispatch", "formatBudget", formatBudget},
	{"bash-edits", "formatBudget", formatBudget},
	{"bash-edits", "bashEditsFormatBudget", bashEditsFormatBudget},
}

// overBudget is each budget that reaches its hook's timeout in timeouts,
// or whose hook timeouts lacks.
func overBudget(timeouts map[string]time.Duration) []string {
	var over []string
	for _, b := range hookBudgets {
		if limit, ok := timeouts[b.hook]; !ok || b.budget >= limit {
			over = append(over, b.name+" under "+b.hook)
		}
	}
	return over
}

// Every budget stays inside the hooks.json timeout of each hook that runs
// it, so a timeout change there can't leave one past its hook's end.
func TestBudgetsFitTheirHooks(t *testing.T) {
	t.Parallel()
	timeouts := testutil.HookTimeouts(t, filepath.Join("..", "..", "..", "hooks", "hooks.json"))
	if over := overBudget(timeouts); len(over) > 0 {
		t.Errorf("budgets at or past their hook's timeout: %v", over)
	}
	lowered := maps.Clone(timeouts)
	lowered["stop-dispatch"] = stopBudget
	if over := overBudget(lowered); len(over) != 1 {
		t.Errorf("a stop-dispatch timeout lowered to stopBudget: got %v, want stopBudget flagged", over)
	}
}
