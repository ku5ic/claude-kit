package e2e

import "testing"

func TestPlanModeContext(t *testing.T) {
	k := New(t)
	t.Run("plan mode prints the investigate pointer", func(t *testing.T) {
		r := k.Hook("plan-mode-context", `{"permission_mode":"plan"}`)
		r.Want(t, 0)
		r.Has(t, "investigate")
	})
	for name, payload := range map[string]string{
		"default mode prints nothing":             `{"permission_mode":"default"}`,
		"missing permission_mode prints nothing":  `{}`,
		"invalid JSON exits clean with no output": "not json",
	} {
		t.Run(name, func(t *testing.T) {
			r := k.Hook("plan-mode-context", payload)
			r.Want(t, 0)
			r.Empty(t)
		})
	}
}
