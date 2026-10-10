package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// stopChecksEnv is one stop-checks test's fixture: a repo whose
// package.json runs fakelint on .ts files through lint-staged, and a
// classifier stub calling that a lint check, its verdict cached by kit
// enforce classify. fakelint, a stub in the repo's node_modules/.bin,
// records its cwd and arguments, and fails when $REPO/fail exists. Each
// test writes a transcript JSONL describing the turn the hook inspects.
// What Stop plans and how it judges a run are the enforce and run
// packages' tests; these are its wiring.
type stopChecksEnv struct {
	t                            *testing.T
	k                            *Kit
	tmp, repo, transcript, calls string
}

func stopChecksSetup(t *testing.T) *stopChecksEnv {
	k := New(t)
	tmp := t.TempDir()
	e := &stopChecksEnv{t: t, k: k, tmp: tmp,
		transcript: filepath.Join(tmp, "transcript.jsonl"), calls: filepath.Join(tmp, "calls")}
	Mkdir(t, filepath.Join(tmp, "repo/node_modules/.bin"))
	e.repo = k.Repo(filepath.Join(tmp, "repo"))
	Stub(t, e.path("node_modules/.bin/fakelint"), fmt.Sprintf("echo \"$PWD|$*\" >>%q\necho \"lint noise\"\n[[ ! -e %q ]]\n", e.calls, e.path("fail")))
	Write(t, e.path(".gitignore"), "node_modules\n")
	Write(t, e.path("package.json"), `{"lint-staged":{"*.ts":"fakelint --check"}}`)
	for _, f := range []string{"a.ts", "b.ts", "notes.md"} {
		Write(t, e.path(f), "x\n")
	}
	lint := sources.Entry{Source: "lint-staged", File: "package.json", Name: "*.ts", Dir: ".", Body: sources.Body{Text: "fakelint --check"}, Files: []string{"*.ts"}, PassFiles: true, Stage: "pre-commit"}
	answer := `{"is_error":false,"subtype":"success","structured_output":{"entries":[{"id":"` + gapfill.Key(lint) + `","role":"check","kind":"lint","mutates":false}],"managers":[],"proposals":[]}}`
	classifier := filepath.Join(tmp, "classifier")
	Stub(t, classifier, "cat >/dev/null\necho '"+answer+"'\n")
	k.Overlay("classifier: [" + strconv.Quote(classifier) + "]\n")
	k.Dir = e.repo
	k.Run("", "enforce", "classify").Want(t, 0)
	k.Dir = k.Home
	return e
}

func (e *stopChecksEnv) path(rel string) string { return filepath.Join(e.repo, rel) }

func (e *stopChecksEnv) line(v any) {
	e.t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		e.t.Fatal(err)
	}
	f, err := os.OpenFile(e.transcript, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		e.t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(append(raw, '\n')); err != nil {
		e.t.Fatal(err)
	}
}

// turn is a real user prompt, then one edit per path.
func (e *stopChecksEnv) turn(tool string, paths ...string) {
	e.line(map[string]any{"type": "user", "message": map[string]any{"content": "do it"}})
	for _, p := range paths {
		e.line(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
			map[string]any{"type": "tool_use", "name": tool, "input": map[string]any{"file_path": p}},
		}}})
		e.line(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result"}}}})
	}
}

func (e *stopChecksEnv) stop(active bool) Result {
	e.t.Helper()
	return e.k.Hook("stop-checks", map[string]any{
		"hook_event_name": "Stop", "session_id": "s1", "cwd": e.repo,
		"transcript_path": e.transcript, "stop_hook_active": active,
	})
}

// explain is kit explain stop, run in the repo.
func (e *stopChecksEnv) explain(args ...string) Result {
	e.t.Helper()
	dir := e.k.Dir
	e.k.Dir = e.repo
	defer func() { e.k.Dir = dir }()
	return e.k.Run("", append([]string{"explain", "stop"}, args...)...)
}

// stopReport stops, wants the hook silent unless it blocks, and returns
// its status with kit explain stop's output, which holds the run's report.
func (e *stopChecksEnv) stopReport() Result {
	e.t.Helper()
	r := e.stop(false)
	if r.Status == 0 && r.Output != "" {
		e.t.Errorf("stop-checks printed on a pass:\n%s", r.Output)
	}
	report := e.explain()
	report.Status = r.Status
	return report
}

func (e *stopChecksEnv) callsIs(want string) {
	e.t.Helper()
	if got := strings.TrimRight(Read(e.t, e.calls), "\n"); got != want {
		e.t.Errorf("calls %q, want %q", got, want)
	}
}

func (e *stopChecksEnv) noCalls() {
	e.t.Helper()
	if Exists(e.calls) {
		e.t.Errorf("a check ran:\n%s", Read(e.t, e.calls))
	}
}

// plan writes a plan file in the repo's plans dir and returns its path.
func (e *stopChecksEnv) plan(body string) string {
	path := e.path(".claude/plans/plan-x.md")
	Write(e.t, path, body)
	return path
}

// review launches /code-review through the Skill tool; finish reports it.
func (e *stopChecksEnv) review() {
	e.line(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_use", "id": "toolu_r1", "name": "Skill", "input": map[string]any{"skill": "code-review"}},
	}}})
}

