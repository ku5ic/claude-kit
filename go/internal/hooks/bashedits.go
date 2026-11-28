package hooks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/weaken"
)

// bashEditTool is the tool_name of the payload BashEdits builds for each
// file a command changed, which written reads before and after from.
const bashEditTool = "kit-bash-edit"

// bashEditsFormatBudget is how long BashEdits spends formatting, all files,
// inside bash-edits' timeout in hooks.json (TestBudgetsFitTheirHooks).
const bashEditsFormatBudget = 90 * time.Second

// gitMoves are the git subcommands that bring in content Claude didn't
// write: checked out, merged, fetched, or restored.
var gitMoves = []string{"checkout", "switch", "pull", "merge", "rebase", "stash", "reset", "restore", "cherry-pick", "revert", "am", "clone"}

// gitMove is true when cmd is git moves and nothing else but cd and pure
// readers (a `| tail` on the output), so its changes aren't checked. Any
// other command beside them may have written them, and a command that
// didn't parse counts as one.
func gitMove(cmd string) bool {
	calls, ok := classify.Words(cmd)
	if !ok {
		return false
	}
	calls = slices.DeleteFunc(calls, func(words []string) bool {
		return len(words) == 0 || filepath.Base(words[0]) == "cd" || classify.ReadsOnly(words)
	})
	return len(calls) > 0 && !slices.ContainsFunc(calls, func(words []string) bool { return !isGitMove(words) })
}

// isGitMove is true when words, one call, is a git move.
func isGitMove(words []string) bool {
	if len(words) == 0 || filepath.Base(words[0]) != "git" {
		return false
	}
	sub, _, _ := classify.GitSubcommand(words[1:])
	return slices.Contains(gitMoves, sub)
}

// hasGitMove is true when any call in cmd is a git move.
func hasGitMove(cmd string) bool {
	calls, ok := classify.Words(cmd)
	return ok && slices.ContainsFunc(calls, isGitMove)
}

// fromHead is which of paths, changed files in root, match HEAD: as it holds
// them, or deleted where it has none. Beside a git move they're the move's.
func fromHead(root string, paths []string) []string {
	var moved []string
	heads := blobsAt(root, "HEAD", paths)
	for _, path := range paths {
		blob, atHead := heads[path]
		data, err := os.ReadFile(path)
		if err == nil && atHead && blob == string(data) || errors.Is(err, os.ErrNotExist) && !atHead {
			moved = append(moved, path)
		}
	}
	return moved
}

// movedFile is the turn's record of the files a git move brought into root,
// which the Stop set leaves out.
func movedFile(h *hook.Hook, root string) string { return turnRootFile(h, root, "moved") }

// movedFiles is the turn's movedFile record for root.
func movedFiles(h *hook.Hook, root string) []string {
	data, _ := os.ReadFile(movedFile(h, root))
	return strings.Fields(string(data))
}

// BashBaseline is the PreToolUse step that snapshots, before a Bash
// command runs, a repo with neither a turn start nor a baseline, so
// bash-edits checks even the first command there. A command the bash
// guard marked read-only waits for the next one. PreToolUse for Bash.
func BashBaseline(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil || p.String(hook.KeyToolName) != "Bash" || h.Config() == nil {
		return nil
	}
	if fsx.IsFile(cache.ReadOnlyBash(h.Paths.CacheDir(), p.SessionID(), p.String(hook.KeyInputCommand))) {
		return nil
	}
	root := git.Toplevel(p.Cwd())
	base := bashTreeFile(h, root)
	if root == "" || base == "" || fsx.IsFile(base) || fsx.IsFile(turnTreeFile(h, root)) {
		return nil
	}
	if tree, err := git.Snapshot(root); err == nil {
		saveTree(h, base, tree)
	}
	return nil
}

