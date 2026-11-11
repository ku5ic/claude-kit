package hooks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
)

// bashEditTool is the tool_name of the payload BashEdits builds for each
// file a command changed, which written reads before and after from.
const bashEditTool = "kit-bash-edit"

// bashEditsFormatBudget is how long BashEdits spends formatting, all files.
const bashEditsFormatBudget = 90 * time.Second

// gitMoves is a git command that brings in content Claude didn't write:
// checked out, merged, fetched, or restored. Its changes aren't checked.
var gitMoves = regexp.MustCompile(`\bgit\s+(checkout|switch|pull|merge|rebase|stash|reset|restore|cherry-pick|revert|am|clone)\b`)

// BashEdits is the PostToolUse hook on Bash: every file the command
// changed gets the checks an Edit gets, slop, weaken, imports, and tokens,
// then sanitize and format. A failure goes back to Claude, which can't be
// stopped from writing it but is told before it moves on. A file an Edit or
// Write left as it is now was checked then and is skipped. The baseline is
// the tree after the last Bash command checked, else the turn's start.
func BashEdits(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	root := git.Toplevel(p.Cwd())
	base := bashTreeFile(h, root)
	if base == "" || h.Config() == nil {
		return nil
	}
	if _, err := os.Stat(base); errors.Is(err, os.ErrNotExist) {
		// Until a command runs this turn, the diff starts at the turn's start.
		if start, err := os.ReadFile(turnTreeFile(h, root)); err == nil {
			saveTree(h, base, string(start))
		}
	}
	changed, before, _, ok := changedSince(h, root, base)
	if !ok || len(changed) == 0 || gitMoves.MatchString(p.String("tool_input.command")) {
		return nil
	}
	var problems []string
	rewritten := false
	// Each format may take formatBudget; past this, files are checked but
	// left unformatted, inside hooks.json's 120s.
	formatUntil := time.Now().Add(bashEditsFormatBudget)
	for _, path := range changed {
		after, err := os.ReadFile(path)
		if err != nil || git.IsBinary(after) || editSeen(h, path, after) {
			continue
		}
		rel, _ := filepath.Rel(root, path)
		old, _ := git.Output(root, "cat-file", "blob", before+":"+filepath.ToSlash(rel))
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
	return h.Block("a file this command changed fails the checks an Edit gets:\n"+strings.Join(problems, "\n"), "bash-edits")
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
// no command touched since.
func MarkEditSeen(h *hook.Hook) error {
	if h.Payload.Err != nil || h.DryRun {
		return h.Payload.Err
	}
	path := h.Payload.FilePath()
	data, err := os.ReadFile(path)
	if path == "" || err != nil {
		return nil
	}
	if m := seenMarker(h, path, data); m != "" {
		_ = fsx.WriteAtomic(m, nil, 0o600)
	}
	return nil
}

// editSeen is true when an Edit or Write left path holding data.
func editSeen(h *hook.Hook, path string, data []byte) bool {
	// An empty marker (no session) fails the stat too.
	_, err := os.Stat(seenMarker(h, path, data))
	return err == nil
}

func seenMarker(h *hook.Hook, path string, data []byte) string {
	sum := sha256.New()
	sum.Write([]byte(path))
	sum.Write([]byte{0})
	sum.Write(data)
	return h.Paths.SessionFile(cache.TurnTree, h.Payload.SessionID(), "seen", hex.EncodeToString(sum.Sum(nil))[:16])
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
