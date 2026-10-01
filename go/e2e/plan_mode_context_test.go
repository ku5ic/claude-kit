package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlanStepPause(t *testing.T) {
	setup := func(t *testing.T) (*Kit, string) {
		k := New(t)
		proj := filepath.Join(k.Home, "proj")
		Mkdir(t, proj)
		k.Git(proj, "init", "-q")
		return k, Physical(t, proj)
	}
	approve := func(k *Kit, proj, session string) Result {
		return k.Hook("plan-mode-context", map[string]string{
			"hook_event_name": "PostToolUse", "tool_name": "ExitPlanMode",
			"session_id": session, "cwd": proj,
		})
	}
	prompt := func(k *Kit, proj, session string) Result {
		return k.Hook("plan-mode-context", map[string]string{
			"hook_event_name": "UserPromptSubmit", "permission_mode": "default",
			"session_id": session, "cwd": proj,
		})
	}

	t.Run("ExitPlanMode asks for step 1 only and names the newest open plan", func(t *testing.T) {
		k, proj := setup(t)
		plans := filepath.Join(proj, ".claude", "plans")
		old := filepath.Join(plans, "plan-old.md")
		Write(t, old, "## Steps\n\n- [ ] one\n")
		os.Chtimes(old, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour))
		Write(t, filepath.Join(plans, "plan-new.md"), "## Steps\n\n- [x] one\n- [ ] two\n")
		r := approve(k, proj, "s1")
		r.Want(t, 0)
		r.Has(t, `"hookEventName":"PostToolUse"`, "plan-new.md", "first unchecked step")
	})

	t.Run("a later prompt reminds while the plan has an open step, then goes silent", func(t *testing.T) {
		k, proj := setup(t)
		plan := filepath.Join(proj, ".claude", "plans", "plan-x.md")
		Write(t, plan, "- [ ] one\n")
		approve(k, proj, "s1").Want(t, 0)
		r := prompt(k, proj, "s1")
		r.Want(t, 0)
		r.Has(t, "plan-x.md", "next unchecked one")
		prompt(k, proj, "other-session").Empty(t)
		Write(t, plan, "- [x] one\n")
		prompt(k, proj, "s1").Empty(t)
	})

	t.Run("ExitPlanMode with no open plan still asks for one step", func(t *testing.T) {
		k, proj := setup(t)
		r := approve(k, proj, "s1")
		r.Want(t, 0)
		r.Has(t, "first unchecked step")
		prompt(k, proj, "s1").Empty(t)
	})

	t.Run("another tool's PostToolUse is silent", func(t *testing.T) {
		k, proj := setup(t)
		r := k.Hook("plan-mode-context", map[string]string{
			"hook_event_name": "PostToolUse", "tool_name": "Edit", "session_id": "s1", "cwd": proj,
		})
		r.Want(t, 0)
		r.Empty(t)
	})
}

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
