package e2e

import (
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// review-checks holds a forked /code-review at its first stop and sends it
// back to run the suite itself; it never runs the suite.
func TestReviewChecks(t *testing.T) {
	t.Parallel()
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
	bash := func(command string) any { return testutil.ToolUse("", "Bash", map[string]any{"command": command}) }
	quiet := func(t *testing.T, e *runChecksEnv, r Result) {
		t.Helper()
		r.Want(t, 0)
		r.Empty(t)
		if e.called("npm") {
			t.Errorf("ran: %s", e.calls("npm"))
		}
	}

	t.Run("a forked /code-review is sent back once to run the checks itself", func(t *testing.T) {
		t.Parallel()
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
	t.Run("a review whose tool output shows a run-checks summary stops freely", func(t *testing.T) {
		t.Parallel()
		e, transcript := setup(t, `{"skillName":"code-review"}`)
		testutil.AppendJSONL(t, transcript, testutil.UserPrompt("review this"),
			testutil.ToolUse("t1", "Bash", map[string]any{"command": "if time kit run-checks; then :; fi"}),
			testutil.ToolResult("t1", []any{map[string]any{"type": "text", "text": "PASS go: vet\nchecks: 5 passed, 0 failed, 1 skipped"}}))
		quiet(t, e, stop(e, transcript, false))
	})
	t.Run("a review that only listed the checks with --plan is still sent back", func(t *testing.T) {
		t.Parallel()
		e, transcript := setup(t, `{"skillName":"code-review"}`)
		testutil.AppendJSONL(t, transcript, bash("kit run-checks --plan | head"))
		stop(e, transcript, false).Want(t, 2)
	})
	t.Run("a search or echo of the text, or --plan after a line continuation, is still sent back", func(t *testing.T) {
		t.Parallel()
		e, transcript := setup(t, `{"skillName":"code-review"}`)
		testutil.AppendJSONL(t, transcript, bash(`rg -n "kit run-checks" rules/`),
			bash("echo next: kit run-checks && git log --grep 'wire kit run-checks'"),
			bash("kit run-checks \\\n  --plan"))
		stop(e, transcript, false).Want(t, 2)
	})
	t.Run("a review that only mentioned kit run-checks is still sent back", func(t *testing.T) {
		t.Parallel()
		e, transcript := setup(t, `{"skillName":"code-review"}`)
		testutil.AppendJSONL(t, transcript, testutil.AssistantText("I'll run kit run-checks later"))
		stop(e, transcript, false).Want(t, 2)
	})
	t.Run("a review outside any project stops freely", func(t *testing.T) {
		t.Parallel()
		e, transcript := setup(t, `{"skillName":"code-review"}`)
		r := e.k.Hook("review-checks", map[string]any{"hook_event_name": "SubagentStop", "agent_type": "general-purpose",
			"agent_transcript_path": transcript, "stop_hook_active": false, "cwd": t.TempDir()})
		quiet(t, e, r)
	})
	t.Run("another forked skill stops freely", func(t *testing.T) {
		t.Parallel()
		e, transcript := setup(t, `{"skillName":"simplify"}`)
		quiet(t, e, stop(e, transcript, false))
	})
	t.Run("a plain subagent, with no sidecar, stops freely", func(t *testing.T) {
		t.Parallel()
		e, transcript := setup(t, "")
		quiet(t, e, stop(e, transcript, false))
	})
	// A model-invoked foreground fork writes no .forked-skill.json; the
	// parent's pending Skill call names it.
	parent := func(t *testing.T, lines ...any) string {
		path := filepath.Join(t.TempDir(), "parent.jsonl")
		testutil.AppendJSONL(t, path, lines...)
		return path
	}
	skillCall := testutil.ToolUse("s1", "Skill", map[string]any{"skill": "code-review"})
	withParent := func(e *runChecksEnv, transcript, parentPath string) Result {
		return e.k.Hook("review-checks", map[string]any{"hook_event_name": "SubagentStop", "agent_type": "general-purpose",
			"agent_transcript_path": transcript, "transcript_path": parentPath, "stop_hook_active": false, "cwd": e.project})
	}
	t.Run("a fork with no sidecar is a review when the parent's code-review Skill call is pending", func(t *testing.T) {
		t.Parallel()
		e, transcript := setup(t, "")
		withParent(e, transcript, parent(t, skillCall)).Want(t, 2)
	})
	t.Run("a subagent after a finished review call stops freely", func(t *testing.T) {
		t.Parallel()
		e, transcript := setup(t, "")
		done := testutil.ToolResult("s1", `Skill "code-review" completed (forked execution).`)
		quiet(t, e, withParent(e, transcript, parent(t, skillCall, done)))
	})
	t.Run("a run-checks summary in persisted output counts", func(t *testing.T) {
		t.Parallel()
		e, transcript := setup(t, `{"skillName":"code-review"}`)
		saved := filepath.Join(t.TempDir(), "out.txt")
		Write(t, saved, "PASS go: vet\n...\nchecks: 5 passed, 0 failed, 1 skipped\n")
		testutil.AppendJSONL(t, transcript, testutil.ToolResult("t1",
			"<persisted-output>\nOutput too large (84.2KB). Full output saved to: "+saved+"\n\nPreview (first 2KB):\nPASS go: vet\n</persisted-output>"))
		quiet(t, e, stop(e, transcript, false))
	})
}