// finish is the task-notification for a review, as a queued command.
func (e *stopChecksEnv) finish(id, status string) {
	e.line(map[string]any{"type": "attachment", "attachment": map[string]any{"type": "queued_command",
		"prompt": "<task-notification>\n<task-id>a1</task-id>\n" + id + "\n<status>" + status + "</status>\n<result>review</result>\n</task-notification>"}})
}

func TestStopPlanDone(t *testing.T) {
	t.Parallel()
	const done = "## Steps\n\n- [x] 1. a\n- [x] 2. b\n"
	t.Run("ticking the last step with no review since the last edit blocks once", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Edit", e.plan(done))
		r := e.stop(false)
		r.Want(t, 2)
		r.Has(t, "plan-x.md is done, but /code-review hasn't finished since the last code edit")
		e.stop(true).Want(t, 0)
	})
	t.Run("a review after the last edit, even in an earlier turn, lets it through", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.review()
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
		e.turn("Edit", e.plan(done))
		e.stop(false).Want(t, 0)
	})
	t.Run("a typed /code-review counts once its notification says completed", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
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
		e := stopChecksSetup(t)
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
		e := stopChecksSetup(t)
		e.review()
		e.turn("Edit", e.path("a.ts"))
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
		e.turn("Edit", e.plan(done))
		e.stop(false).Want(t, 2)
	})
	t.Run("an edit after the review needs a new one", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.review()
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
		e.turn("Edit", e.path("a.ts"), e.plan(done))
		e.stop(false).Want(t, 2)
	})
	t.Run("a plan with an open step, or one ticked in an earlier turn, doesn't gate", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Edit", e.plan("## Steps\n\n- [x] 1. a\n- [ ] 2. b\n"))
		e.stop(false).Want(t, 0)
		e.plan(done)
		e.turn("Edit", e.path("b.ts"))
		e.stop(false).Want(t, 0)
	})
	t.Run("checkboxes outside the Steps list, or in fenced code, don't hold a done plan open", func(t *testing.T) {
		for _, plan := range []string{
			done + "\n## Notes\n\n- [ ] a template item\n",
			done + "\n```md\n- [ ] an example step\n```\n",
			"- [x] 1. a\n\n```\n- [ ] an example step\n```\n",
		} {
			e := stopChecksSetup(t)
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
		e := stopChecksSetup(t)
		e.stop(false).Want(t, 0)
		e.noCalls()
	})
	t.Run("stop_hook_active lets the stop through even when checks would fail", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		Touch(t, e.path("fail"))
		e.stop(true).Want(t, 0)
		e.noCalls()
	})
	t.Run("an edit runs the project's check on only the edited file, silent on a pass", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Edit", e.path("b.ts"), e.path("notes.md"), e.path("b.ts"))
		r := e.stopReport()
		r.Want(t, 0)
		r.Has(t, "last stop-checks run:\nPASS lint (package.json: *.ts)\n")
		e.callsIs(e.repo + "|--check b.ts")
	})
	t.Run("kit explain stop names what claims a file, the command, and the binary's source", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		r := e.explain(e.path("a.ts"), e.path("notes.md"))
		r.Want(t, 0)
		r.Has(t, "lint (package.json: *.ts)\n  runs in  .\n  file     a.ts\n  command  fakelint --check a.ts\n  source   "+e.path("node_modules/.bin/fakelint")+" (local)\n  blocks   findings on changed lines\n",
			"unclaimed  "+e.path("notes.md"))
	})
	t.Run("with no files, kit explain stop takes the working tree's changes", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.explain().Has(t, "  file     a.ts\n  file     b.ts\n")
	})
	t.Run("a failing check blocks with its output", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		Touch(t, e.path("fail"))
		r := e.stop(false)
		r.Want(t, 2)
		r.Has(t, "FAIL lint (package.json: *.ts)\n  cmd: fakelint --check a.ts\n", "lint noise", "checks: 0 passed, 1 failed, 0 skipped")
	})
	t.Run("an entry with no verdict is recorded for kit explain stop, not run", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		if err := os.RemoveAll(filepath.Join(e.k.Claude, "cache", "enforce")); err != nil {
			t.Fatal(err)
		}
		e.turn("Edit", e.path("a.ts"))
		e.stopReport().Has(t, "SKIP package.json: *.ts (unclassified)")
		e.noCalls()
	})
	t.Run("edits committed in the turn skip the checks", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.k.Git(e.repo, "add", "-A")
		e.k.Git(e.repo, "commit", "-q", "-m", "edit")
		e.stop(false).Want(t, 0)
		e.noCalls()
	})
	t.Run("a turn without edit tools skips the checks", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.turn("Read", e.path("a.ts"))
		e.stop(false)
		e.noCalls()
	})
	t.Run("an edit in an earlier turn does not count", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Read", e.path("a.ts"))
		e.stop(false)
		e.noCalls()
	})
	t.Run("a meta user entry does not start a new turn", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.line(map[string]any{"type": "user", "isMeta": true, "message": map[string]any{"content": "skill loaded"}})
		e.stop(false)
		if !Exists(e.calls) {
			t.Error("the edit's check did not run")
		}
	})
	t.Run("outside a git worktree exits clean", func(t *testing.T) {
		t.Parallel()
		e := stopChecksSetup(t)
		e.repo = filepath.Join(e.tmp, "plain")
		Write(t, e.path("a.ts"), "x\n")
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.noCalls()
	})
}
