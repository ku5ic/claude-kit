package e2e

import (
	"path/filepath"
	"testing"
)

// review-checks holds a forked /code-review at its first stop and sends it
// back to run the suite itself; it never runs the suite.
func TestReviewChecks(t *testing.T) {
	setup := func(t *testing.T, sidecar string) (*runChecksEnv, string) {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		e.stub("npm", 0)
		dir := t.TempDir()
		transcript := filepath.Join(dir, "agent-x.jsonl")
		Write(t, transcript, "")
		if sidecar != "" {
			Write(t, filepath.Join(dir, "agent-x.forked-skill.json"), sidecar)
		}
		return e, transcript
	}
	stop := func(e *runChecksEnv, transcript string, active bool) Result {
		return e.k.Hook("review-checks", map[string]any{"hook_event_name": "SubagentStop", "agent_type": "general-purpose",
			"agent_transcript_path": transcript, "stop_hook_active": active, "cwd": e.project})
	}
	quiet := func(t *testing.T, e *runChecksEnv, r Result) {
		t.Helper()
		r.Want(t, 0)
		r.Empty(t)
		if e.called("npm") {
			t.Errorf("ran: %s", e.calls("npm"))
		}
	}

	t.Run("a forked /code-review is sent back once to run the checks itself", func(t *testing.T) {
		e, transcript := setup(t, `{"skillName":"code-review","effort":"medium"}`)
		r := stop(e, transcript, false)
		r.Want(t, 2)
		r.Has(t, "run `kit run-checks` in the checkout you reviewed", "timed per rules/tooling.md section 2", "a target you didn't check out, don't run it",
			"return your full report again, every finding you already had unchanged", "per rules/verify.md section 1")
		if e.called("npm") {
			t.Errorf("the hook ran the suite: %s", e.calls("npm"))
		}
		quiet(t, e, stop(e, transcript, true))
	})
	t.Run("a review that already ran kit run-checks stops freely", func(t *testing.T) {
		e, transcript := setup(t, `{"skillName":"code-review"}`)
		Write(t, transcript, `{"type":"user","message":{"content":"review this"}}`+"\n"+
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"kit run-checks","timeout":600000}}]}}`+"\n")
		quiet(t, e, stop(e, transcript, false))
	})
	t.Run("a review that only listed the checks with --plan is still sent back", func(t *testing.T) {
		e, transcript := setup(t, `{"skillName":"code-review"}`)
		Write(t, transcript, `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"kit run-checks --plan | head"}}]}}`+"\n")
		stop(e, transcript, false).Want(t, 2)
	})
	t.Run("a review that only mentioned kit run-checks is still sent back", func(t *testing.T) {
		e, transcript := setup(t, `{"skillName":"code-review"}`)
		Write(t, transcript, `{"type":"assistant","message":{"content":[{"type":"text","text":"I'll run kit run-checks later"}]}}`+"\n")
		stop(e, transcript, false).Want(t, 2)
	})
	t.Run("a review outside any project stops freely", func(t *testing.T) {
		e, transcript := setup(t, `{"skillName":"code-review"}`)
		r := e.k.Hook("review-checks", map[string]any{"hook_event_name": "SubagentStop", "agent_type": "general-purpose",
			"agent_transcript_path": transcript, "stop_hook_active": false, "cwd": t.TempDir()})
		quiet(t, e, r)
	})
	t.Run("another forked skill stops freely", func(t *testing.T) {
		e, transcript := setup(t, `{"skillName":"simplify"}`)
		quiet(t, e, stop(e, transcript, false))
	})
	t.Run("a plain subagent, with no sidecar, stops freely", func(t *testing.T) {
		e, transcript := setup(t, "")
		quiet(t, e, stop(e, transcript, false))
	})
}
