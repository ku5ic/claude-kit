package hooks

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// sheetSetup is a stopEnv with its files committed, .claude ignored, and
// plan approved in session s1.
func sheetSetup(t *testing.T, plan string) *stopEnv {
	t.Helper()
	e := stopSetup(t)
	testutil.Write(t, e.path(".gitignore"), "node_modules\n.claude\n")
	testutil.Git(t, e.repo, "add", "-A")
	testutil.Git(t, e.repo, "commit", "-q", "-m", "files")
	e.plan(plan)
	e.k.run("plan-mode-context", map[string]string{
		"hook_event_name": "PostToolUse", "tool_name": "ExitPlanMode", "session_id": "s1", "cwd": e.repo,
	}).Want(t, 0)
	return e
}

// tick edits the plan to body, as Claude ticking a step would.
func (e *stopEnv) tick(body string) {
	e.t.Helper()
	plan := e.path(".claude/plans/plan-x.md")
	testutil.Write(e.t, plan, body)
	e.k.run("plan-tick", map[string]any{
		"hook_event_name": "PostToolUse", "tool_name": "Edit", "session_id": "s1", "cwd": e.repo,
		"tool_input": map[string]any{"file_path": plan},
	}).Want(e.t, 0)
}

// notice is a Stop run's systemMessage.
func notice(t *testing.T, r result) string {
	t.Helper()
	var out map[string]string
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatalf("stdout isn't JSON: %v\n%s", err, r.Output)
	}
	return out["systemMessage"]
}

// hasLine fails t unless msg has a line matching each pattern.
func hasLine(t *testing.T, msg string, patterns ...string) {
	t.Helper()
	for _, p := range patterns {
		if !regexp.MustCompile(`(?m)^` + p + `$`).MatchString(msg) {
			t.Errorf("no line matching %q in:\n%s", p, msg)
		}
	}
}

const threeSteps = "## Steps\n\n- [x] 1. one\n- [ ] 2. change a.ts and package-lock.json\n- [ ] 3. three\n"

