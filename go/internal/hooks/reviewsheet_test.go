package hooks

import (
	"encoding/json"
	"fmt"
	"maps"
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
	var out struct {
		SystemMessage string `json:"systemMessage"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatalf("stdout isn't JSON: %v\n%s", err, r.Output)
	}
	return out.SystemMessage
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
	t.Run("a step's sheet groups files by their reason to look, each group counted, and lists the rest last", func(t *testing.T) {
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
			``,
			`  the plan doesn't name this file \(1\)`,
			`    c\.ts`,
			`  lockfile \(1\)`,
			`    package-lock\.json`,
			`  other files \(1\)`,
			`    a\.ts +\+1 -0`,
		)
	})
	t.Run("an added kit: allow marker is a reason to look", func(t *testing.T) {
		t.Parallel()
		e := sheetSetup(t, threeSteps)
		e.start()
		testutil.Write(t, e.path("a.ts"), "x\n// kit: allow flaky upstream\n")
		e.turn("Edit", e.path("a.ts"))
		hasLine(t, notice(t, e.stop(false)), `  "kit: allow" marker added \(1\)`, `    a\.ts`)
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
		hasLine(t, msg, `kit review: step 2 of 3, 12 files, \+12 -0`, `  the plan doesn't name this file \(12\)`, `  more  3 files in .*kit-review-step-2-of-3\.txt`)
		if n := strings.Count(msg, "\n    f"); n != 9 {
			t.Errorf("%d file lines, want 9 and the more line:\n%s", n, msg)
		}
	})
	t.Run("past the limit, a group with no file shown leaves no heading", func(t *testing.T) {
		t.Parallel()
		e := sheetSetup(t, threeSteps)
		e.start()
		var files []string
		for i := range 12 {
			name := fmt.Sprintf("f%02d.md", i)
			testutil.Write(t, e.path(name), "x\n")
			files = append(files, e.path(name))
		}
		testutil.Write(t, e.path("a.ts"), "x\ny\n")
		testutil.Write(t, e.path("package-lock.json"), "{}\n")
		e.turn("Edit", append(files, e.path("a.ts"), e.path("package-lock.json"))...)
		msg := notice(t, e.stop(false))
		lines := strings.Split(msg, "\n")
		for i, l := range lines {
			if strings.HasPrefix(l, "  ") && !strings.HasPrefix(l, "    ") && strings.HasSuffix(l, ")") && (i+1 == len(lines) || !strings.HasPrefix(lines[i+1], "    ")) {
				t.Errorf("heading %q has no file under it:\n%s", l, msg)
			}
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
		hasLine(t, msg, `kit review: step 1 of 3, 1 files, \+1 -1`, `    a\.ts +\+1 -1`, `kit review: step 2 of 3, 1 files, \+1 -1`, `    b\.ts +\+1 -1`)
		if strings.Count(msg, "kit review:") != 2 {
			t.Errorf("want two sheets:\n%s", msg)
		}
		// The next Stop starts from the last tick: nothing new, no sheet.
		e.turn("Edit", e.path("b.ts"))
		e.stop(false).Lacks(t, "kit review")
	})
}

