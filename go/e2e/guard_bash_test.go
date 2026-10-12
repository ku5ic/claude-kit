package e2e

import (
	"path/filepath"
	"testing"
)

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

// A hook's log trims itself once it passes the overlay's log_max_lines by a
// tenth (at least one line).
func TestHookLogsTrimOnAppend(t *testing.T) {
	t.Parallel()
	k := New(t)
	k.Overlay("log_max_lines: 2\n")
	for range 5 {
		k.Hook("guard-bash", Payload("Bash", "find . -delete", "s1", "")).Want(t, 2)
	}
	if n := len(JSONLines(t, filepath.Join(k.Claude, "logs/guards.jsonl"))); n != 3 {
		t.Errorf("guards.jsonl has %d lines, want 3: trimmed to 2 at the fourth block, then one more", n)
	}
}
