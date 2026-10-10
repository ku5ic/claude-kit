package e2e

import "testing"

// `kit hook guard-edit` wiring: the binary reads the payload and prints an
// ask as one PreToolUse JSON object. The checks themselves are tested in
// process, in internal/hooks.
func TestGuardEdit(t *testing.T) {
	t.Parallel()
	t.Run("ask: Edit a CI workflow, which rules/workflow.md says needs confirmation", func(t *testing.T) {
		t.Parallel()
		r := New(t).Hook("guard-edit", Payload("Edit", "/tmp/project/.github/workflows/ci.yml", "", ""))
		r.Want(t, 0)
		r.Has(t, `"permissionDecision":"ask"`, "CI workflow")
	})
}
