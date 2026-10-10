package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// stopEnv is one stop-checks case: a repo whose package.json runs fakelint
// on .ts files through lint-staged, classified as a lint check. fakelint,
// in the repo's node_modules/.bin, records its cwd and arguments and fails
// when $REPO/fail exists. Each case writes the transcript of the turn the
// hook inspects.
type stopEnv struct {
	t                            *testing.T
	k                            *sandbox
	tmp, repo, transcript, calls string
}

func stopSetup(t *testing.T) *stopEnv {
	t.Helper()
	tmp := t.TempDir()
	e := &stopEnv{t: t, k: newSandbox(t), tmp: tmp, transcript: filepath.Join(tmp, "transcript.jsonl"), calls: filepath.Join(tmp, "calls")}
	testutil.Mkdir(t, filepath.Join(tmp, "repo/node_modules/.bin"))
	e.repo = repo(t, filepath.Join(tmp, "repo"))
	stub(t, e.path("node_modules/.bin/fakelint"), fmt.Sprintf("echo \"$PWD|$*\" >>%q\necho \"lint noise\"\n[[ ! -e %q ]]\n", e.calls, e.path("fail")))
	testutil.Write(t, e.path(".gitignore"), "node_modules\n")
	testutil.Write(t, e.path("package.json"), `{"lint-staged":{"*.ts":"fakelint --check"}}`)
	for _, f := range []string{"a.ts", "b.ts", "notes.md"} {
		testutil.Write(t, e.path(f), "x\n")
	}
	lint := sources.Entry{Source: "lint-staged", File: "package.json", Name: "*.ts", Dir: ".", Body: sources.Body{Text: "fakelint --check"}, Files: []string{"*.ts"}, PassFiles: true, Stage: "pre-commit"}
	e.k.classify(e.repo, envelope(t, []map[string]any{verdict(lint, map[string]any{"role": "check", "kind": "lint"})}, nil, nil))
	return e
}

func (e *stopEnv) path(rel string) string { return filepath.Join(e.repo, rel) }

func (e *stopEnv) line(v any) { testutil.AppendJSONL(e.t, e.transcript, v) }

// turn is a real user prompt, then one edit per path.
func (e *stopEnv) turn(tool string, paths ...string) {
	e.line(map[string]any{"type": "user", "message": map[string]any{"content": "do it"}})
	for _, p := range paths {
		e.line(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
			map[string]any{"type": "tool_use", "name": tool, "input": map[string]any{"file_path": p}},
		}}})
		e.line(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result"}}}})
	}
}

func (e *stopEnv) stop(active bool) result {
	e.t.Helper()
	return e.k.run("stop-checks", map[string]any{
		"hook_event_name": "Stop", "session_id": "s1", "cwd": e.repo,
		"transcript_path": e.transcript, "stop_hook_active": active,
	})
}

// stopReport stops, wants a pass to print only its systemMessage, and
// returns its status with the report it kept for kit explain stop.
func (e *stopEnv) stopReport() (int, string) {
	e.t.Helper()
	r := e.stop(false)
	var out map[string]string
	if err := json.Unmarshal([]byte(r.Stdout), &out); r.Status == 0 && (err != nil || len(out) != 1 || !strings.HasPrefix(out["systemMessage"], "stop checks: ")) {
		e.t.Errorf("a pass prints more or other than its systemMessage:\n%s", r.Output)
	}
	return r.Status, read(e.t, cache.StopReport(e.k.paths.CacheDir(), e.repo))
}

func (e *stopEnv) noCalls() {
	e.t.Helper()
	if exists(e.calls) {
		e.t.Errorf("a check ran:\n%s", read(e.t, e.calls))
	}
}

// plan writes a plan file in the repo's plans dir and returns its path.
func (e *stopEnv) plan(body string) string {
	path := e.path(".claude/plans/plan-x.md")
	testutil.Write(e.t, path, body)
	return path
}

// review launches /code-review through the Skill tool.
func (e *stopEnv) review() {
	e.line(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_use", "id": "toolu_r1", "name": "Skill", "input": map[string]any{"skill": "code-review"}},
	}}})
}

// finish is the task-notification for a review, as a queued command.
func (e *stopEnv) finish(id, status string) {
	e.line(map[string]any{"type": "attachment", "attachment": map[string]any{"type": "queued_command",
		"prompt": "<task-notification>\n<task-id>a1</task-id>\n" + id + "\n<status>" + status + "</status>\n<result>review</result>\n</task-notification>"}})
}

