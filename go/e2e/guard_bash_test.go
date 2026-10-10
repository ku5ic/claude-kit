package e2e

import "testing"

// `kit hook guard-bash` wiring: the binary reads the payload, blocks with
// exit 2, and prints an ask as one PreToolUse JSON object. The rules
// themselves are tested in process, in internal/bashguard.
func TestGuardBash(t *testing.T) {
	t.Parallel()
	k := New(t)
	r := k.Hook("guard-bash", Payload("Bash", "rm -rf /", "", ""))
	r.Want(t, 2)
	r.Has(t, "Blocked by guard-bash:", "Command: rm -rf /")
	r = k.Hook("guard-bash", Payload("Bash", "git push origin feat/thing", "", ""))
	r.Want(t, 0)
	r.Has(t, `"hookEventName":"PreToolUse"`, `"permissionDecision":"ask"`)
	k.Hook("guard-bash", Payload("Bash", "ls -la", "", "")).Empty(t)
}
