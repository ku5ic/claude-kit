package hooks

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/rotate"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestPlanStepPause(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*sandbox, string) {
		k := newSandbox(t)
		proj := filepath.Join(k.home, "proj")
		testutil.Mkdir(t, proj)
		testutil.Git(t, proj, "init", "-q")
		testutil.Write(t, filepath.Join(proj, ".claude", "settings.json"), `{"plansDirectory": ".claude/plans"}`)
		return k, proj
	}
	approve := func(k *sandbox, proj, session string) result {
		return k.run("plan-mode-context", map[string]string{
			"hook_event_name": "PostToolUse", "tool_name": "ExitPlanMode",
			"session_id": session, "cwd": proj,
		})
	}
	prompt := func(k *sandbox, proj, session string) result {
		return k.run("plan-mode-context", map[string]string{
			"hook_event_name": "UserPromptSubmit", "permission_mode": "default",
			"session_id": session, "cwd": proj,
		})
	}

	t.Run("ExitPlanMode asks for step 1 only and names the newest open plan", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		plans := filepath.Join(proj, ".claude", "plans")
		old := filepath.Join(plans, "plan-old.md")
		testutil.Write(t, old, "## Steps\n\n- [ ] one\n")
		testutil.Age(t, time.Hour, old)
		testutil.Write(t, filepath.Join(plans, "plan-new.md"), "## Steps\n\n- [x] one\n- [ ] two\n")
		r := approve(k, proj, "s1")
		r.Want(t, 0)
		r.Has(t, `"hookEventName":"PostToolUse"`, "plan-new.md is approved under step_pause: step. Its first unchecked step is this turn's whole scope")
	})
	t.Run("with plansDirectory unset and no project plans dir, the plan in ~/.claude/plans is named", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		if err := os.Remove(filepath.Join(proj, ".claude", "settings.json")); err != nil {
			t.Fatal(err)
		}
		testutil.Write(t, filepath.Join(k.paths.PlansHome(), "plan-home.md"), "## Steps\n\n- [ ] one\n")
		approve(k, proj, "s1").Has(t, "plan-home.md")
	})
	t.Run("with plansDirectory unset, an existing project plans dir comes first", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		if err := os.Remove(filepath.Join(proj, ".claude", "settings.json")); err != nil {
			t.Fatal(err)
		}
		testutil.Write(t, filepath.Join(proj, ".claude", "plans", "plan-proj.md"), "## Steps\n\n- [ ] one\n")
		testutil.Write(t, filepath.Join(k.paths.PlansHome(), "plan-home.md"), "## Steps\n\n- [ ] one\n")
		approve(k, proj, "s1").Has(t, "plan-proj.md")
	})
	t.Run("a later prompt reminds while the plan has an open step, then goes silent", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		plan := filepath.Join(proj, ".claude", "plans", "plan-x.md")
		testutil.Write(t, plan, "- [ ] one\n")
		approve(k, proj, "s1").Want(t, 0)
		r := prompt(k, proj, "s1")
		r.Want(t, 0)
		r.Has(t, "plan-x.md", "has open steps. Under step_pause: step, a turn covers the next unchecked step, then review")
		prompt(k, proj, "other-session").Empty(t)
		testutil.Write(t, plan, "- [x] one\n")
		prompt(k, proj, "s1").Empty(t)
	})
	t.Run("a reminded plan's marker stays fresh, so scratch-rotate keeps it", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		testutil.Write(t, filepath.Join(proj, ".claude", "plans", "plan-x.md"), "- [ ] one\n")
		approve(k, proj, "s1").Want(t, 0)
		markers, _ := filepath.Glob(filepath.Join(k.claude, "cache", "plan-active", "*"))
		if len(markers) != 1 {
			t.Fatalf("plan-active markers = %v", markers)
		}
		testutil.Age(t, 72*time.Hour, markers[0])
		prompt(k, proj, "s1").Has(t, "plan-x.md")
		// kit scratch-rotate with its default 30 days.
		if status := rotate.Run(k.paths, "", 30, false, io.Discard, io.Discard); status != 0 {
			t.Fatalf("scratch-rotate exited %d", status)
		}
		prompt(k, proj, "s1").Has(t, "plan-x.md")
	})
	t.Run("ExitPlanMode with no open plan still asks for one step", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		r := approve(k, proj, "s1")
		r.Want(t, 0)
		r.Has(t, "first unchecked step")
		prompt(k, proj, "s1").Empty(t)
	})
	t.Run("one session with a plan in each of two worktrees keeps both", func(t *testing.T) {
		t.Parallel()
		k, main := setup(t)
		testutil.Git(t, main, "commit", "-q", "--allow-empty", "-m", "init")
		wt := filepath.Join(k.home, "wt")
		testutil.Git(t, main, "worktree", "add", "-q", wt)
		wt = testutil.Physical(t, wt)
		testutil.Write(t, filepath.Join(wt, ".claude", "settings.json"), `{"plansDirectory": ".claude/plans"}`)
		testutil.Write(t, filepath.Join(main, ".claude", "plans", "plan-main.md"), "- [ ] one\n")
		testutil.Write(t, filepath.Join(wt, ".claude", "plans", "plan-wt.md"), "- [ ] one\n")
		approve(k, main, "s1").Has(t, "plan-main.md")
		approve(k, wt, "s1").Has(t, "plan-wt.md")
		r := prompt(k, main, "s1")
		r.Has(t, "plan-main.md")
		r.Lacks(t, "plan-wt.md")
		prompt(k, wt, "s1").Has(t, "plan-wt.md")
	})
	t.Run("step_pause picks the pause line, from the overlay or the light preset", func(t *testing.T) {
		t.Parallel()
		for overlay, want := range map[string][]string{
			"step_pause: plan\n": {"step_pause: plan. Its unchecked steps run in order, and one review follows the last", "a turn covers the next unchecked step, and review waits for the last one"},
			"step_pause: off\n":  {"step_pause: off. Its unchecked steps run in order, with no review stop", "a turn covers the next unchecked step, with no review stop"},
			"preset: light\n":    {"step_pause: plan. Its unchecked steps run in order, and one review follows the last", "a turn covers the next unchecked step, and review waits for the last one"},
		} {
			k, proj := setup(t)
			k.overlay(overlay)
			testutil.Write(t, filepath.Join(proj, ".claude", "plans", "plan-x.md"), "- [ ] one\n")
			approve(k, proj, "s1").Has(t, want[0])
			r := prompt(k, proj, "s1")
			r.Has(t, "plan-x.md", want[1])
			r.Lacks(t, "step_pause: step")
		}
	})
	t.Run("another tool's PostToolUse is silent", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		r := k.run("plan-mode-context", map[string]string{
			"hook_event_name": "PostToolUse", "tool_name": "Edit", "session_id": "s1", "cwd": proj,
		})
		r.Want(t, 0)
		r.Empty(t)
	})
}

