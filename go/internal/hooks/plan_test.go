package hooks

import (
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/rotate"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestPlanStepPause(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*sandbox, string) {
		k := newSandbox(t)
		proj := filepath.Join(k.home, "proj")
		testutil.Mkdir(t, proj)
		testutil.Git(t, proj, "init", "-q")
		return k, proj
	}
	approve := func(k *sandbox, proj, session string) result {
		return k.run("plan-mode-context", map[string]string{
			"hook_event_name": "PostToolUse", "tool_name": "ExitPlanMode",
			"session_id": session, "cwd": proj,
		})
	}
	prompt := func(k *sandbox, proj, session string) result {
		return k.run("plan-mode-context", map[string]string{
			"hook_event_name": "UserPromptSubmit", "permission_mode": "default",
			"session_id": session, "cwd": proj,
		})
	}

	t.Run("ExitPlanMode asks for step 1 only and names the newest open plan", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		plans := filepath.Join(proj, ".claude", "plans")
		old := filepath.Join(plans, "plan-old.md")
		testutil.Write(t, old, "## Steps\n\n- [ ] one\n")
		testutil.Age(t, time.Hour, old)
		testutil.Write(t, filepath.Join(plans, "plan-new.md"), "## Steps\n\n- [x] one\n- [ ] two\n")
		r := approve(k, proj, "s1")
		r.Want(t, 0)
		r.Has(t, `"hookEventName":"PostToolUse"`, "plan-new.md", "first unchecked step")
	})
	t.Run("a later prompt reminds while the plan has an open step, then goes silent", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		plan := filepath.Join(proj, ".claude", "plans", "plan-x.md")
		testutil.Write(t, plan, "- [ ] one\n")
		approve(k, proj, "s1").Want(t, 0)
		r := prompt(k, proj, "s1")
		r.Want(t, 0)
		r.Has(t, "plan-x.md", "next unchecked one")
		prompt(k, proj, "other-session").Empty(t)
		testutil.Write(t, plan, "- [x] one\n")
		prompt(k, proj, "s1").Empty(t)
	})
	t.Run("a reminded plan's marker stays fresh, so scratch-rotate keeps it", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		testutil.Write(t, filepath.Join(proj, ".claude", "plans", "plan-x.md"), "- [ ] one\n")
		approve(k, proj, "s1").Want(t, 0)
		markers, _ := filepath.Glob(filepath.Join(k.claude, "cache", "plan-active", "*"))
		if len(markers) != 1 {
			t.Fatalf("plan-active markers = %v", markers)
		}
		testutil.Age(t, 72*time.Hour, markers[0])
		prompt(k, proj, "s1").Has(t, "plan-x.md")
		// kit scratch-rotate with its default 30 days.
		if status := rotate.Run(k.paths, 30, false, io.Discard, io.Discard); status != 0 {
			t.Fatalf("scratch-rotate exited %d", status)
		}
		prompt(k, proj, "s1").Has(t, "plan-x.md")
	})
	t.Run("ExitPlanMode with no open plan still asks for one step", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		r := approve(k, proj, "s1")
		r.Want(t, 0)
		r.Has(t, "first unchecked step")
		prompt(k, proj, "s1").Empty(t)
	})
	t.Run("another tool's PostToolUse is silent", func(t *testing.T) {
		t.Parallel()
		k, proj := setup(t)
		r := k.run("plan-mode-context", map[string]string{
			"hook_event_name": "PostToolUse", "tool_name": "Edit", "session_id": "s1", "cwd": proj,
		})
		r.Want(t, 0)
		r.Empty(t)
	})
}

func TestPlanModeContext(t *testing.T) {
	t.Parallel()
	for name, payload := range map[string]string{
		"default mode prints nothing":            `{"permission_mode":"default"}`,
		"missing permission_mode prints nothing": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newSandbox(t).run("plan-mode-context", payload)
			r.Want(t, 0)
			r.Empty(t)
		})
	}
}
