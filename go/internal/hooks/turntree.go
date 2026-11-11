package hooks

import (
	"os"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
)

// TurnTree is the UserPromptSubmit hook that records the work tree as the
// turn starts, so stop-checks also checks files the turn changed through
// Bash, which the transcript doesn't name as edits.
func TurnTree(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	root := git.Toplevel(h.Payload.Cwd())
	file := turnTreeFile(h, root)
	if file == "" || h.DryRun {
		return nil
	}
	tree, err := git.Snapshot(root)
	if err != nil {
		// A stale snapshot would charge this turn with an earlier one's changes.
		_ = os.Remove(file)
		return nil
	}
	return fsx.WriteAtomic(file, []byte(tree), 0o600)
}

// turnChanges is the absolute paths whose content changed since the turn's
// start; false when no snapshot of the turn's start exists. It records the
// tree now as the next start, for a turn no prompt opens.
func turnChanges(h *hook.Hook, root string) ([]string, bool) {
	file := turnTreeFile(h, root)
	if file == "" {
		return nil, false
	}
	before, err := os.ReadFile(file)
	if err != nil {
		return nil, false
	}
	after, err := git.Snapshot(root)
	if err != nil {
		return nil, false
	}
	if !h.DryRun {
		_ = fsx.WriteAtomic(file, []byte(after), 0o600)
	}
	changed, err := git.TreeDiff(root, string(before), after)
	if err != nil {
		return nil, false
	}
	for i, path := range changed {
		changed[i] = fsx.Abs(root, path)
	}
	return changed, true
}

// turnTreeFile is where the session keeps root's turn-start snapshot; ""
// outside a work tree or without a session.
func turnTreeFile(h *hook.Hook, root string) string {
	if root == "" {
		return ""
	}
	return h.Paths.SessionFile(cache.TurnTree, h.Payload.SessionID(), cache.RootKey(root))
}
