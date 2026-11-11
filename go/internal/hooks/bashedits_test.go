package hooks

import (
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestBashEdits(t *testing.T) {
	t.Parallel()
	// tell is a tier 2 AI tell, split so this file doesn't carry it.
	const tell = "Caching plays a " + "crucial role in the gate.\n"
	setup := func(t *testing.T) (*sandbox, string) {
		k := newSandbox(t)
		dir := repo(t, filepath.Join(t.TempDir(), "repo"))
		testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes.\n")
		testutil.Git(t, dir, "add", "-A")
		testutil.Git(t, dir, "commit", "-q", "-m", "notes")
		k.run("turn-tree", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": dir}).Want(t, 0)
		return k, dir
	}
	ran := func(k *sandbox, dir, cmd string) result {
		return k.run("bash-edits", map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": dir,
			"tool_name": "Bash", "tool_input": map[string]any{"command": cmd}})
	}

	t.Run("a tell a command writes is sent back, as an Edit's would be", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes.\n"+tell)
		r := ran(k, dir, "sed -i '' 's/x/y/' notes.md")
		r.Want(t, 2)
		r.Has(t, "a file this command changed fails the checks an Edit gets", "crucial role", "notes.md")
	})
	t.Run("a clean change passes, and the next command starts from it", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes, longer.\n")
		ran(k, dir, "echo").Want(t, 0)
		testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes, longer.\n"+tell)
		ran(k, dir, "echo").Want(t, 2)
		ran(k, dir, "ls").Want(t, 0)
	})
	t.Run("only lines the command added count", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		testutil.Write(t, filepath.Join(dir, "old.md"), tell)
		testutil.Git(t, dir, "add", "-A")
		testutil.Git(t, dir, "commit", "-q", "-m", "old")
		k.run("turn-tree", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": dir}).Want(t, 0)
		testutil.Write(t, filepath.Join(dir, "old.md"), tell+"A plain line.\n")
		ran(k, dir, "echo").Want(t, 0)
	})
	t.Run("a file an Edit left as it is was checked then, and is skipped", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		path := filepath.Join(dir, "notes.md")
		testutil.Write(t, path, "Notes.\n"+tell)
		k.run("edit-seen", map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": dir,
			"tool_name": "Edit", "tool_input": map[string]any{"file_path": path}}).Want(t, 0)
		ran(k, dir, "go test ./...").Want(t, 0)
	})
	t.Run("a git command that brings in others' content isn't checked", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes.\n"+tell)
		ran(k, dir, "git pull --rebase").Want(t, 0)
	})
	t.Run("an edit made before the turn's prompt isn't charged to the turn", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		ran(k, dir, "ls").Want(t, 0)
		testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes.\n"+tell)
		k.run("turn-tree", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": dir}).Want(t, 0)
		ran(k, dir, "ls").Want(t, 0)
	})
	t.Run("slop in disabled_rules lets the tell through", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		k.overlay("disabled_rules: [slop]\n")
		testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes.\n"+tell)
		ran(k, dir, "echo").Want(t, 0)
	})
}
