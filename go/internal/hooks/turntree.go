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
	if h.DryRun {
		return nil
	}
	_ = os.Remove(claimsUnchecked(h)) // the last turn's, shown by its Stop
	root := git.Toplevel(h.Payload.Cwd())
	file := turnTreeFile(h, root)
	if file == "" {
		return nil
	}
	// bash-edits diffs from the turn's start until a command runs, so an
	// edit made between turns, by the user, is never charged to Claude.
	_ = os.Remove(bashTreeFile(h, root))
	tree, err := git.Snapshot(root)
	if err != nil {
		// A stale snapshot would charge this turn with an earlier one's changes.
		_ = os.Remove(file)
		return nil
	}
	return fsx.WriteAtomic(file, []byte(tree), 0o600)
}

// turnChanges is the absolute paths whose content changed since the turn's
// start, and the tree now; false when no snapshot of the turn's start
// exists. It records the tree now as the next start, for a turn no prompt
// opens.
func turnChanges(h *hook.Hook, root string) (changed []string, tree string, ok bool) {
	changed, _, tree, ok = changedSince(h, root, turnTreeFile(h, root))
	return changed, tree, ok
}

// changedSince is the absolute paths whose content changed since the tree
// file records, that tree, and the tree now, which replaces it in file;
// false when file is "" or holds no tree.
func changedSince(h *hook.Hook, root, file string) (changed []string, before, now string, ok bool) {
	if file == "" {
		return nil, "", "", false
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, "", "", false
	}
	before = string(data)
	now, err = git.Snapshot(root)
	if err != nil {
		return nil, before, "", false
	}
	saveTree(h, file, now)
	changed, err = git.TreeDiff(root, before, now)
	if err != nil {
		return nil, before, now, false
	}
	for i, path := range changed {
		changed[i] = fsx.Abs(root, path)
	}
	return changed, before, now, true
}

// turnTreeFile is where the session keeps root's turn-start snapshot; ""
// outside a work tree or without a session.
func turnTreeFile(h *hook.Hook, root string) string {
	if root == "" {
		return ""
	}
	return h.Paths.SessionFile(cache.TurnTree, h.Payload.SessionID(), cache.RootKey(root))
}