// TestPickedUpPlan covers a plan continued in a session that never approved
// it, as after /clear.
func TestPickedUpPlan(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) *stopEnv {
		e := stopSetup(t)
		testutil.Write(t, e.path(".gitignore"), "node_modules\n.claude\n")
		testutil.Git(t, e.repo, "add", "-A")
		testutil.Git(t, e.repo, "commit", "-q", "-m", "files")
		return e
	}
	edit := func(e *stopEnv, path string, more map[string]any) {
		payload := map[string]any{"hook_event_name": "PostToolUse", "tool_name": "Edit", "session_id": "s1", "cwd": e.repo,
			"tool_input": map[string]any{"file_path": path}}
		maps.Copy(payload, more)
		e.k.run("plan-tick", payload).Want(e.t, 0)
	}
	prompt := func(e *stopEnv) result {
		return e.k.run("plan-mode-context", map[string]string{"hook_event_name": "UserPromptSubmit", "permission_mode": "default", "session_id": "s1", "cwd": e.repo})
	}
	t.Run("the session's edit of a plan with an open step makes it the active plan", func(t *testing.T) {
		t.Parallel()
		e := setup(t)
		plan := e.plan(threeSteps)
		prompt(e).Empty(t)
		edit(e, plan, nil)
		prompt(e).Has(t, "Plan "+plan+" has open steps")
		e.start()
		testutil.Write(t, e.path("a.ts"), "x\ny\n")
		e.turn("Edit", e.path("a.ts"))
		hasLine(t, notice(t, e.stop(false)), `kit review: step 2 of 3, 1 files, \+1 -0`)
	})
	t.Run("a batched pause shows a sheet for each step ticked since", func(t *testing.T) {
		t.Parallel()
		e := setup(t)
		e.k.overlay("step_pause: plan\n")
		e.plan("## Steps\n\n- [ ] 1. a.ts\n- [ ] 2. b.ts\n- [ ] 3. three\n")
		e.start()
		testutil.Write(t, e.path("a.ts"), "step one\n")
		e.tick("## Steps\n\n- [x] 1. a.ts\n- [ ] 2. b.ts\n- [ ] 3. three\n")
		testutil.Write(t, e.path("b.ts"), "step two\n")
		e.tick("## Steps\n\n- [x] 1. a.ts\n- [x] 2. b.ts\n- [ ] 3. three\n")
		e.turn("Edit", e.path("a.ts"), e.path("b.ts"))
		hasLine(t, notice(t, e.stop(false)), `kit review: step 2 of 3, 1 files, \+1 -1`, `    b\.ts +\+1 -1`)
	})
	t.Run("an edit in plan mode or by a subagent doesn't", func(t *testing.T) {
		t.Parallel()
		for _, more := range []map[string]any{{"permission_mode": "plan"}, {"agent_id": "a1"}} {
			e := setup(t)
			edit(e, e.plan(threeSteps), more)
			prompt(e).Empty(t)
		}
	})
	t.Run("a plan with no open step, or a checklist outside the plan dir, doesn't", func(t *testing.T) {
		t.Parallel()
		e := setup(t)
		edit(e, e.plan("## Steps\n\n- [x] one\n"), nil)
		edit(e, e.path(".claude/plans/gone.md"), nil)
		testutil.Write(t, e.path("todo.md"), threeSteps)
		edit(e, e.path("todo.md"), nil)
		testutil.Write(t, e.path(".claude/plans/notes.txt"), threeSteps)
		edit(e, e.path(".claude/plans/notes.txt"), nil)
		prompt(e).Empty(t)
	})
	t.Run("an approved plan stays active when the session edits another", func(t *testing.T) {
		t.Parallel()
		e := sheetSetup(t, threeSteps)
		other := e.path(".claude/plans/plan-y.md")
		testutil.Write(t, other, threeSteps)
		edit(e, other, nil)
		prompt(e).Has(t, "plan-x.md has open steps")
	})
	t.Run("another session's edit doesn't make it this one's", func(t *testing.T) {
		t.Parallel()
		e := setup(t)
		edit(e, e.plan(threeSteps), map[string]any{"session_id": "s2"})
		prompt(e).Empty(t)
	})
	t.Run("a plan the session wrote new isn't picked up, even when edited later", func(t *testing.T) {
		t.Parallel()
		e := setup(t)
		plan := e.plan(threeSteps)
		e.edit("Write", plan, false)
		edit(e, plan, map[string]any{"tool_name": "Write", "transcript_path": e.transcript})
		e.edit("Edit", plan, false)
		edit(e, plan, map[string]any{"transcript_path": e.transcript})
		prompt(e).Empty(t)
	})
	t.Run("a plan the session read before rewriting it is picked up", func(t *testing.T) {
		t.Parallel()
		e := setup(t)
		plan := e.plan(threeSteps)
		e.edit("Read", plan, false)
		e.edit("Write", plan, false)
		edit(e, plan, map[string]any{"tool_name": "Write", "transcript_path": e.transcript})
		prompt(e).Has(t, "Plan "+plan+" has open steps")
	})
}

// planChangesEnv is one session submitting a plan, with the transcript its
// answers are counted in.
type planChangesEnv struct {
	*sandbox
	proj, plan, transcript string
}

const planV1 = "# Plan\n\nWhy.\n\n## Steps\n\n- [ ] 1. one\n- [ ] 2. two\n\n## Risks\n\nNone.\n"

var planV2 = strings.Replace(planV1, "- [ ] 2. two\n", "- [ ] 2. two, split\n- [ ] 3. three\n", 1) + "\n## Rollback\n\nRevert.\n"

func planChangesSetup(t *testing.T) *planChangesEnv {
	k := newSandbox(t)
	e := &planChangesEnv{sandbox: k, proj: repo(t, filepath.Join(k.home, "proj")), transcript: filepath.Join(t.TempDir(), "s.jsonl")}
	e.plan = filepath.Join(e.proj, ".claude", "plans", "plan-x.md")
	e.prompt("plan it")
	return e
}

func (e *planChangesEnv) prompt(text string) {
	testutil.AppendJSONL(e.t, e.transcript, testutil.UserPrompt(text))
}