// BashEdits is the PostToolUse hook on Bash: every file the command
// changed gets the checks an Edit gets, slop, weaken, imports, and tokens,
// then sanitize and format. A failure goes back to Claude, which can't be
// stopped from writing it but is told before it moves on. A file an Edit or
// Write left as it is now was checked then and is skipped. The baseline is
// the tree after the last Bash command checked, else the turn's start.
// writes is what the command writes by itself (bashguard.WriteTargets): a
// git move with any of them beside it, or on it, is checked.
func BashEdits(h *hook.Hook, writes []string) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	// The bash guard marked this command as read-only, so there is no diff to take.
	if marker := cache.ReadOnlyBash(h.Paths.CacheDir(), p.SessionID(), p.String(hook.KeyInputCommand)); marker != "" && fsx.IsFile(marker) {
		_ = os.Remove(marker)
		return nil
	}
	root := git.Toplevel(p.Cwd())
	base := bashTreeFile(h, root)
	if base == "" || h.Config() == nil {
		return nil
	}
	if !hasBaseline(h, root, base) {
		return nil
	}
	changed, before, _, ok := changedSince(h, root, base)
	if !ok || len(changed) == 0 {
		return nil
	}
	changes := readChanges(h, changed)
	cmd := p.String(hook.KeyInputCommand)
	if gitMove(cmd) && len(repoWrites(root, writes)) == 0 {
		return nil
	}
	if hasGitMove(cmd) {
		changed, changes = dropMoves(h, root, changed, changes)
	}
	olds := blobsAt(root, before, changed)
	problems := append(lostTests(h, root, olds, changed), checkChanges(h, root, base, changes, olds)...)
	if len(problems) == 0 {
		return nil
	}
	return h.Block("a file this command changed fails the checks an Edit gets\n"+strings.Join(problems, "\n"), "bash-edits")
}

// hasBaseline is whether root has a tree for this command's diff to start
// from. Until a command runs this turn, the diff starts at the turn's
// start. A repo the turn didn't start in got its baseline from
// BashBaseline before the command; without one, this command's changes
// there go unchecked and the next one's diff starts here.
func hasBaseline(h *hook.Hook, root, base string) bool {
	if _, err := os.Stat(base); !errors.Is(err, os.ErrNotExist) {
		return true
	}
	start, err := os.ReadFile(turnTreeFile(h, root))
	if err != nil {
		if tree, err := git.Snapshot(root); err == nil {
			saveTree(h, base, tree)
		}
		return false
	}
	saveTree(h, base, string(start))
	return true
}

// bashChange is one file a Bash command changed: its content now, and
// what the turn's last Edit of it left, if one did.
type bashChange struct {
	path, old string
	after     []byte
	edited    bool
	err       error
}

// readChanges reads each changed file and records the ones the command
// made. A file as the turn's last Edit left it is that Edit's, counted
// already; whoever wrote the change, a claim checked before it is stale.
func readChanges(h *hook.Hook, changed []string) []bashChange {
	changes := make([]bashChange, len(changed))
	var made []string
	for i, path := range changed {
		c := bashChange{path: path}
		c.after, c.err = os.ReadFile(path)
		c.old, c.edited = lastEdit(h, path)
		if !c.edited || c.err != nil || c.old != string(c.after) {
			made = append(made, path)
		}
		changes[i] = c
	}
	recordBashEdit(h, made)
	return changes
}

// dropMoves records the files a git move brought in from HEAD and leaves
// them out of the checks.
func dropMoves(h *hook.Hook, root string, changed []string, changes []bashChange) ([]string, []bashChange) {
	moved := fromHead(root, changed)
	if record := movedFile(h, root); len(moved) > 0 && record != "" && !h.DryRun {
		_ = fsx.Append(record, []byte(strings.Join(moved, "\n")+"\n"))
	}
	changed = slices.DeleteFunc(changed, func(path string) bool { return slices.Contains(moved, path) })
	changes = slices.DeleteFunc(changes, func(c bashChange) bool { return slices.Contains(moved, c.path) })
	return changed, changes
}

