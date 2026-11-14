package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/git"
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
	testutil.Stub(t, e.path("node_modules/.bin/fakelint"), fmt.Sprintf("echo \"$PWD|$*\" >>%q\necho \"lint noise\"\n[[ ! -e %q ]]\n", e.calls, e.path("fail")))
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
		e.edit(tool, p, false)
	}
}

// edit is one edit tool call on path and its result, failed when denied.
func (e *stopEnv) edit(tool, path string, denied bool) {
	id := "toolu_" + filepath.Base(path)
	e.line(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_use", "id": id, "name": tool, "input": map[string]any{"file_path": path}},
	}}})
	e.line(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": denied}}}})
}

// bash is Bash call id running change in dir, with bash-edits run on it
// as PostToolUse would.
func (e *stopEnv) bash(id, dir string, change func()) {
	e.line(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_use", "id": id, "name": "Bash", "input": map[string]any{"command": "edit"}},
	}}})
	if change != nil {
		change()
	}
	e.k.run("bash-edits", map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": dir, "tool_use_id": id,
		"tool_name": "Bash", "tool_input": map[string]any{"command": "edit"}})
	e.line(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id}}}})
}

// start runs turn-tree as a prompt would, snapshotting the tree.
func (e *stopEnv) start() {
	e.t.Helper()
	e.k.run("turn-tree", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": e.repo}).Want(e.t, 0)
}

// checked is the files fakelint ran on, in order.
func (e *stopEnv) checked() string {
	e.t.Helper()
	if !exists(e.calls) {
		return ""
	}
	return strings.TrimRight(read(e.t, e.calls), "\n")
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
	if err := json.Unmarshal([]byte(r.Stdout), &out); r.Status == 0 && (err != nil || len(out) != 1 || !strings.HasPrefix(out["systemMessage"], "kit stop: ")) {
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

// plan writes a plan file in the repo's plans dir, set by plansDirectory,
// and returns its path.
func (e *stopEnv) plan(body string) string {
	testutil.Write(e.t, e.path(".claude/settings.json"), `{"plansDirectory": ".claude/plans"}`)
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
	t.Run("ticking the last step through Bash blocks too; a Bash call that only names the plan doesn't", func(t *testing.T) {
		t.Parallel()
		// call is a Bash call on the plan, at, and its result, with change
		// run in between, as the command would.
		call := func(e *stopEnv, at time.Time, cmd string, change func()) {
			e.line(map[string]any{"type": "assistant", "timestamp": at.UTC().Format(time.RFC3339Nano), "message": map[string]any{"content": []any{
				map[string]any{"type": "tool_use", "id": "toolu_b1", "name": "Bash", "input": map[string]any{"command": cmd}},
			}}})
			if change != nil {
				change()
			}
			e.line(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_b1"}}}})
		}
		e := stopSetup(t)
		plan := e.plan("## Steps\n\n- [x] 1. a\n- [ ] 2. b\n")
		e.turn("Edit", e.path("a.ts"))
		call(e, time.Now().Add(-time.Second), "sed -i '' 's/^- \\[ \\] 2\\. /- [x] 2. /' .claude/plans/plan-x.md", func() { testutil.Write(t, plan, done) })
		e.stop(false).Has(t, "plan-x.md is done")

		e = stopSetup(t)
		e.plan(done)
		e.turn("Edit", e.path("a.ts"))
		call(e, time.Now().Add(time.Second), "grep -c 'x' .claude/plans/plan-x.md", nil)
		e.stop(false).Want(t, 0)

		// Another file whose name ends in the plan's doesn't name it.
		e = stopSetup(t)
		plan = e.plan("## Steps\n\n- [x] 1. a\n- [ ] 2. b\n")
		e.turn("Edit", e.path("a.ts"))
		call(e, time.Now().Add(-time.Second), "sed -i '' 's/x/y/' notes/myplan-x.md", func() { testutil.Write(t, plan, done) })
		e.stop(false).Want(t, 0)

		e = stopSetup(t)
		plan = e.plan("## Steps\n\n- [x] 1. a\n- [ ] 2. b\n")
		e.turn("Edit", e.path("a.ts"))
		call(e, time.Now().Add(-time.Second), `cd .claude/plans && sed -i '' 's/x/y/' "plan-x.md"`, func() { testutil.Write(t, plan, done) })
		e.stop(false).Has(t, "plan-x.md is done")
	})
	t.Run("a plan in Claude Code's default ~/.claude/plans is gated too", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		plan := filepath.Join(e.k.paths.PlansHome(), "plan-x.md")
		testutil.Write(t, plan, done)
		e.turn("Edit", e.path("a.ts"))
		e.turn("Edit", plan)
		e.stop(false).Has(t, "plan-x.md is done")
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
	t.Run("a review launched before one Stop and finished after it counts at the next", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.review()
		e.stop(false)
		// The next Stop reads on from here, with the launch in its saved walk.
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
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
	t.Run("a Bash edit to code after the review needs a new one", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.review()
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
		e.line(map[string]any{"type": "user", "message": map[string]any{"content": "do it"}})
		e.start()
		e.bash("toolu_b1", e.repo, func() { testutil.Write(t, e.path("a.ts"), "y\n") })
		e.edit("Edit", e.plan(done), false)
		e.stop(false).Want(t, 2)
	})
	t.Run("a Bash call that changed no code, or only another repo's, keeps the review", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		other := repo(t, filepath.Join(e.tmp, "other"))
		testutil.Write(t, filepath.Join(other, "c.ts"), "x\n")
		e.review()
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
		e.line(map[string]any{"type": "user", "message": map[string]any{"content": "do it"}})
		e.start()
		e.bash("toolu_b1", e.repo, nil)
		e.bash("toolu_b2", e.repo, func() { testutil.Write(t, e.path(".claude/scratch/x.log"), "x\n") })
		// The first call in a repo the turn didn't start in sets its baseline.
		e.bash("toolu_b3", other, nil)
		e.bash("toolu_b4", other, func() { testutil.Write(t, filepath.Join(other, "c.ts"), "y\n") })
		if record := read(t, e.k.paths.SessionFile(cache.Claims, "s1", bashEditsRecord)); !strings.Contains(record, "toolu_b4") {
			t.Fatalf("the other repo's edit wasn't recorded, so this case proves nothing: %q", record)
		}
		e.edit("Edit", e.plan(done), false)
		e.stop(false).Want(t, 0)
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

// readState is the saved Stop read of e's session, "" before the first.
func (e *stopEnv) readState() string {
	e.t.Helper()
	files, _ := filepath.Glob(filepath.Join(e.k.paths.CacheDir(), cache.StopRead, "s1-*"))
	if len(files) != 1 {
		return ""
	}
	return read(e.t, files[0])
}

func size(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func TestStopReadsTheTranscriptOnce(t *testing.T) {
	t.Parallel()
	const done = "## Steps\n\n- [x] 1. a\n- [x] 2. b\n"
	notes := func(e *stopEnv) []string { return slices.Repeat([]string{e.path("notes.md")}, 8) }
	t.Run("a transcript shorter than the saved read is read whole", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", notes(e)...)
		e.stop(false).Want(t, 0)
		read := size(t, e.transcript)
		testutil.Write(t, e.transcript, "")
		e.turn("Edit", e.path("a.ts"))
		e.turn("Edit", e.plan(done))
		if size(t, e.transcript) >= read {
			t.Fatal("the new transcript isn't shorter than the read")
		}
		e.stop(false).Want(t, 2)
	})
	t.Run("another transcript is read whole", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", notes(e)...)
		e.stop(false).Want(t, 0)
		read := size(t, e.transcript)
		e.transcript = filepath.Join(e.tmp, "other.jsonl")
		e.turn("Edit", e.path("a.ts"))
		e.turn("Edit", append([]string{e.plan(done)}, notes(e)...)...)
		if size(t, e.transcript) <= read {
			t.Fatal("the other transcript isn't longer than the read")
		}
		e.stop(false).Want(t, 2)
	})
	t.Run("a garbled saved read is read whole", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		files, _ := filepath.Glob(filepath.Join(e.k.paths.CacheDir(), cache.StopRead, "s1-*"))
		if len(files) != 1 {
			t.Fatalf("saved reads: %q", files)
		}
		testutil.Write(t, files[0], "{not json")
		e.turn("Edit", e.plan(done))
		e.stop(false).Want(t, 2)
	})
	t.Run("a saved read of another version is read whole", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		files, _ := filepath.Glob(filepath.Join(e.k.paths.CacheDir(), cache.StopRead, "s1-*"))
		if len(files) != 1 {
			t.Fatalf("saved reads: %q", files)
		}
		// A walk from a build before the version, its review line past the
		// edit: kept, it would let the done plan through.
		var saved map[string]any
		if err := json.Unmarshal([]byte(read(t, files[0])), &saved); err != nil {
			t.Fatal(err)
		}
		delete(saved, "version")
		saved["walk"].(map[string]any)["last_review"] = 1000
		data, _ := json.Marshal(saved)
		testutil.Write(t, files[0], string(data))
		e.turn("Edit", e.plan(done))
		e.stop(false).Want(t, 2)
	})
	t.Run("an unreadable transcript leaves the saved read as it was", func(t *testing.T) {
		t.Parallel()
		if os.Geteuid() == 0 {
			// kit: allow root reads a mode 000 file, and a non-file transcript never reaches the read
			t.Skip("root reads a mode 000 file")
		}
		e := stopSetup(t)
		e.turn("Edit", notes(e)...)
		e.turn("Edit", notes(e)...)
		e.stop(false).Want(t, 0)
		before := e.readState()
		if before == "" {
			t.Fatal("the first Stop saved no read")
		}
		if err := os.Chmod(e.transcript, 0); err != nil {
			t.Fatal(err)
		}
		e.stop(false).Want(t, 0)
		if after := e.readState(); after != before {
			t.Errorf("the saved read moved:\nbefore %s\nafter  %s", before, after)
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
	t.Run("the receipt names a report the verifier failed on, once, and no file a check doesn't claim", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		marker := e.k.paths.SessionFile(cache.Claims, "s1", "unchecked")
		testutil.Write(t, marker, "your report\n")
		e.turn("Edit", e.path("a.ts"), e.path("notes.md"))
		r := e.stop(false)
		r.Has(t, `  pass  lint\n\n  unchecked  your report  verifier failed"`)
		r.Lacks(t, "notes.md")
		if exists(marker) {
			t.Error("the verifier's failure would show again next turn")
		}
	})
	t.Run("an edit no check claims prints nothing", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("notes.md"))
		r := e.stop(false)
		r.Want(t, 0)
		r.Empty(t)
	})
	t.Run("a tree the full gate already checked isn't checked again", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		testutil.Touch(t, e.path("fail"))
		tree, err := git.Snapshot(e.repo)
		if err != nil {
			t.Fatal(err)
		}
		testutil.Write(t, cache.FullGateTree(e.k.paths.CacheDir(), e.repo), tree)
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, `"systemMessage":"kit stop: skipped, the full gate already checked this tree"`)
		e.noCalls()
		testutil.Write(t, e.path("a.ts"), "changed after the gate\n")
		e.stop(false).Want(t, 0)
		if !exists(e.calls) {
			t.Error("a tree changed after the full gate went unchecked")
		}
	})
	t.Run("checks that passed on this tree aren't run again; other files, content, or a failure are", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		runs := func() int { return strings.Count(e.checked(), "\n") + min(len(e.checked()), 1) }
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		r := e.stop(false)
		r.Want(t, 0)
		r.Has(t, `"systemMessage":"kit stop: skipped, these checks already passed on this tree"`)
		if runs() != 1 {
			t.Fatalf("checks ran %d times, want 1:\n%s", runs(), e.checked())
		}
		e.turn("Edit", e.path("b.ts"))
		e.stop(false).Want(t, 0)
		testutil.Write(t, e.path("b.ts"), "changed\n")
		e.stop(false).Want(t, 0)
		if runs() != 3 {
			t.Errorf("checks ran %d times, want 3: another file, then new content", runs())
		}
		testutil.Touch(t, e.path("fail"))
		e.stop(false).Want(t, 0)
		e.stop(false).Want(t, 0)
		if runs() != 5 {
			t.Errorf("checks ran %d times, want 5: a failure is never reused", runs())
		}
	})
	t.Run("a question-only turn after a full gate stays silent", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		testutil.Write(t, e.path("a.ts"), "dirty\n")
		testutil.Write(t, cache.FullGateTree(e.k.paths.CacheDir(), e.repo), "any")
		e.turn("Read", e.path("a.ts"))
		r := e.stop(false)
		r.Want(t, 0)
		r.Empty(t)
	})
	t.Run("a failing check blocks with its detail, and the receipt beside it names it with the detail file", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		testutil.Touch(t, e.path("fail"))
		r := e.stop(false)
		r.Want(t, 0)
		var out map[string]string
		if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil || out["decision"] != "block" ||
			!strings.HasPrefix(out["reason"], "kit stop: BLOCKED 1 of 1 checks failed\n\nFAIL  lint (package.json: *.ts)\n      lint noise\nmore  ") ||
			!strings.HasSuffix(out["reason"], "\ndo    fix them, or reply in one line with the cause\n\nrule  checks-failed") ||
			!strings.HasPrefix(out["systemMessage"], "kit stop: 1 failed, 0 skipped, 0 passed\n\n  FAIL  lint (package.json: *.ts)  -> ") {
			t.Errorf("want a block with the failure, and a receipt naming it with its detail file:\n%s", r.Output)
		}
		if _, receiptFile, _ := strings.Cut(out["systemMessage"], "  -> "); !strings.Contains(out["reason"], "more  "+receiptFile+"\n") {
			t.Errorf("the receipt and the block name different detail files:\n%s", r.Output)
		}
		// The command and the rest of the output go to the file, not the block.
		if strings.Contains(out["reason"], "cmd: ") {
			t.Errorf("the block repeats the gate's command:\n%s", out["reason"])
		}
		file, _, _ := strings.Cut(strings.TrimPrefix(out["reason"][strings.LastIndex(out["reason"], "more  "):], "more  "), "\n")
		if detail := read(t, file); !strings.Contains(detail, "FAIL lint (package.json: *.ts)\n  cmd: fakelint --check a.ts\n") {
			t.Errorf("detail file:\n%s", detail)
		}
	})
	t.Run("an entry the classifier can't answer is recorded for kit explain stop, with why, not run", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		if err := os.RemoveAll(filepath.Join(e.k.claude, "cache", cache.Enforce)); err != nil {
			t.Fatal(err)
		}
		failing := filepath.Join(e.tmp, "classifier")
		testutil.FakeTool(t, failing, filepath.Join(e.tmp, "classifier.calls"), "exit 1")
		e.k.overlay("classifier: [" + strconv.Quote(failing) + "]\n")
		e.turn("Edit", e.path("a.ts"))
		if _, report := e.stopReport(); !strings.Contains(report, "SKIP package.json: *.ts (unclassified (classifier: ") {
			t.Errorf("report:\n%s", report)
		}
		e.noCalls()
	})
	t.Run("a check changed mid-session is classified at Stop and run, once", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		testutil.Write(t, e.path("package.json"), `{"lint-staged":{"*.ts":"fakelint --check --strict"}}`)
		strict := sources.Entry{Source: "lint-staged", File: "package.json", Name: "*.ts", Dir: ".", Body: sources.Body{Text: "fakelint --check --strict"}, Files: []string{"*.ts"}, PassFiles: true, Stage: "pre-commit"}
		answer := filepath.Join(e.tmp, "answer.json")
		testutil.Write(t, answer, envelope(t, []map[string]any{verdict(strict, map[string]any{"role": "check", "kind": "lint"})}, nil, nil))
		stub, asked := filepath.Join(e.tmp, "classifier"), filepath.Join(e.tmp, "classifier.calls")
		testutil.FakeTool(t, stub, asked, fmt.Sprintf("cat >/dev/null\ncat %q", answer))
		e.k.overlay("classifier: [" + strconv.Quote(stub) + "]\n")
		e.turn("Edit", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		if got := e.checked(); got != e.repo+"|--check --strict a.ts" {
			t.Errorf("calls %q, want the changed check on a.ts", got)
		}
		calls := len(testutil.Calls(t, asked))
		e.turn("Edit", e.path("b.ts"))
		e.stop(false).Want(t, 0)
		if calls == 0 || len(testutil.Calls(t, asked)) != calls {
			t.Errorf("classifier ran %d times, then %d: want some, then none more", calls, len(testutil.Calls(t, asked))-calls)
		}
	})
	t.Run("a file changed through Bash is checked", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.start()
		e.turn("Bash", "")
		testutil.Write(t, e.path("a.ts"), "sed -i wrote this\n")
		e.stop(false).Want(t, 0)
		if got := e.checked(); got != e.repo+"|--check a.ts" {
			t.Errorf("calls %q, want a.ts checked", got)
		}
	})
	t.Run("a file already dirty is checked only when the turn changes it again", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		testutil.Write(t, e.path("a.ts"), "dirty before the turn\n")
		testutil.Write(t, e.path("b.ts"), "dirty before the turn\n")
		e.start()
		e.turn("Bash", "")
		testutil.Write(t, e.path("a.ts"), "changed again through Bash\n")
		e.stop(false).Want(t, 0)
		if got := e.checked(); got != e.repo+"|--check a.ts" {
			t.Errorf("calls %q, want only a.ts checked", got)
		}
	})
	t.Run("a file committed mid-turn is checked", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.start()
		e.turn("Edit", e.path("a.ts"))
		testutil.Write(t, e.path("a.ts"), "edited\n")
		testutil.Git(t, e.repo, "add", "-A")
		testutil.Git(t, e.repo, "commit", "-q", "-m", "edit")
		e.stop(false).Want(t, 0)
		if got := e.checked(); got != e.repo+"|--check a.ts" {
			t.Errorf("calls %q, want a.ts checked", got)
		}
	})
	t.Run("a turn that changes nothing runs nothing", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		testutil.Write(t, e.path("a.ts"), "dirty before the turn\n")
		e.start()
		e.turn("Read", e.path("a.ts"))
		e.stop(false).Want(t, 0)
		e.noCalls()
	})
	t.Run("without a turn-start snapshot, edits committed in the turn skip the checks", func(t *testing.T) {
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

// An edit to the file of an entry the cache can't answer starts a
// background classify; any other edit doesn't.
func TestAnEditToAnUnansweredEntrysFileReclassifies(t *testing.T) {
	t.Parallel()
	e := stopSetup(t)
	cfg, _, err := sources.LoadConfig(e.k.paths)
	if err != nil {
		t.Fatal(err)
	}
	unanswered := func(rel string) bool {
		_, ok := unanswered(cfg, e.k.paths.CacheDir(), e.path(rel))
		return ok
	}
	if unanswered("package.json") || unanswered("a.ts") {
		t.Error("a classified repo reclassifies")
	}
	testutil.Write(t, e.path("package.json"), `{"lint-staged":{"*.ts":"fakelint --check --strict"}}`)
	if !unanswered("package.json") {
		t.Error("a changed check's file doesn't reclassify")
	}
	if unanswered("a.ts") {
		t.Error("a source file reclassifies")
	}
}

// A finished /code-review's findings reach only Claude, so the first ask
// about them or end of turn is held once until a review scratch
// report is written after it (skills/kit-rules/reference/output.md
// section 3).
func TestReviewReport(t *testing.T) {
	t.Parallel()
	report := func(e *stopEnv) string { return e.path(".claude/scratch/review-branch-20261114-1714.md") }
	stop := func(e *stopEnv, active bool) result {
		return e.k.run("review-report", map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": e.repo,
			"transcript_path": e.transcript, "stop_hook_active": active})
	}
	ask := func(e *stopEnv, tool string) result {
		return e.k.run("review-report", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "cwd": e.repo,
			"transcript_path": e.transcript, "tool_name": tool, "tool_input": map[string]any{}})
	}
	finished := func(t *testing.T) *stopEnv {
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		e.review()
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
		return e
	}
	t.Run("no report blocks Stop once per review", func(t *testing.T) {
		t.Parallel()
		e := finished(t)
		ask(e, "Bash").Allowed(t)
		r := stop(e, false)
		r.Want(t, 2)
		r.Has(t, "kit scratch-dir review", "rule  review-report")
		// A block shown once ends it, so a turn can't loop on Stop.
		stop(e, true).Want(t, 0)
		stop(e, false).Want(t, 0)
		ask(e, "AskUserQuestion").Allowed(t)
		e.review()
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
		stop(e, false).Want(t, 2)
	})
	t.Run("no report denies AskUserQuestion once per review", func(t *testing.T) {
		t.Parallel()
		e := finished(t)
		ask(e, "AskUserQuestion").Denied(t)
		ask(e, "AskUserQuestion").Allowed(t)
		stop(e, false).Want(t, 0)
	})
	t.Run("a report written after the review lets both through", func(t *testing.T) {
		t.Parallel()
		e := finished(t)
		e.turn("Write", report(e))
		stop(e, false).Want(t, 0)
		ask(e, "AskUserQuestion").Allowed(t)
	})
	t.Run("a report Write another guard denied doesn't count", func(t *testing.T) {
		t.Parallel()
		e := finished(t)
		e.edit("Write", report(e), true)
		stop(e, false).Want(t, 2)
	})
	t.Run("a report written before the review finished doesn't count", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.review()
		e.turn("Write", report(e))
		e.finish("<tool-use-id>toolu_r1</tool-use-id>", "completed")
		stop(e, false).Want(t, 2)
	})
	t.Run("a review still running, or none, blocks nothing", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.turn("Edit", e.path("a.ts"))
		stop(e, false).Want(t, 0)
		e.review()
		stop(e, false).Want(t, 0)
		ask(e, "AskUserQuestion").Allowed(t)
	})
	t.Run("a marker that can't be saved lets Stop through, and says so", func(t *testing.T) {
		t.Parallel()
		e := finished(t)
		// A file sits where the marker's directory goes, so the write fails.
		testutil.Write(t, filepath.Join(e.k.paths.CacheDir(), cache.ReviewReport), "")
		r := stop(e, false)
		r.Want(t, 0)
		r.Has(t, "kit review-report: let through unchecked")
	})
	t.Run("review-report in disabled_rules lets it through", func(t *testing.T) {
		t.Parallel()
		e := finished(t)
		e.k.overlay("disabled_rules: [review-report]\n")
		stop(e, false).Want(t, 0)
	})
}
