package e2e

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectSubagentContext(t *testing.T) {
	t.Parallel()
	t.Run("wraps the context in hookSpecificOutput, the only shape SubagentStart reads", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		proj := filepath.Join(t.TempDir(), "testproject")
		Mkdir(t, proj)
		k.Git(proj, "init", "-q")

		r := k.Hook("inject-subagent-context", map[string]any{"session_id": "s1", "cwd": proj})
		r.Want(t, 0)

		var out struct {
			HookSpecificOutput struct {
				HookEventName     string `json:"hookEventName"`
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, r.Stdout)
		}
		if got := out.HookSpecificOutput.HookEventName; got != "SubagentStart" {
			t.Errorf("hookEventName = %q, want SubagentStart\n%s", got, r.Stdout)
		}
		if ctx := out.HookSpecificOutput.AdditionalContext; !strings.Contains(ctx, "<scratch>") {
			t.Errorf("additionalContext lacks the <scratch> block\n%s", r.Stdout)
		}
	})
	t.Run("names bashguard's throwaway-commit allow lists", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		proj := filepath.Join(t.TempDir(), "testproject")
		Mkdir(t, proj)
		k.Git(proj, "init", "-q")

		r := k.Hook("inject-subagent-context", map[string]any{"session_id": "s1", "cwd": proj})
		r.Want(t, 0)
		for _, want := range []string{
			"runs only :, cd, cp, echo, git add, git commit, git init, mkdir, mktemp, printf, rm, set, true,",
			"Its rm, cp, and > write only inside this path and outside any .git.",
			"exports only GIT_AUTHOR_DATE, GIT_AUTHOR_EMAIL, GIT_AUTHOR_NAME, GIT_COMMITTER_DATE, GIT_COMMITTER_EMAIL, GIT_COMMITTER_NAME, HOME.",
		} {
			if !strings.Contains(r.Stdout, want) {
				t.Errorf("stdout lacks %q\n%s", want, r.Stdout)
			}
		}
	})
}
