package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/cache"
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
	t.Run("a command that changed code is recorded for the claims gate", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		record := k.paths.SessionFile(cache.Claims, "s1", bashEditsRecord)
		run := func(id string) {
			k.run("bash-edits", map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": dir, "tool_use_id": id,
				"tool_name": "Bash", "tool_input": map[string]any{"command": "edit"}}).Want(t, 0)
		}
		run("t0")
		testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes, longer.\n")
		run("t1")
		testutil.Write(t, filepath.Join(dir, ".claude", "scratch", "x.log"), "x\n")
		run("t2")
		var got bashEdit
		if line := testutil.Read(t, record); json.Unmarshal([]byte(line), &got) != nil || strings.Count(line, "\n") != 1 ||
			got.ID != "t1" || len(got.Files) != 1 || filepath.Base(got.Files[0]) != "notes.md" {
			t.Errorf("record = %q, want only the call that changed notes.md, with it", line)
		}
	})
	t.Run("a command the guard marked read-only is skipped, once", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes.\n"+tell)
		marker := cache.ReadOnlyBash(k.paths.CacheDir(), "s1", "cat notes.md")
		testutil.Write(t, marker, "")
		ran(k, dir, "cat notes.md").Want(t, 0)
		if _, err := os.Stat(marker); err == nil {
			t.Error("the marker was left for the next call")
		}
		ran(k, dir, "cat notes.md").Want(t, 2)
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
	t.Run("a command after an Edit is charged only with what it changed", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		path := filepath.Join(dir, "notes.md")
		testutil.Write(t, path, "Notes.\n"+tell)
		k.run("edit-seen", map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": dir,
			"tool_name": "Edit", "tool_input": map[string]any{"file_path": path}}).Want(t, 0)
		testutil.Write(t, path, "Notes.\n"+tell+"A plain line.\n")
		ran(k, dir, "sed -i '' '$a\\' notes.md").Want(t, 0)
	})
	t.Run("another repo's edits between turns aren't charged to the next", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		other := repo(t, filepath.Join(t.TempDir(), "other"))
		testutil.Write(t, filepath.Join(other, "a.md"), "A.\n")
		testutil.Git(t, other, "add", "-A")
		testutil.Git(t, other, "commit", "-q", "-m", "a")
		ran(k, other, "ls").Want(t, 0)
		testutil.Write(t, filepath.Join(other, "a.md"), "A.\n"+tell)
		k.run("turn-tree", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": dir}).Want(t, 0)
		ran(k, other, "ls").Want(t, 0)
		// From there on, that repo's changes are checked.
		testutil.Write(t, filepath.Join(other, "b.md"), tell)
		ran(k, other, "ls").Want(t, 2)
	})
	t.Run("a repo the turn didn't start in gets its baseline before its first command, which is checked", func(t *testing.T) {
		t.Parallel()
		k, _ := setup(t)
		other := repo(t, filepath.Join(t.TempDir(), "other"))
		k.run("bash-baseline", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "cwd": other,
			"tool_name": "Bash", "tool_input": map[string]any{"command": "tee b.md"}}).Want(t, 0)
		testutil.Write(t, filepath.Join(other, "b.md"), tell)
		ran(k, other, "tee b.md").Want(t, 2)
	})
	t.Run("a git command that brings in others' content isn't checked", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes.\n"+tell)
		ran(k, dir, "git pull --rebase").Want(t, 0)
	})
	// Only a command that is one git move and nothing else skips the checks.
	for _, cmd := range []string{"git restore other.md; sed -i '' 's/x/y/' notes.md", "echo git checkout main", "git checkout main && sed -i '' 's/x/y/' notes.md"} {
		t.Run("a git move beside another command is checked: "+cmd, func(t *testing.T) {
			t.Parallel()
			k, dir := setup(t)
			testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes.\n"+tell)
			ran(k, dir, cmd).Want(t, 2)
		})
	}
	t.Run("a file a git move brought in, beside another command, isn't checked", func(t *testing.T) {
		t.Parallel()
		k, dir := setup(t)
		testutil.Git(t, dir, "checkout", "-q", "-b", "x")
		testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes.\n"+tell)
		testutil.Git(t, dir, "commit", "-q", "-am", "theirs")
		testutil.Git(t, dir, "checkout", "-q", "-")
		testutil.Git(t, dir, "checkout", "-q", "x")
		ran(k, dir, "git checkout x && go test ./...").Want(t, 0)
	})
	for _, cmd := range []string{"git -C . checkout main", "cd sub && git pull"} {
		t.Run("one git move alone isn't checked: "+cmd, func(t *testing.T) {
			t.Parallel()
			k, dir := setup(t)
			testutil.Write(t, filepath.Join(dir, "notes.md"), "Notes.\n"+tell)
			ran(k, dir, cmd).Want(t, 0)
		})
	}
	// A test file a command deletes, or renames out of the test glob, takes
	// its assertions with it.
	testWithAssertions := func(t *testing.T) (*sandbox, string) {
		k := newSandbox(t)
		dir := repo(t, filepath.Join(t.TempDir(), "repo"))
		testutil.Write(t, filepath.Join(dir, "a_test.go"), "package a\n\nfunc TestA(t *testing.T) {\n\tt.Errorf(\"one\")\n\tt.Fatalf(\"two\")\n}\n")
		testutil.Git(t, dir, "add", "-A")
		testutil.Git(t, dir, "commit", "-q", "-m", "test")
		k.run("turn-tree", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": dir}).Want(t, 0)
		return k, dir
	}
	t.Run("a deleted test file is sent back with its lost assertions", func(t *testing.T) {
		t.Parallel()
		k, dir := testWithAssertions(t)
		if err := os.Remove(filepath.Join(dir, "a_test.go")); err != nil {
			t.Fatal(err)
		}
		r := ran(k, dir, "rm a_test.go")
		r.Want(t, 2)
		r.Has(t, "2 fewer assertion line(s)", "a_test.go")
	})
	t.Run("a test renamed out of the test glob is sent back", func(t *testing.T) {
		t.Parallel()
		k, dir := testWithAssertions(t)
		if err := os.Rename(filepath.Join(dir, "a_test.go"), filepath.Join(dir, "a_test.go.bak")); err != nil {
			t.Fatal(err)
		}
		ran(k, dir, "mv a_test.go a_test.go.bak").Want(t, 2)
	})
	t.Run("a test renamed to another test keeps its assertions", func(t *testing.T) {
		t.Parallel()
		k, dir := testWithAssertions(t)
		if err := os.Rename(filepath.Join(dir, "a_test.go"), filepath.Join(dir, "b_test.go")); err != nil {
			t.Fatal(err)
		}
		ran(k, dir, "mv a_test.go b_test.go").Want(t, 0)
	})
	t.Run("weaken in disabled_rules lets a deleted test through", func(t *testing.T) {
		t.Parallel()
		k, dir := testWithAssertions(t)
		k.overlay("disabled_rules: [weaken]\n")
		if err := os.Remove(filepath.Join(dir, "a_test.go")); err != nil {
			t.Fatal(err)
		}
		ran(k, dir, "rm a_test.go").Want(t, 0)
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

// Only a write that lands in the repo stops a git move counting as alone: a
// redirect to /dev/null or to a log outside the repo brings no file in.
func TestRepoWritesKeepsOnlyWritesInTheRepo(t *testing.T) {
	t.Parallel()
	root := testutil.Physical(t, t.TempDir())
	notes := filepath.Join(root, "notes.md")
	got := repoWrites(root, []string{"/dev/null", filepath.Join(t.TempDir(), "git.log"), notes})
	if want := []string{notes}; !slices.Equal(got, want) {
		t.Errorf("repoWrites = %q, want %q", got, want)
	}
}

// A git move stays one when read-only calls sit beside it, piped or
// chained, and when it chains with another move; a writer beside it, or a
// reader flag that runs a command, makes it more than a move.
func TestGitMoveIgnoresReadersBesideIt(t *testing.T) {
	t.Parallel()
	for cmd, want := range map[string]bool{
		"git pull":                                true,
		"git pull --rebase 2>&1 | tail -20":       true,
		"git checkout main && git pull":           true,
		"cd sub && git stash pop | head -5":       true,
		"git pull && npm install":                 false,
		"git pull | tee pull.log":                 false,
		"git pull 2>&1 | rg --pre ./run.sh merge": false,
		"git status | tail -3":                    false,
		"tail -3 notes.md":                        false,
	} {
		if got := gitMove(cmd); got != want {
			t.Errorf("gitMove(%q) = %v, want %v", cmd, got, want)
		}
	}
}