func TestGuardPlan(t *testing.T) {
	t.Parallel()
	const plan = "/proj/.claude/plans/plan-x.md"
	write := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"w","name":"Write","input":{"file_path":"` + plan + `"}}]}}`
	critic := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"c","name":"Agent","input":{"subagent_type":"kit:plan-critic"}}]}}`
	launched := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"c","content":"Async agent launched successfully.\nagentId: a1"}]}}`
	notice := func(status string) string {
		return `{"type":"user","message":{"content":"<task-notification>\n<task-id>a1</task-id>\n<tool-use-id>c</tool-use-id>\n<status>` + status + `</status>\n</task-notification>"}}`
	}
	exit := func(k *sandbox, lines ...string) map[string]any {
		path := filepath.Join(k.home, "session.jsonl")
		testutil.Write(t, path, strings.Join(lines, "\n")+"\n")
		return map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "tool_name": "ExitPlanMode",
			"transcript_path": path, "tool_input": map[string]any{"plan": "# X", "planFilePath": plan}}
	}
	t.Run("no critique: sent back once, naming the critic and the plan; resent, it goes through", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		r := k.run("guard-plan", exit(k, write))
		r.Denied(t)
		r.Has(t, "kit:plan-critic", plan)
		k.run("guard-plan", exit(k, write)).Allowed(t)
	})
	t.Run("a transcript that can't be read counts as no critique", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		for _, path := range []string{"", filepath.Join(k.home, "missing.jsonl")} {
			payload := exit(k, write)
			payload["transcript_path"] = path
			r := k.run("guard-plan", payload)
			r.Denied(t)
			r.Has(t, "kit:plan-critic")
			k = newSandbox(t)
		}
	})
	t.Run("a critique after the plan was written, then a revision: allowed", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.run("guard-plan", exit(k, write, critic, write)).Allowed(t)
	})
	t.Run("in one entry, a critique after the write counts and one before it doesn't", func(t *testing.T) {
		t.Parallel()
		writeBlock := `{"type":"tool_use","id":"w","name":"Write","input":{"file_path":"` + plan + `"}}`
		criticBlock := `{"type":"tool_use","id":"c","name":"Agent","input":{"subagent_type":"kit:plan-critic"}}`
		entry := func(blocks ...string) string {
			return `{"type":"assistant","message":{"content":[` + strings.Join(blocks, ",") + `]}}`
		}
		k := newSandbox(t)
		k.run("guard-plan", exit(k, entry(writeBlock, criticBlock))).Allowed(t)
		k = newSandbox(t)
		k.run("guard-plan", exit(k, entry(criticBlock, writeBlock))).Denied(t)
	})
	t.Run("a critique that errored, or ran in the background and failed: sent back", func(t *testing.T) {
		t.Parallel()
		failed := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"c","is_error":true,"content":"agent failed"}]}}`
		k := newSandbox(t)
		k.run("guard-plan", exit(k, write, critic, failed)).Denied(t)
		k = newSandbox(t)
		k.run("guard-plan", exit(k, write, critic, launched, notice("failed"))).Denied(t)
		// An api-request-blob echoes the call; the echo isn't a new run.
		blob := `{"type":"api-request-blob","message":{"role":"assistant","content":[{"type":"tool_use","id":"c","name":"Agent","input":{"subagent_type":"kit:plan-critic"}}]}}`
		k = newSandbox(t)
		k.run("guard-plan", exit(k, write, critic, failed, blob)).Denied(t)
	})
	t.Run("a critique that finished, or still runs in the background: allowed", func(t *testing.T) {
		t.Parallel()
		finished := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"c","content":"the plan holds"}]}}`
		for _, lines := range [][]string{{write, critic, finished}, {write, critic, launched}, {write, critic, launched, notice("completed")}} {
			k := newSandbox(t)
			k.run("guard-plan", exit(k, lines...)).Allowed(t)
		}
	})
	t.Run("a marker that can't be saved lets the plan through, and says so", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		// A file sits where the marker's directory goes, so the write fails.
		testutil.Write(t, filepath.Dir(k.paths.SessionFile(cache.PlanCritic, "s", "plan-x.md")), "")
		r := k.run("guard-plan", exit(k, write))
		r.Allowed(t)
		r.Has(t, "kit plan-critic: let through unchecked")
	})
	t.Run("a critique only before the plan was written: sent back", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.run("guard-plan", exit(k, critic, write)).Denied(t)
	})
	t.Run("disabled: allowed", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.overlay("disabled_rules: [plan-critic]\n")
		k.run("guard-plan", exit(k, write)).Allowed(t)
	})
	// A plan whose own marker gives a reason skips the critic, and the skip is
	// logged; a marker with no reason, quoted, or fenced is a mention.
	withPlan := func(k *sandbox, text string) map[string]any {
		in := exit(k, write)
		in["tool_input"] = map[string]any{"plan": text, "planFilePath": plan}
		return in
	}
	t.Run("a critic-skip marker with a reason: allowed, and logged with the reason", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.run("guard-plan", withPlan(k, "# X\n\n<!-- critic: skip renames one flag, no design choice -->\n\n## Steps\n")).Allowed(t)
		data, _ := os.ReadFile(filepath.Join(k.claude, "logs", "guards.jsonl"))
		if !strings.Contains(string(data), `"event":"critic-skip"`) || !strings.Contains(string(data), `"reason":"renames one flag, no design choice"`) {
			t.Errorf("guards.jsonl = %s", data)
		}
	})
	for name, text := range map[string]string{
		"no reason":     "# X\n\n<!-- critic: skip -->\n",
		"a code span":   "# X\n\nName `<!-- critic: skip <reason> -->` to skip it.\n",
		"a fenced line": "# X\n\n```markdown\n<!-- critic: skip renames one flag -->\n```\n",
		"inside prose":  "# X\n\nWrite <!-- critic: skip why --> on its own line.\n",
	} {
		t.Run("a critic-skip mention, "+name+": sent back", func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			k.run("guard-plan", withPlan(k, text)).Denied(t)
		})
	}
}

// Every check runs on one ExitPlanMode, so one deny names each reason
// instead of a round trip per check.
func TestPlanDispatchDeniesOnceForEveryReason(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	dir := repo(t, t.TempDir())
	testutil.Put(t, dir, "src/stop.go", "a\nb\nc\n")
	k.overlay("size_budgets:\n  plan: 5\ndisabled_rules: [plan-critic]\nclaims:\n  timeout: 0\n")
	r := k.run("plan-dispatch", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "cwd": dir, "tool_name": "ExitPlanMode",
		"tool_input": map[string]any{"plan": "# Plan\n\n- src/stop.go:40 returns early, so the gate never runs twice.", "planFilePath": filepath.Join(dir, ".claude/plans/p.md")}})
	r.Denied(t)
	r.Has(t, "over the 5-word budget", "citation(s) in the plan p.md that don't resolve")
}

// plan-critic's run covers a plan's claims. With plan-critic off, the plan
// waits for the claim-checker agent on its exact text.
func TestPlanDispatchVerifiesClaims(t *testing.T) {
	t.Parallel()
	const plan = "/proj/.claude/plans/plan-x.md"
	write := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"w","name":"Write","input":{"file_path":"` + plan + `"}}]}}`
	critic := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"c","name":"Agent","input":{"subagent_type":"kit:plan-critic"}}]}}`
	exit := func(k *sandbox, text string, lines ...string) map[string]any {
		path := filepath.Join(k.home, "session.jsonl")
		testutil.Write(t, path, strings.Join(lines, "\n")+"\n")
		return map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "tool_name": "ExitPlanMode",
			"transcript_path": path, "tool_input": map[string]any{"plan": text, "planFilePath": plan}}
	}
	t.Run("with plan-critic on, its run covers the claims", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("", claim)
		k.run("plan-dispatch", exit(k, "# Plan\n\n"+claim, write, critic)).Allowed(t)
		if got := testutil.Calls(t, calls); got != nil {
			t.Errorf("verifier ran: %q", got)
		}
	})
	t.Run("a plan revised after its critique gets the no-model pass", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("", claim)
		revised := exit(k, "# Plan\n\nSee `go/internal/nope/gone.go`.", write, critic, write)
		r := k.run("plan-dispatch", revised)
		r.Denied(t)
		r.Has(t, "gone.go")
		k.run("plan-dispatch", revised).Allowed(t)
		if got := testutil.Calls(t, calls); got != nil {
			t.Errorf("verifier ran: %q", got)
		}
	})
	t.Run("with plan-critic off, denied until the claim-checker ran on each of its statements", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.verifier("disabled_rules: [plan-critic]\n", claim)
		r := k.run("plan-dispatch", exit(k, "# Plan\n\n"+claim, write))
		r.Denied(t)
		r.Has(t, "kit:claim-checker", "the plan plan-x.md", "ExitPlanMode again")
		id := checkID.FindString(r.Stdout)
		k.run("plan-dispatch", exit(k, "# Plan\n\n"+claim, write, checkerLine(id))).Allowed(t)
		// A retitled plan states nothing new; one with a new sentence needs
		// a run on it.
		k.run("plan-dispatch", exit(k, "# Plan v2\n\n"+claim, write, checkerLine(id))).Allowed(t)
		k.run("plan-dispatch", exit(k, "# Plan v2\n\n"+claim+"\n\nThe cache lives in the session directory.", write, checkerLine(id))).Denied(t)
	})
	t.Run("a transcript that can't be read sends the plan back once, for Claude to check itself", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.verifier("disabled_rules: [plan-critic]\n", claim)
		p := exit(k, "# Plan\n\n"+claim)
		p["transcript_path"] = filepath.Join(k.home, "missing.jsonl")
		r := k.run("plan-dispatch", p)
		r.Denied(t)
		r.Has(t, "every claim in the plan plan-x.md", "check each claim with a tool yourself")
		k.run("plan-dispatch", p).Allowed(t)
	})
	t.Run("a critic-skip marker skips the critic, not the claim check", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.verifier("", claim)
		r := k.run("plan-dispatch", exit(k, "# Plan\n\n<!-- critic: skip renames one flag -->\n\n"+claim, write))
		r.Denied(t)
		r.Has(t, "kit:claim-checker", "the plan plan-x.md")
		r.Lacks(t, "kit:plan-critic agent hasn't reviewed")
	})
	t.Run("disabled_rules: [claims] lets the plan through", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.verifier("disabled_rules: [plan-critic, claims]\n", claim)
		k.run("plan-dispatch", exit(k, "# Plan\n\n"+claim, write)).Allowed(t)
	})
	t.Run("no critique yet: the critic gate denies and the verifier never runs", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		calls := k.verifier("", claim)
		k.run("plan-dispatch", exit(k, "# Plan\n\n"+claim, write)).Has(t, "kit:plan-critic")
		if got := testutil.Calls(t, calls); got != nil {
			t.Errorf("verifier ran before the critique: %q", got)
		}
	})
	t.Run("an earlier step's deny and the critic's ask come in one deny", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.verifier("size_budgets: {plan: 3}\n", claim)
		text := "# Plan\n\n" + claim
		k.run("plan-dispatch", exit(k, text, write)).Has(t, "words", "kit:plan-critic")
	})
	t.Run("resent with no critique, the plan's claims still get checked", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.verifier("", claim)
		text := "# Plan\n\n" + claim
		k.run("plan-dispatch", exit(k, text, write)).Has(t, "kit:plan-critic")
		r := k.run("plan-dispatch", exit(k, text, write))
		r.Denied(t)
		r.Has(t, "kit:claim-checker", "the plan plan-x.md")
	})
	t.Run("every claim confirmed: allowed", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		k.verifier("")
		k.run("plan-dispatch", exit(k, "# Plan\n\n"+claim, write, critic)).Allowed(t)
	})
}

func TestPlanModeContext(t *testing.T) {
	t.Parallel()
	for name, payload := range map[string]string{
		"default mode prints nothing":            `{"permission_mode":"default"}`,
		"missing permission_mode prints nothing": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newSandbox(t).run("plan-mode-context", payload)
			r.Want(t, 0)
			r.Empty(t)
		})
	}
}
