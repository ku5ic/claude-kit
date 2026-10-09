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
		r.Has(t, "run `kit run-checks`", "checks: N passed, M failed", "local checks don't apply",
			"return your full report again, every finding you already had unchanged", "at least 90% sure", "never twice")
		if e.called("npm") {
			t.Errorf("the hook ran the suite: %s", e.calls("npm"))
		}
		quiet(t, e, stop(e, transcript, true))
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
