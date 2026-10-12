package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// `kit hook guard-dispatch` wiring: guard-edit's and, with
// CLAUDE_GUARD_SKILLS=1, guard-skills' checks run against one payload read,
// the first block exits 2, and each check fails open on its own. The checks
// and their order are tested in process, in internal/hooks.
func TestGuardDispatch(t *testing.T) {
	t.Parallel()
	sandbox := func(t *testing.T) *Kit {
		k := NewPlugin(t)
		k.Setenv("CLAUDE_GUARD_SKILLS", "1")
		return k
	}

	t.Run("guard-edit's check fires first and blocks a lockfile edit", func(t *testing.T) {
		k := sandbox(t)
		k.KitYML("lockfile_globs: [package-lock.json]\n")
		r := k.Hook("guard-dispatch", map[string]any{
			"tool_input": map[string]any{"file_path": "/tmp/project/package-lock.json", "content": "harmless content"},
			"session_id": "s1",
			"tool_name":  "Write",
		})
		r.Want(t, 2)
		r.Has(t, "Blocked by guard-edit:")
	})

	// Malformed JSON breaks every check's payload read alike, so this can't
	// isolate one check's failure from another's success (internal/hook's
	// tests do), but it proves the dispatcher fails open end to end, with a
	// notice per check.
	t.Run("malformed JSON payload fails open through both checks instead of erroring out", func(t *testing.T) {
		r := sandbox(t).Hook("guard-dispatch", "not valid json")
		r.Want(t, 0)
		r.Has(t, "guard-edit: unexpected error, failing open", "guard-skills: unexpected error, failing open")
	})

	t.Run("each fail-open is logged to guards.jsonl, since its stderr reaches no one", func(t *testing.T) {
		k := sandbox(t)
		k.Hook("guard-dispatch", "not valid json").Want(t, 0)
		log := Read(t, filepath.Join(k.Claude, "logs/guards.jsonl"))
		for _, hook := range []string{"guard-edit", "guard-skills"} {
			if !strings.Contains(log, `"hook":"`+hook+`","event":"fail-open"`) {
				t.Errorf("no fail-open line for %s:\n%s", hook, log)
			}
		}
	})

	t.Run("empty stdin payload fails open cleanly", func(t *testing.T) {
		sandbox(t).Hook("guard-dispatch", "").Want(t, 0)
	})
}
