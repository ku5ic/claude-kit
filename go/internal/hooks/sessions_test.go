package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// startAs and stopAs are start and stop for session in the checkout at dir,
// with that session's own transcript.
func (e *stopEnv) startAs(session, dir string) {
	e.t.Helper()
	e.k.run("turn-tree", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": session, "cwd": dir}).Want(e.t, 0)
}

func (e *stopEnv) stopAs(session, dir, transcript string) result {
	e.t.Helper()
	return e.k.run("stop-checks", map[string]any{
		"hook_event_name": "Stop", "session_id": session, "cwd": dir, "transcript_path": transcript,
	})
}

// turnIn writes a transcript at path: a prompt, then an Edit of each file.
func turnIn(t *testing.T, path string, files ...string) {
	t.Helper()
	testutil.AppendJSONL(t, path, map[string]any{"type": "user", "message": map[string]any{"content": "do it"}})
	for _, f := range files {
		testutil.AppendJSONL(t, path, map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
			map[string]any{"type": "tool_use", "name": "Edit", "input": map[string]any{"file_path": f}},
		}}})
	}
}

func TestTwoSessionsInOneCheckout(t *testing.T) {
	t.Parallel()
	t.Run("a session's Stop checks its own edit, not one another session made in its turn", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		testutil.Git(t, e.repo, "add", "-A")
		testutil.Git(t, e.repo, "commit", "-q", "-m", "files")
		e.startAs("s1", e.repo)
		e.startAs("s2", e.repo)
		testutil.Write(t, e.path("b.ts"), "s2's edit\n")
		testutil.Write(t, e.path("a.ts"), "s1's edit\n")
		turnIn(t, e.transcript, e.path("a.ts"))
		e.stopAs("s1", e.repo, e.transcript).Want(t, 0)
		if got := e.checked(); got != e.repo+"|--check a.ts" {
			t.Errorf("s1 checked %q, want only its own a.ts", got)
		}
	})
	t.Run("a session whose turn ran Bash still checks what changed", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		testutil.Git(t, e.repo, "add", "-A")
		testutil.Git(t, e.repo, "commit", "-q", "-m", "files")
		e.startAs("s1", e.repo)
		e.turn("Bash", "")
		testutil.Write(t, e.path("b.ts"), "a Bash edit, or another session's: Stop can't tell\n")
		e.stopAs("s1", e.repo, e.transcript).Want(t, 0)
		if got := e.checked(); got != e.repo+"|--check b.ts" {
			t.Errorf("checked %q, want b.ts", got)
		}
	})
	t.Run("the full gate's record covers the shared tree for both sessions, until it changes", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		testutil.Write(t, e.path("a.ts"), "s1's edit\n")
		tree, err := git.Snapshot(e.repo)
		if err != nil {
			t.Fatal(err)
		}
		testutil.Write(t, cache.FullGateTree(e.k.paths.CacheDir(), e.repo), tree)
		other := filepath.Join(e.tmp, "s2.jsonl")
		turnIn(t, other, e.path("a.ts"))
		e.stopAs("s2", e.repo, other).Has(t, "the full gate already checked this tree")
		e.noCalls()
	})
}

func TestTwoWorktreesOfOneRepo(t *testing.T) {
	t.Parallel()
	e := stopSetup(t)
	testutil.Git(t, e.repo, "add", "-A")
	testutil.Git(t, e.repo, "commit", "-q", "-m", "files")
	wt := filepath.Join(e.tmp, "wt")
	testutil.Git(t, e.repo, "worktree", "add", "-q", wt)
	wt = testutil.Physical(t, wt)
	if err := os.Symlink(e.path("node_modules"), filepath.Join(wt, "node_modules")); err != nil {
		t.Fatal(err)
	}
	lint := sources.Entry{Source: "lint-staged", File: "package.json", Name: "*.ts", Dir: ".", Body: sources.Body{Text: "fakelint --check"}, Files: []string{"*.ts"}, PassFiles: true, Stage: "pre-commit"}
	e.k.classify(wt, envelope(t, []map[string]any{verdict(lint, map[string]any{"role": "check", "kind": "lint"})}, nil, nil))

	// The full gate passed in the main checkout; the worktree's own edit
	// still gets its Stop.
	testutil.Write(t, e.path("a.ts"), "main's edit\n")
	tree, err := git.Snapshot(e.repo)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, cache.FullGateTree(e.k.paths.CacheDir(), e.repo), tree)
	e.startAs("s2", wt)
	testutil.Write(t, filepath.Join(wt, "b.ts"), "worktree's edit\n")
	other := filepath.Join(e.tmp, "s2.jsonl")
	turnIn(t, other, filepath.Join(wt, "b.ts"))
	e.stopAs("s2", wt, other).Want(t, 0)
	if got := e.checked(); got != wt+"|--check b.ts" {
		t.Errorf("worktree checked %q, want its b.ts", got)
	}

	// Each checkout keeps its own report, and a Stop in one never removes
	// the other's full-gate record.
	turnIn(t, e.transcript, e.path("a.ts"))
	e.stopAs("s1", e.repo, e.transcript).Has(t, "the full gate already checked this tree")
	main, worktree := cache.StopReport(e.k.paths.CacheDir(), e.repo), cache.StopReport(e.k.paths.CacheDir(), wt)
	if main == worktree || !exists(worktree) || exists(main) {
		t.Errorf("main report %s (want none: its Stop was skipped), worktree report %s", main, worktree)
	}
	if !exists(cache.FullGateTree(e.k.paths.CacheDir(), e.repo)) {
		t.Error("the worktree's Stop removed the main checkout's full-gate record")
	}
}
