package e2e

import (
	"path/filepath"
	"testing"
)

// `kit hook review-checks` wiring: a forked /code-review is held at its
// first stop with exit 2 and let through on the second; the hook never runs
// the suite itself. Which reviewers it holds is tested in process, in
// internal/hooks.
func TestReviewChecks(t *testing.T) {
	t.Parallel()
	t.Run("a forked /code-review is sent back once to run the checks itself", func(t *testing.T) {
		t.Parallel()
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		e.stub("npm", 0)
		dir := t.TempDir()
		transcript := filepath.Join(dir, "agent-x.jsonl")
		Write(t, transcript, "")
		Write(t, filepath.Join(dir, "agent-x.forked-skill.json"), `{"skillName":"code-review","effort":"medium"}`)
		stop := func(active bool) Result {
			return e.k.Hook("review-checks", map[string]any{"hook_event_name": "SubagentStop", "agent_type": "general-purpose",
				"agent_transcript_path": transcript, "stop_hook_active": active, "cwd": e.project})
		}

		r := stop(false)
		r.Want(t, 2)
		r.Has(t, "run `kit run-checks` in the checkout you reviewed", "timed per rules/tooling.md section 2", "a target you didn't check out, don't run it",
			"return your full report again, every finding you already had unchanged", "per rules/verify.md section 1")
		r = stop(true)
		r.Want(t, 0)
		r.Empty(t)
		if e.called("npm") {
			t.Errorf("the hook ran the suite: %s", e.calls("npm"))
		}
	})
}
