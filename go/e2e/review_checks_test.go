package e2e

import (
	"path/filepath"
	"testing"
)

// `kit hook review-checks` wiring: at a forked /code-review's first stop the
// hook runs the full gate itself and shows the user every verdict; the
// second stop goes through untouched. Which reviewers it gates, and a
// failing gate, are tested in process, in internal/hooks.
func TestReviewChecks(t *testing.T) {
	t.Parallel()
	t.Run("a forked /code-review's first stop runs the full gate once", func(t *testing.T) {
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
		r.Want(t, 0)
		r.Has(t, `"systemMessage":"run-checks checks: `)
		r = stop(true)
		r.Want(t, 0)
		r.Empty(t)
	})
}
