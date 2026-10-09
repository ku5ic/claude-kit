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
	typed := func(e *runChecksEnv, args string) Result {
		return e.k.Hook("review-checks", map[string]any{"hook_event_name": "UserPromptExpansion", "command_name": "code-review",
			"command_args": args, "cwd": e.project})
	}
	invoked := func(e *runChecksEnv, skill string) Result {
		return e.k.Hook("review-checks", map[string]any{"hook_event_name": "PostToolUse", "tool_name": "Skill",
			"tool_input": map[string]any{"skill": skill}, "cwd": e.project})
	}

	t.Run("a typed /code-review runs the suite and adds the result as context", func(t *testing.T) {
		e := setup(t, 0)
		r := typed(e, "")
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
	t.Run("a review of a PR or another branch skips the suite and says why", func(t *testing.T) {
		for name, r := range map[string]func(*runChecksEnv) Result{
			"typed PR URL": func(e *runChecksEnv) Result { return typed(e, "high https://github.com/o/r/pull/8474") },
			"invoked PR number": func(e *runChecksEnv) Result {
				return e.k.Hook("review-checks", map[string]any{"hook_event_name": "PostToolUse", "tool_name": "Skill",
					"tool_input": map[string]any{"skill": "code-review", "args": "8474 --comment"}, "cwd": e.project})
			},
			"other branch": func(e *runChecksEnv) Result { return typed(e, "feature/x") },
		} {
			t.Run(name, func(t *testing.T) {
				e := setup(t, 0)
				res := r(e)
				res.Want(t, 0)
				res.Has(t, "kit run-checks did not run", "not the working tree")
				if e.called("npm") {
					t.Errorf("ran: %s", e.calls("npm"))
				}
			})
		}
	})
	t.Run("a working-tree target still runs the suite", func(t *testing.T) {
		for _, args := range []string{"high --fix --max-findings 5", "main", "package.json"} {
			t.Run(args, func(t *testing.T) {
				e := setup(t, 0)
				typed(e, args).Has(t, "kit run-checks ran for this /code-review and passed")
			})
		}
	})
}
