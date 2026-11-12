package e2e

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// bash-dispatch runs guard-bash then guard-commit in one process: a block
// from either exits 2, and asks from both print one decision object.
func TestBashDispatch(t *testing.T) {
	t.Parallel()
	k := guardCommitStubbed(t, 0)
	k.Hook("bash-dispatch", guardCommitPayload("rm -rf ~", "")).Want(t, 2)

	r := k.Hook("bash-dispatch", guardCommitPayload(`git commit -m "fix: add sums" && git push`, ""))
	r.Want(t, 0)
	var out struct {
		HookSpecificOutput struct {
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatalf("stdout isn't one JSON object: %v\n%s", err, r.Stdout)
	}
	reason := out.HookSpecificOutput.PermissionDecisionReason
	if out.HookSpecificOutput.PermissionDecision != "ask" || !strings.Contains(reason, "push") || !strings.Contains(reason, "commit message") {
		t.Errorf("want one ask carrying both reasons; got %+v", out.HookSpecificOutput)
	}
}

// bash-dispatch names the patterns skill for a file a command reads.
func TestBashDispatchNudgesOnARead(t *testing.T) {
	t.Parallel()
	k := NewPlugin(t)
	k.KitYML("skill_file_map:\n  - on: basename\n    globs: [\"*.sh\"]\n    skills: [bash-patterns]\n")
	Write(t, filepath.Join(k.Claude, "logs/skills.jsonl"), "")
	dir := t.TempDir()
	Write(t, filepath.Join(dir, "run.sh"), "echo hi\n")
	r := k.Hook("bash-dispatch", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "cwd": dir,
		"tool_name": "Bash", "tool_input": map[string]any{"command": "cat run.sh"}})
	r.Want(t, 0)
	r.Has(t, "bash-patterns covers this file")
}

// prompt-dispatch: plan-mode-context's plain text and reply-length's
// context reach Claude in one JSON object, neither dropped.
func TestPromptDispatch(t *testing.T) {
	t.Parallel()
	r := New(t).Hook("prompt-dispatch", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "permission_mode": "plan", "prompt": "go"})
	r.Want(t, 0)
	var out struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatalf("stdout isn't one JSON object: %v\n%s", err, r.Stdout)
	}
	ctx := out.HookSpecificOutput.AdditionalContext
	if out.HookSpecificOutput.HookEventName != "UserPromptSubmit" || !strings.Contains(ctx, "load the investigate skill") || !strings.Contains(ctx, "check each claim") {
		t.Errorf("want the plan line and the claims line in one context; got %+v", out.HookSpecificOutput)
	}
}

// stop-dispatch measures the reply for the next prompt's correction.
func TestStopDispatch(t *testing.T) {
	t.Parallel()
	k := New(t)
	k.Hook("prompt-dispatch", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "prompt": "go"}).Want(t, 0)
	k.Hook("stop-dispatch", map[string]any{"hook_event_name": "Stop", "session_id": "s1", "stop_hook_active": false,
		"last_assistant_message": strings.TrimSpace(strings.Repeat("word ", 41))}).Want(t, 0)
	k.Hook("prompt-dispatch", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "prompt": "next"}).
		Has(t, "Your last reply ran 41 words against a 40-word ceiling")
}

// post-edit-dispatch runs sanitize-output before format-dispatch.
func TestPostEditDispatch(t *testing.T) {
	t.Parallel()
	k := New(t)
	k.Setenv("CLAUDE_SANITIZE_TYPOGRAPHY", "1")
	file := filepath.Join(t.TempDir(), "notes.txt")
	Write(t, file, "a "+sanitizeOutputEmDash+" b\n")
	k.Hook("post-edit-dispatch", map[string]any{"tool_input": map[string]any{"file_path": file}}).Want(t, 0)
	if got := Read(t, file); got != "a - b\n" {
		t.Errorf("not sanitized: %q", got)
	}
}
