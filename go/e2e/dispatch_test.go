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

// post-edit-dispatch runs sanitize-output before format-dispatch.
func TestPostEditDispatch(t *testing.T) {
	k := New(t)
	k.Setenv("CLAUDE_SANITIZE_TYPOGRAPHY", "1")
	file := filepath.Join(t.TempDir(), "notes.txt")
	Write(t, file, "a "+sanitizeOutputEmDash+" b\n")
	k.Hook("post-edit-dispatch", map[string]any{"tool_input": map[string]any{"file_path": file}}).Want(t, 0)
	if got := Read(t, file); got != "a - b\n" {
		t.Errorf("not sanitized: %q", got)
	}
}