func TestStopPlanDone(t *testing.T) {
	t.Parallel()
	const done = "## Steps\n\n- [x] 1. a\n- [x] 2. b\n"
	t.Run("ticking the last step with no review since the last edit blocks once", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Edit", e.plan(done))
		r := e.stop(false)
		r.Want(t, 2)
		r.Has(t, "plan-x.md is done, but /code-review hasn't finished since the last code edit")
		e.stop(true).Want(t, 0)
	})
	t.Run("a review after the last edit, even in an earlier turn, lets it through", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.review()
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
		e.turn("Edit", e.plan(done))
		e.stop(false).Want(t, 0)
	})
	t.Run("a typed /code-review counts once its notification says completed", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.line(map[string]any{"type": "user", "message": map[string]any{"content": "/code-review high"}})
		e.line(map[string]any{"type": "system", "subtype": "local_command",
			"content": `<forked-skill-launch>{"agentId":"a1","skillName":"code-review"}</forked-skill-launch>`})
		e.line(map[string]any{"type": "user", "message": map[string]any{"content": "<task-notification>\n<task-id>a1</task-id>\n<status>completed</status>\n</task-notification>"}})
		e.turn("Edit", e.plan(done))
		e.stop(false).Want(t, 0)
	})
	t.Run("a review that was stopped, or hasn't finished, doesn't count", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.review()
		e.turn("Edit", e.plan(done))
		e.stop(false).Want(t, 2)
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "killed")
		e.turn("Edit", e.plan(done))
		e.stop(false).Want(t, 2)
	})
	t.Run("an edit made while the review ran needs a new one", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.review()
		e.turn("Edit", e.path("a.ts"))
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
		e.turn("Edit", e.plan(done))
		e.stop(false).Want(t, 2)
	})
	t.Run("an edit after the review needs a new one", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.review()
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
		e.turn("Edit", e.path("a.ts"), e.plan(done))
		e.stop(false).Want(t, 2)
	})
	t.Run("a plan with an open step, or one ticked in an earlier turn, doesn't gate", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Edit", e.plan("## Steps\n\n- [x] 1. a\n- [ ] 2. b\n"))
		e.stop(false).Want(t, 0)
		e.plan(done)
		e.turn("Edit", e.path("b.ts"))
		e.stop(false).Want(t, 0)
	})
	t.Run("checkboxes outside the Steps list, or in fenced code, don't hold a done plan open", func(t *testing.T) {
		t.Parallel()
		for _, plan := range []string{
			done + "\n## Notes\n\n- [ ] a template item\n",
			done + "\n```md\n- [ ] an example step\n```\n",
			"- [x] 1. a\n\n```\n- [ ] an example step\n```\n",
		} {
			e := stopSetup(t)
			e.turn("Edit", e.path("a.ts"))
			e.turn("Edit", e.plan(plan))
			e.stop(false).Want(t, 2)
		}
	})
}

func TestStopChecks(t *testing.T) {
	t.Parallel()
	t.Run("missing transcript runs nothing", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.stop(false).Want(t, 0)
		e.noCalls()
	})
	t.Run("stop_hook_active lets the stop through even when checks would fail", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		testutil.Touch(t, e.path("fail"))
		e.stop(true).Want(t, 0)
		e.noCalls()
	})
	t.Run("an edit runs the project's check on only the edited file", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Edit", e.path("b.ts"), e.path("notes.md"), e.path("b.ts"))
		status, report := e.stopReport()
		if status != 0 || !strings.HasPrefix(report, "PASS lint (package.json: *.ts)\n") {
			t.Errorf("status %d, report:\n%s", status, report)
		}
		if got := strings.TrimRight(read(t, e.calls), "\n"); got != e.repo+"|--check b.ts" {
			t.Errorf("calls %q, want %q", got, e.repo+"|--check b.ts")
		}
	})
	t.Run("an entry with no verdict is recorded for kit explain stop, not run", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		if err := os.RemoveAll(filepath.Join(e.k.claude, "cache", cache.Enforce)); err != nil {
			t.Fatal(err)
		}
		e.turn("Edit", e.path("a.ts"))
		if _, report := e.stopReport(); !strings.Contains(report, "SKIP package.json: *.ts (unclassified)") {
			t.Errorf("report:\n%s", report)
		}
		e.noCalls()
	})
	t.Run("edits committed in the turn skip the checks", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		testutil.Git(t, e.repo, "add", "-A")
		testutil.Git(t, e.repo, "commit", "-q", "-m", "edit")
		e.stop(false).Want(t, 0)
		e.noCalls()
	})
	t.Run("a turn without edit tools skips the checks", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Read", e.path("a.ts"))
		e.stop(false)
		e.noCalls()
	})
	t.Run("an edit in an earlier turn does not count", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Read", e.path("a.ts"))
		e.stop(false)
		e.noCalls()
	})
	t.Run("a meta user entry does not start a new turn", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.line(map[string]any{"type": "user", "isMeta": true, "message": map[string]any{"content": "skill loaded"}})
		e.stop(false)
		if !exists(e.calls) {
			t.Error("the edit's check did not run")
		}
	})
	t.Run("outside a git worktree exits clean", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.repo = filepath.Join(e.tmp, "plain")
		testutil.Write(t, e.path("a.ts"), "x\n")
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.noCalls()
	})
}
