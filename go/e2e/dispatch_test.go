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
	if out.HookSpecificOutput.PermissionDecision != "ask" || !strings.Contains(reason, "push") || !strings.Contains(reason, "doesn't ask for a commit") {
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

// bash-dispatch gives a file a command writes the path rules an Edit gets.
func TestBashDispatchGuardsWhatACommandWrites(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cmd, verdict, has string
	}{
		{"echo K=1 >> .env", "block", "writing a credential or key file"},
		{"cp hook .git/hooks/pre-commit", "block", "edit inside .git/"},
		{"echo x > go.sum", "block", "lockfile edit"},
		{"echo 'on: push' > .github/workflows/ci.yml", "ask", "CI workflow"},
		{"echo x > notes/a.md", "pass", ""},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			t.Parallel()
			k := NewPlugin(t)
			k.KitYML("sensitive_paths: [\".env\"]\nlockfile_globs: [go.sum]\n")
			r := k.Hook("bash-dispatch", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "cwd": t.TempDir(),
				"tool_name": "Bash", "tool_input": map[string]any{"command": tc.cmd}})
			switch tc.verdict {
			case "block":
				r.Want(t, 2)
				r.Has(t, tc.has)
			case "ask":
				r.Want(t, 0)
				r.Has(t, `"ask"`, tc.has)
			default:
				r.Want(t, 0)
				r.Lacks(t, `"ask"`, `"deny"`)
			}
		})
	}
}

// bash-dispatch sizes and claim-checks what a command posts through gh, as
// connector-dispatch does a connector post.
func TestBashDispatchChecksWhatACommandPosts(t *testing.T) {
	t.Parallel()
	const claim = "The build fails because Foo was renamed in v2."
	for _, tc := range []struct {
		name, cmd, has string
	}{
		{"a claim in a PR body", `gh pr create --title "Rename Foo" --body "` + claim + `"`, "kit:claim-checker"},
		{"a claim in an API body field", `gh api repos/o/r/issues/1/comments -f body="` + claim + `"`, "kit:claim-checker"},
		{"a PR body over the post budget", `gh pr create --title "Rename Foo" --body "` + strings.Repeat("Which one? ", 320) + `"`, "over the 300-word budget"},
		{"a question in a PR body", `gh pr create --title "Rename Foo" --body "Which branch should I use?"`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := New(t)
			r := k.Hook("bash-dispatch", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "cwd": t.TempDir(),
				"tool_name": "Bash", "tool_input": map[string]any{"command": tc.cmd}})
			r.Want(t, 0)
			if tc.has == "" {
				r.Lacks(t, `"deny"`)
				return
			}
			r.Has(t, `"deny"`, tc.has)
		})
	}
}

// bash-edits sizes and claim-checks a scratch report a command wrote, as
// guard-dispatch does a Write of it, and sends it back with a block.
func TestBashEditsChecksAReportACommandWrote(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, file, has string
	}{
		{"an investigation report", "explore-auth.md", "kit:claim-checker"},
		{"a scratch note", "notes.md", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := New(t)
			dir := t.TempDir()
			Write(t, filepath.Join(dir, ".claude", "scratch", tc.file), "# Explore\n\nThe build fails because Foo was renamed in v2.\n")
			r := k.Hook("bash-edits", map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": dir,
				"tool_name": "Bash", "tool_input": map[string]any{"command": "cat > .claude/scratch/" + tc.file + " <<'EOF'\nreport\nEOF"}})
			if tc.has == "" {
				r.Want(t, 0)
				r.Lacks(t, tc.file)
				return
			}
			r.Want(t, 2)
			r.Has(t, tc.file, tc.has)
		})
	}
}

// A git move counts as alone only when nothing else in the command writes:
// a builtin's redirect beside it, or on the git call itself, is checked.
func TestBashEditsChecksAWriteBesideAGitMove(t *testing.T) {
	t.Parallel()
	const tell = "In today's fast-paced world, it serves as a testament to quality.\n"
	for _, tc := range []struct {
		cmd     string
		checked bool
	}{
		{"printf x > notes.md && git checkout -- README.md", true},
		{"git stash show -p > notes.md", true},
		{"git checkout -- README.md", false},
		{"git checkout -- README.md 2>/dev/null", false},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			t.Parallel()
			k := New(t)
			dir := Physical(t, t.TempDir())
			k.Git(dir, "init", "-q")
			Write(t, filepath.Join(dir, "README.md"), "Readme.\n")
			k.Git(dir, "add", "-A")
			k.Git(dir, "commit", "-q", "-m", "init")
			k.Hook("prompt-dispatch", map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": dir, "prompt": "go"})
			Write(t, filepath.Join(dir, "notes.md"), "Notes.\n"+tell)
			r := k.Hook("bash-edits", map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": dir,
				"tool_name": "Bash", "tool_input": map[string]any{"command": tc.cmd}})
			if !tc.checked {
				r.Want(t, 0)
				return
			}
			r.Want(t, 2)
			r.Has(t, "notes.md")
		})
	}
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
	if out.HookSpecificOutput.HookEventName != "UserPromptSubmit" || !strings.Contains(ctx, "starts from the investigate skill's findings") || !strings.Contains(ctx, "a claim stands only when a tool checked it") {
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
		Has(t, "The last reply ran 41 words against a 40-word ceiling")
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