// checkChanges gives each change the checks an Edit gets, then sanitizes
// and formats it while bashEditsFormatBudget allows: each format may take
// formatBudget, and past that files are checked but left unformatted. The
// next command's diff starts from what sanitize and format left.
func checkChanges(h *hook.Hook, root, base string, changes []bashChange, olds map[string]string) []string {
	var problems []string
	rewritten := false
	formatUntil := time.Now().Add(bashEditsFormatBudget)
	for _, c := range changes {
		found, checked := checkChange(h, c, olds)
		problems = append(problems, found...)
		if !checked || time.Until(formatUntil) <= formatBudget {
			continue
		}
		afterEdit(h, c.path)
		if now, err := os.ReadFile(c.path); err == nil && !bytes.Equal(now, c.after) {
			rewritten = true
		}
	}
	if rewritten {
		if tree, err := git.Snapshot(root); err == nil {
			saveTree(h, base, tree)
		}
	}
	return problems
}

// checkChange is the edit checks' problems with c, and whether it was
// checked. An Edit since the baseline was checked and confirmed then: the
// diff starts from what it left, and the baseline now holds it.
func checkChange(h *hook.Hook, c bashChange, olds map[string]string) ([]string, bool) {
	if c.err != nil || git.IsBinary(c.after) {
		return nil, false
	}
	old := olds[c.path]
	if c.edited {
		_ = os.Remove(editedFile(h, c.path))
		if c.old == string(c.after) {
			return nil, false
		}
		old = c.old
	}
	return editChecks(h, c.path, old, string(c.after)), true
}

// repoWrites is the writes that land in root, its symlinks resolved: a
// redirect to /dev/null, or to a log outside the repo, brings no file in.
func repoWrites(root string, writes []string) []string {
	return slices.DeleteFunc(slices.Clone(writes), func(w string) bool {
		return !fsx.Within(root, filepath.Join(fsx.PhysicalPath(filepath.Dir(w)), filepath.Base(w)))
	})
}

// blobsAt is each of paths' content in tree, a tree or commit of the repo
// at root, read in one git call; a path tree doesn't hold has no entry.
func blobsAt(root, tree string, paths []string) map[string]string {
	specs := make([]string, len(paths))
	for i, path := range paths {
		rel, _ := filepath.Rel(root, path)
		specs[i] = tree + ":" + filepath.ToSlash(rel)
	}
	blobs := git.Blobs(root, specs)
	byPath := make(map[string]string, len(blobs))
	for i, path := range paths {
		if blob, ok := blobs[specs[i]]; ok {
			byPath[path] = blob
		}
	}
	return byPath
}

// lostTests is the send-back for the assertion lines the test files a
// command deleted, or renamed out of the test glob, took with them: what
// they held at the baseline less what the test files it added hold.
func lostTests(h *hook.Hook, root string, olds map[string]string, changed []string) []string {
	cfg := h.Config()
	if cfg == nil || h.RuleDisabled("weaken") {
		return nil
	}
	c := cfg.WeakenedChecks
	lost := 0
	var gone []string
	for _, path := range changed {
		if !weaken.IsTest(c, path) {
			continue
		}
		old, atBase := olds[path]
		now, readErr := os.ReadFile(path)
		switch {
		case errors.Is(readErr, os.ErrNotExist) && atBase:
			if n := weaken.Assertions(c, path, old); n > 0 {
				lost += n
				rel, _ := filepath.Rel(root, path)
				gone = append(gone, rel)
			}
		case readErr == nil && !atBase:
			lost -= weaken.Assertions(c, path, string(now))
		}
	}
	if lost <= 0 {
		return nil
	}
	return []string{fmt.Sprintf("this command leaves %d fewer assertion line(s): %s deleted or renamed out of the tests. Confirm only if they're redundant or moved", lost, strings.Join(gone, ", "))}
}

