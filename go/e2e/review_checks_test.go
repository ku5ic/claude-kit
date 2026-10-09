package e2e

import "testing"

// review-checks runs the full suite once per /code-review, typed or invoked,
// and hands the result to Claude as context.
func TestReviewChecks(t *testing.T) {
	setup := func(t *testing.T, code int) *runChecksEnv {
		e := runChecksSetup(t)
		e.write("package.json", `{"scripts":{"lint":"eslint ."}}`+"\n")
		e.stub("npm", code)
		return e
	}
	typed := func(e *runChecksEnv) Result {
		return e.k.Hook("review-checks", map[string]any{"hook_event_name": "UserPromptExpansion", "command_name": "code-review", "cwd": e.project})
	}
	invoked := func(e *runChecksEnv, skill string) Result {
		return e.k.Hook("review-checks", map[string]any{"hook_event_name": "PostToolUse", "tool_name": "Skill",
			"tool_input": map[string]any{"skill": skill}, "cwd": e.project})
	}

	t.Run("a typed /code-review runs the suite and adds the result as context", func(t *testing.T) {
		e := setup(t, 0)
		r := typed(e)
		r.Want(t, 0)
		r.Has(t, `"hookEventName":"UserPromptExpansion"`, "kit run-checks ran for this /code-review and passed", "PASS js: lint (lint)")
		e.callsEqual("npm", e.phys(".")+" run lint")
	})
	t.Run("an invoked /code-review runs it too; a failure is a finding, never a block", func(t *testing.T) {
		e := setup(t, 1)
		r := invoked(e, "code-review")
		r.Want(t, 0)
		r.Has(t, `"hookEventName":"PostToolUse"`, "FAILED; each failure is a finding of this review", "FAIL js: lint (lint)")
	})
	t.Run("any other skill or command runs nothing", func(t *testing.T) {
		e := setup(t, 0)
		invoked(e, "simplify").Want(t, 0)
		e.k.Hook("review-checks", map[string]any{"hook_event_name": "UserPromptExpansion", "command_name": "verify", "cwd": e.project}).Want(t, 0)
		if e.called("npm") {
			t.Errorf("ran: %s", e.calls("npm"))
		}
	})
}