func TestReviewSheet(t *testing.T) {
	t.Parallel()
	t.Run("a step's sheet puts a lockfile and a file the plan never names first, and lists the rest", func(t *testing.T) {
		t.Parallel()
		e := sheetSetup(t, threeSteps)
		e.start()
		testutil.Write(t, e.path("a.ts"), "x\ny\n")
		testutil.Write(t, e.path("package-lock.json"), "{}\n")
		testutil.Write(t, e.path("c.ts"), "new\n")
		e.turn("Edit", e.path("a.ts"), e.path("package-lock.json"), e.path("c.ts"))
		msg := notice(t, e.stop(false))
		hasLine(t, msg,
			`kit review: step 2 of 3, 3 files, \+3 -0`,
			`look  c\.ts +the plan doesn't name this file`,
			`look  package-lock\.json +lockfile`,
			`file  a\.ts +\+1 -0`,
		)
	})
	t.Run("an added kit: allow marker is a reason to look", func(t *testing.T) {
		t.Parallel()
		e := sheetSetup(t, threeSteps)
		e.start()
		testutil.Write(t, e.path("a.ts"), "x\n// kit: allow flaky upstream\n")
		e.turn("Edit", e.path("a.ts"))
		hasLine(t, notice(t, e.stop(false)), `look  a\.ts +"kit: allow" marker added`)
	})
	t.Run("past ten lines, the rest go to a scratch file the last line names", func(t *testing.T) {
		t.Parallel()
		e := sheetSetup(t, threeSteps)
		e.start()
		var files []string
		for i := range 12 {
			name := fmt.Sprintf("f%02d.md", i)
			testutil.Write(t, e.path(name), "x\n")
			files = append(files, e.path(name))
		}
		e.turn("Edit", files...)
		msg := notice(t, e.stop(false))
		hasLine(t, msg, `kit review: step 2 of 3, 12 files, \+12 -0`, `more  3 files in .*kit-review-step-2-of-3\.txt`)
		if n := strings.Count(msg, "\nlook  "); n != 9 {
			t.Errorf("%d look lines, want 9 and the more line:\n%s", n, msg)
		}
	})
	t.Run("a turn that changes nothing tracked gets no sheet", func(t *testing.T) {
		t.Parallel()
		e := sheetSetup(t, threeSteps)
		e.start()
		e.turn("Edit", e.path(".claude/plans/plan-x.md"))
		e.stop(false).Lacks(t, "kit review")
	})
	t.Run("a batched pause prints one sheet per step ticked", func(t *testing.T) {
		t.Parallel()
		e := stopSetup(t)
		e.k.overlay("step_pause: plan\n")
		testutil.Write(t, e.path(".gitignore"), "node_modules\n.claude\n")
		testutil.Git(t, e.repo, "add", "-A")
		testutil.Git(t, e.repo, "commit", "-q", "-m", "files")
		e.plan("## Steps\n\n- [ ] 1. a.ts\n- [ ] 2. b.ts\n- [ ] 3. three\n")
		e.k.run("plan-mode-context", map[string]string{
			"hook_event_name": "PostToolUse", "tool_name": "ExitPlanMode", "session_id": "s1", "cwd": e.repo,
		}).Want(t, 0)
		e.start()
		testutil.Write(t, e.path("a.ts"), "step one\n")
		e.tick("## Steps\n\n- [x] 1. a.ts\n- [ ] 2. b.ts\n- [ ] 3. three\n")
		testutil.Write(t, e.path("b.ts"), "step two\n")
		e.tick("## Steps\n\n- [x] 1. a.ts\n- [x] 2. b.ts\n- [ ] 3. three\n")
		e.turn("Edit", e.path("a.ts"), e.path("b.ts"))
		msg := notice(t, e.stop(false))
		hasLine(t, msg, `kit review: step 1 of 3, 1 files, \+1 -1`, `file  a\.ts +\+1 -1`, `kit review: step 2 of 3, 1 files, \+1 -1`, `file  b\.ts +\+1 -1`)
		if strings.Count(msg, "kit review:") != 2 {
			t.Errorf("want two sheets:\n%s", msg)
		}
		// The next Stop starts from the last tick: nothing new, no sheet.
		e.turn("Edit", e.path("b.ts"))
		e.stop(false).Lacks(t, "kit review")
	})
}

func TestPlanChanges(t *testing.T) {
	t.Parallel()
	const v1 = "# Plan\n\nWhy.\n\n## Steps\n\n- [ ] 1. one\n- [ ] 2. two\n\n## Risks\n\nNone.\n"
	submit := func(k *sandbox, proj, text string) result {
		return k.run("plan-changes", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "tool_name": "ExitPlanMode", "cwd": proj,
			"tool_input": map[string]any{"plan": text, "planFilePath": "/plans/plan-x.md"}})
	}
	setup := func(t *testing.T) (*sandbox, string) {
		k := newSandbox(t)
		proj := repo(t, filepath.Join(k.home, "proj"))
		submit(k, proj, v1).Empty(t)
		return k, proj
	}
	t.Run("an unchanged resubmit prints nothing", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		submit(k, proj, v1).Empty(t)
	})
	t.Run("a section added and a section changed are listed, with the full diff in scratch", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		v2 := strings.Replace(v1, "- [ ] 2. two\n", "- [ ] 2. two, split\n- [ ] 3. three\n", 1) + "\n## Rollback\n\nRevert.\n"
		msg := notice(t, submit(k, proj, v2))
		hasLine(t, msg,
			`kit plan: resubmitted, 2 sections changed`,
			`changed  ## Steps  \+2 -1`,
			`added  ## Rollback  \+2`,
			`diff  .*kit-plan-changes\.diff`,
		)
		if diff := read(t, filepath.Join(proj, ".claude", "scratch", "kit-plan-changes.diff")); !strings.Contains(diff, "+- [ ] 3. three") {
			t.Errorf("diff:\n%s", diff)
		}
	})
	t.Run("a section removed is listed", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		hasLine(t, notice(t, submit(k, proj, strings.Replace(v1, "\n## Risks\n\nNone.\n", "", 1))), `removed  ## Risks  -2`)
	})
}
