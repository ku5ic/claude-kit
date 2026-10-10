package e2e

import "testing"

// `kit hook plan-mode-context` wiring: plan mode prints plain context, and
// a payload that doesn't parse exits clean. The plan-step pause is tested
// in process, in internal/hooks.
func TestPlanModeContext(t *testing.T) {
	t.Parallel()
	k := New(t)
	t.Run("plan mode prints the investigate pointer", func(t *testing.T) {
		r := k.Hook("plan-mode-context", `{"permission_mode":"plan"}`)
		r.Want(t, 0)
		r.Has(t, "investigate")
	})
	t.Run("invalid JSON exits clean with no output", func(t *testing.T) {
		r := k.Hook("plan-mode-context", "not json")
		r.Want(t, 0)
		r.Empty(t)
	})
}