// editChecks runs the PreToolUse edit checks on path's change from before
// to after, each on its own hook so one's decision can't hide another's,
// and returns each deny, ask, or block reason.
func editChecks(h *hook.Hook, path, before, after string) []string {
	raw, _ := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse", "session_id": h.Payload.SessionID(), "cwd": h.Payload.Cwd(),
		"tool_name": bashEditTool, "tool_input": map[string]any{"file_path": path, "before": before, "content": after},
	})
	var reasons []string
	for _, c := range contentChecks {
		sub := subHook(h, c.Name, raw)
		if b := hook.RunCheck(sub, c.Name, c.Check); b != nil {
			reasons = append(reasons, b.Reason)
			continue
		}
		if decision, reason := sub.Decision(); decision == "deny" || decision == "ask" {
			reasons = append(reasons, reason)
		}
	}
	return reasons
}

// afterEdit runs what PostToolUse runs after an Edit: sanitize, then format.
func afterEdit(h *hook.Hook, path string) {
	raw, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse", "session_id": h.Payload.SessionID(), "cwd": h.Payload.Cwd(),
		"tool_name": "Write", "tool_input": map[string]any{"file_path": path},
	})
	for _, c := range []hook.NamedCheck{{Name: "sanitize-output", Check: SanitizeOutput}, {Name: "format-dispatch", Check: FormatDispatch}} {
		// Neither blocks: they rewrite the file or leave it.
		_ = hook.RunCheck(subHook(h, c.Name, raw), c.Name, c.Check)
	}
}

// subHook is a hook named name over payload raw, with h's paths and config.
// Its output is discarded, and the caller reads its decision.
func subHook(h *hook.Hook, name string, raw []byte) *hook.Hook {
	sub := &hook.Hook{Name: name, Payload: hook.ParsePayload(raw), Paths: h.Paths, Stdout: io.Discard, Stderr: io.Discard,
		Now: h.Now, Home: h.Home, DryRun: h.DryRun}
	sub.SetConfig(h.Config())
	return sub
}

// MarkEditSeen is the PostToolUse step after an Edit or Write and its
// formatting. It records the file as it now is, so BashEdits skips a file
// no command touched since and checks only what a command changed after.
func MarkEditSeen(h *hook.Hook) error {
	if h.Payload.Err != nil || h.DryRun {
		return h.Payload.Err
	}
	path := h.Payload.FilePath()
	data, err := os.ReadFile(path)
	if path == "" || err != nil {
		return nil
	}
	if file := editedFile(h, path); file != "" {
		_ = fsx.WriteAtomic(file, data, 0o600)
	}
	return nil
}

// lastEdit is what the turn's last Edit or Write left in path, and whether
// one did.
func lastEdit(h *hook.Hook, path string) (string, bool) {
	file := editedFile(h, path)
	if file == "" {
		return "", false
	}
	data, err := os.ReadFile(file)
	return string(data), err == nil
}

func editedFile(h *hook.Hook, path string) string {
	sum := sha256.Sum256([]byte(path))
	return h.Paths.SessionFile(cache.TurnTree, h.Payload.SessionID(), "edited", hex.EncodeToString(sum[:])[:16])
}

// bashTreeFile is where the session keeps root's tree as the last Bash
// command left it.
func bashTreeFile(h *hook.Hook, root string) string { return turnRootFile(h, root, "bash") }

// turnRootFile is the turn's session file named kind for root, which the
// turn-tree hook clears; "" outside a work tree or without a session.
func turnRootFile(h *hook.Hook, root, kind string) string {
	if root == "" {
		return ""
	}
	return h.Paths.SessionFile(cache.TurnTree, h.Payload.SessionID(), kind, cache.RootKey(root))
}

func saveTree(h *hook.Hook, file, tree string) {
	if !h.DryRun {
		_ = fsx.WriteAtomic(file, []byte(tree), 0o600)
	}
}