// ask is an AskUserQuestion and its result, an error when rejected.
func (e *planChangesEnv) ask(id string, rejected bool) {
	testutil.AppendJSONL(e.t, e.transcript, testutil.ToolUse(id, "AskUserQuestion", map[string]any{}),
		map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": rejected}}}})
}

func (e *planChangesEnv) submit(text string) result {
	e.t.Helper()
	return e.run("plan-changes", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "tool_name": "ExitPlanMode", "cwd": e.proj,
		"transcript_path": e.transcript, "tool_input": map[string]any{"plan": text, "planFilePath": e.plan}})
}

// reason is the deny's reason, for a result that denied.
func (r result) reason(t *testing.T) string {
	t.Helper()
	r.Denied(t)
	var out struct {
		HookSpecificOutput struct {
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatal(err)
	}
	return out.HookSpecificOutput.PermissionDecisionReason
}

func TestPlanChanges(t *testing.T) {
	t.Parallel()
	t.Run("a first submit and an unchanged resubmit print nothing", func(t *testing.T) {
		t.Parallel()
		e := planChangesSetup(t)
		e.submit(planV1).Empty(t)
		e.submit(planV1).Empty(t)
		e.submit(strings.ReplaceAll(planV1, "\n\n", "\n\n\n")).Empty(t) // blank lines alone
	})
	t.Run("a changed resubmit is denied, with each changed section, the diff, and last the plan's absolute path", func(t *testing.T) {
		t.Parallel()
		e := planChangesSetup(t)
		e.submit(planV1)
		msg := e.submit(planV2).reason(t)
		hasLine(t, msg,
			`changed  ## Steps  \+2 -1`,
			`added  ## Rollback  \+2`,
			`diff  .*kit-plan-changes\.diff`,
			`do    .*skills/kit-rules/reference/workflow\.md section 3.*`,
			`rule  plan-changes`,
		)
		body, _, _ := strings.Cut(msg, "\n\nrule  ")
		if !strings.HasSuffix(body, "\nplan  "+e.plan) {
			t.Errorf("the reason doesn't end with the plan's path:\n%s", msg)
		}
		if diff := read(t, filepath.Join(e.proj, ".claude", "scratch", "kit-plan-changes.diff")); !strings.Contains(diff, "+- [ ] 3. three") {
			t.Errorf("diff:\n%s", diff)
		}
	})
	t.Run("a section removed is listed", func(t *testing.T) {
		t.Parallel()
		e := planChangesSetup(t)
		e.submit(planV1)
		hasLine(t, e.submit(strings.Replace(planV1, "\n## Risks\n\nNone.\n", "", 1)).reason(t), `removed  ## Risks  -2`)
	})
	t.Run("the same text passes once the user typed, and only then", func(t *testing.T) {
		t.Parallel()
		e := planChangesSetup(t)
		e.submit(planV1)
		e.submit(planV2).Denied(t)
		e.submit(planV2).Denied(t)
		e.prompt("<task-notification>\n<task-id>a1</task-id>\n<status>completed</status>\n</task-notification>")
		e.submit(planV2).Denied(t)
		e.prompt("go")
		e.submit(planV2).Empty(t)
		e.submit(planV2).Empty(t)
	})
	t.Run("an answered AskUserQuestion counts, a rejected one doesn't", func(t *testing.T) {
		t.Parallel()
		e := planChangesSetup(t)
		e.submit(planV1)
		e.submit(planV2).Denied(t)
		e.ask("toolu_1", true)
		e.submit(planV2).Denied(t)
		e.ask("toolu_2", false)
		e.submit(planV2).Empty(t)
	})
	t.Run("an answer before the deny doesn't count", func(t *testing.T) {
		t.Parallel()
		e := planChangesSetup(t)
		e.submit(planV1)
		e.prompt("split step two")
		e.submit(planV2).Denied(t)
	})
	t.Run("a later edit is a new change: diffed against what was answered, else against what was last shown", func(t *testing.T) {
		t.Parallel()
		v3 := strings.Replace(planV2, "None.", "One.", 1)
		e := planChangesSetup(t)
		e.submit(planV1)
		e.submit(planV2).Denied(t)
		msg := e.submit(v3).reason(t)
		hasLine(t, msg, `added  ## Rollback  \+2`, `changed  ## Risks  \+1 -1`)
		e.submit(planV2).Denied(t)
		e.prompt("and the risk")
		msg = e.submit(v3).reason(t)
		hasLine(t, msg, `changed  ## Risks  \+1 -1`)
		if strings.Contains(msg, "## Rollback") {
			t.Errorf("lists a change the user answered:\n%s", msg)
		}
		e.prompt("go")
		e.submit(v3).Empty(t)
	})
	t.Run("a first submit that replaces a readable plan is diffed against it", func(t *testing.T) {
		t.Parallel()
		for _, ref := range []string{"Replaces `.claude/plans/plan-old.md`. Sources: that plan.", "Replaces .claude/plans/plan-old.md", "Replaces `plan-old.md`."} {
			e := planChangesSetup(t)
			testutil.Write(t, filepath.Join(e.proj, ".claude", "plans", "plan-old.md"), planV1)
			v2 := strings.Replace(planV2, "Why.", ref, 1)
			hasLine(t, e.submit(v2).reason(t), `changed  ## Steps  \+2 -1`, `added  ## Rollback  \+2`)
			e.prompt("go")
			e.submit(v2).Empty(t)
		}
	})
	t.Run("a held plan that passed is no longer what the next change is diffed against", func(t *testing.T) {
		t.Parallel()
		v3 := strings.Replace(planV2, "None.", "One.", 1)
		v4 := v3 + "\n## Extra\n\nMore.\n"
		e := planChangesSetup(t)
		e.submit(planV1)
		e.submit(planV2).Denied(t)
		e.prompt("go")
		e.submit(planV2).Empty(t)
		readable := e.transcript
		e.transcript = filepath.Join(t.TempDir(), "gone.jsonl")
		e.submit(v3).Allowed(t)
		e.transcript = readable
		e.prompt("and more")
		msg := e.submit(v4).reason(t)
		hasLine(t, msg, `added  ## Extra  \+2`)
		if strings.Contains(msg, "## Risks") {
			t.Errorf("diffed against the held plan, not the one last shown:\n%s", msg)
		}
	})
	t.Run("a first submit that replaces no readable file passes", func(t *testing.T) {
		t.Parallel()
		e := planChangesSetup(t)
		e.submit(strings.Replace(planV2, "Why.", "Replaces `.claude/plans/gone.md`.", 1)).Empty(t)
	})
	t.Run("a Replaces file outside the repo and the plan folder is never read", func(t *testing.T) {
		t.Parallel()
		outside := filepath.Join(t.TempDir(), "notes.md")
		testutil.Write(t, outside, planV1)
		for _, ref := range []string{"Replaces " + outside + ".", "Replaces ../../" + filepath.Base(filepath.Dir(outside)) + "/notes.md."} {
			e := planChangesSetup(t)
			e.submit(strings.Replace(planV2, "Why.", ref, 1)).Empty(t)
		}
	})
	t.Run("a first submit that says replaces in passing passes", func(t *testing.T) {
		t.Parallel()
		e := planChangesSetup(t)
		testutil.Write(t, filepath.Join(e.proj, ".claude", "plans", "plan-old.md"), planV1)
		e.submit(strings.Replace(planV2, "Why.", "This replaces `.claude/plans/plan-old.md` one day.", 1)).Empty(t)
	})
	t.Run("a transcript that can't be read passes, since no answer could lift the deny", func(t *testing.T) {
		t.Parallel()
		e := planChangesSetup(t)
		e.submit(planV1)
		e.transcript = filepath.Join(t.TempDir(), "gone.jsonl")
		e.submit(planV2).Allowed(t)
	})
	t.Run("plan-changes in disabled_rules or shadow_rules lets it through, and the next change is diffed against it", func(t *testing.T) {
		t.Parallel()
		for _, key := range []string{"disabled_rules", "shadow_rules"} {
			e := planChangesSetup(t)
			e.overlay(key + ": [plan-changes]\n")
			e.submit(planV1)
			e.submit(planV2).Allowed(t)
			e.overlay("")
			e.submit(planV2).Empty(t)
		}
	})
	t.Run("a plan another check sends back was never shown, so its fix is no change", func(t *testing.T) {
		t.Parallel()
		e := planChangesSetup(t)
		testutil.Write(t, filepath.Join(e.proj, "src", "stop.go"), "a\n")
		e.overlay("disabled_rules: [plan-critic]\nclaims:\n  timeout: 0\n")
		dispatch := func(text string) result {
			return e.run("plan-dispatch", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "tool_name": "ExitPlanMode", "cwd": e.proj,
				"transcript_path": e.transcript, "tool_input": map[string]any{"plan": text, "planFilePath": e.plan}})
		}
		r := dispatch(planV1 + "\n- src/stop.go:40 returns early.\n")
		r.Denied(t)
		r.Lacks(t, "plan-changes")
		dispatch(planV1).Allowed(t)
		dispatch(planV2).Has(t, "plan-changes")
	})
}
