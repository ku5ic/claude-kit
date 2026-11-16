package hooks

import (
	"os"
	"path/filepath"

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
	// Every repo's snapshots, bash-edits' baselines, and edit records are
	// the last turn's: an edit made between turns, by the user, is never
	// charged to Claude.
	if session := h.Paths.SessionFile(cache.TurnTree, h.Payload.SessionID()); session != "" {
		stale, _ := filepath.Glob(session + "-*")
		for _, f := range stale {
			_ = os.Remove(f)
		}
	}
	root := git.Toplevel(h.Payload.Cwd())
	file := turnTreeFile(h, root)
	if file == "" {
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
	before, ok = readTree(file)
	if !ok {
		return nil, "", "", false
	}
	changed, now, ok = diffTree(root, before)
	if now != "" {
		saveTree(h, file, now)
	}
	return changed, before, now, ok
}

// changedThisTurn is turnChanges without recording the tree now, for a
// guard that looks before a tool runs and must leave Stop's baseline alone.
func changedThisTurn(h *hook.Hook, root string) ([]string, bool) {
	before, ok := readTree(turnTreeFile(h, root))
	if !ok {
		return nil, false
	}
	changed, _, ok := diffTree(root, before)
	return changed, ok
}

// readTree is the tree file records, false when file is "" or unreadable.
func readTree(file string) (string, bool) {
	if file == "" {
		return "", false
	}
	data, err := os.ReadFile(file)
	return string(data), err == nil
}

// diffTree is the absolute paths whose content differs between the tree
// before and root's tree now, and that tree; now is "" when git can't take
// it.
func diffTree(root, before string) (changed []string, now string, ok bool) {
	now, err := git.Snapshot(root)
	if err != nil {
		return nil, "", false
	}
	changed, err = git.TreeDiff(root, before, now)
	if err != nil {
		return nil, now, false
	}
	for i, path := range changed {
		changed[i] = fsx.Abs(root, path)
	}
	return changed, now, true
}

// turnTreeFile is where the session keeps root's turn-start snapshot; ""
// outside a work tree or without a session.
func turnTreeFile(h *hook.Hook, root string) string {
	if root == "" {
		return ""
	}
	return h.Paths.SessionFile(cache.TurnTree, h.Payload.SessionID(), cache.RootKey(root))
}
