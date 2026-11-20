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

// bashEditsFormatBudget is how long BashEdits spends formatting, all files.
const bashEditsFormatBudget = 90 * time.Second

// gitMoves are the git subcommands that bring in content Claude didn't
// write: checked out, merged, fetched, or restored.
var gitMoves = []string{"checkout", "switch", "pull", "merge", "rebase", "stash", "reset", "restore", "cherry-pick", "revert", "am", "clone"}

// gitMove is true when cmd is one git move and nothing else but cd, so its
// changes aren't checked. Any other command beside it may have written
// them, and a command that didn't parse counts as one.
func gitMove(cmd string) bool {
	calls, ok := classify.Words(cmd)
	if !ok {
		return false
	}
	calls = slices.DeleteFunc(calls, func(words []string) bool { return len(words) == 0 || filepath.Base(words[0]) == "cd" })
	if len(calls) != 1 || filepath.Base(calls[0][0]) != "git" {
		return false
	}
	sub, _, _ := classify.GitSubcommand(calls[0][1:])
	return slices.Contains(gitMoves, sub)
}

// BashBaseline is the PreToolUse step that snapshots, before a Bash
// command runs, a repo with neither a turn start nor a baseline, so
// bash-edits checks even the first command there. PreToolUse for Bash.
func BashBaseline(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil || p.String(hook.KeyToolName) != "Bash" || h.Config() == nil {
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
	if _, err := os.Stat(base); errors.Is(err, os.ErrNotExist) {
		// Until a command runs this turn, the diff starts at the turn's
		// start. A repo the turn didn't start in got its baseline from
		// BashBaseline before the command; without one, this command's
		// changes there go unchecked and the next one's diff starts here.
		start, err := os.ReadFile(turnTreeFile(h, root))
		if err != nil {
			if tree, err := git.Snapshot(root); err == nil {
				saveTree(h, base, tree)
			}
			return nil
		}
		saveTree(h, base, string(start))
	}
	changed, before, _, ok := changedSince(h, root, base)
	if !ok || len(changed) == 0 {
		return nil
	}
	type change struct {
		path, old string
		after     []byte
		edited    bool
		err       error
	}
	changes := make([]change, len(changed))
	var made []string
	for i, path := range changed {
		c := change{path: path}
		c.after, c.err = os.ReadFile(path)
		c.old, c.edited = lastEdit(h, path)
		// A file as the turn's last Edit left it is that Edit's, counted
		// already.
		if !c.edited || c.err != nil || c.old != string(c.after) {
			made = append(made, path)
		}
		changes[i] = c
	}
	// Whoever wrote the change, a claim checked before it is stale.
	recordBashEdit(h, made)
	if gitMove(p.String(hook.KeyInputCommand)) && len(repoWrites(root, writes)) == 0 {
		return nil
	}
	problems := lostTests(h, root, before, changed)
	rewritten := false
	// Each format may take formatBudget; past this, files are checked but
	// left unformatted, inside hooks.json's 300s.
	formatUntil := time.Now().Add(bashEditsFormatBudget)
	for _, c := range changes {
		path, after, old := c.path, c.after, c.old
		if c.err != nil || git.IsBinary(after) {
			continue
		}
		// An Edit since the baseline was checked and confirmed then: the
		// diff starts from what it left, and the baseline now holds it.
		if c.edited {
			_ = os.Remove(editedFile(h, path))
			if old == string(after) {
				continue
			}
		} else {
			rel, _ := filepath.Rel(root, path)
			old, _ = git.Output(root, "cat-file", "blob", before+":"+filepath.ToSlash(rel))
		}
		problems = append(problems, editChecks(h, path, old, string(after))...)
		if time.Until(formatUntil) > formatBudget {
			afterEdit(h, path)
			if now, err := os.ReadFile(path); err == nil && !bytes.Equal(now, after) {
				rewritten = true
			}
		}
	}
	// The next command's diff starts from what sanitize and format left.
	if rewritten {
		if tree, err := git.Snapshot(root); err == nil {
			saveTree(h, base, tree)
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return h.Block("a file this command changed fails the checks an Edit gets\n"+strings.Join(problems, "\n"), "bash-edits")
}

// repoWrites is the writes that land in root, its symlinks resolved: a
// redirect to /dev/null, or to a log outside the repo, brings no file in.
func repoWrites(root string, writes []string) []string {
	return slices.DeleteFunc(slices.Clone(writes), func(w string) bool {
		return !fsx.Within(root, filepath.Join(fsx.PhysicalPath(filepath.Dir(w)), filepath.Base(w)))
	})
}

// lostTests is the send-back for the assertion lines the test files a
// command deleted, or renamed out of the test glob, took with them: what
// they held at the baseline less what the test files it added hold.
func lostTests(h *hook.Hook, root, before string, changed []string) []string {
	cfg := h.Config()
	if cfg == nil || h.RuleDisabled("weaken") {
		return nil
	}
	c := cfg.WeakenedChecks
	lost := 0
	var gone []string
	for _, path := range changed {
		rel, _ := filepath.Rel(root, path)
		old, err := git.Output(root, "cat-file", "blob", before+":"+filepath.ToSlash(rel))
		now, readErr := os.ReadFile(path)
		switch {
		case errors.Is(readErr, os.ErrNotExist) && err == nil:
			if n := weaken.Assertions(c, path, old); n > 0 {
				lost += n
				gone = append(gone, rel)
			}
		case readErr == nil && err != nil:
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
// command left it; "" outside a work tree or without a session.
func bashTreeFile(h *hook.Hook, root string) string {
	if root == "" {
		return ""
	}
	return h.Paths.SessionFile(cache.TurnTree, h.Payload.SessionID(), "bash", cache.RootKey(root))
}

func saveTree(h *hook.Hook, file, tree string) {
	if !h.DryRun {
		_ = fsx.WriteAtomic(file, []byte(tree), 0o600)
	}
}
