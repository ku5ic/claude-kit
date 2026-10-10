package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// log-skills appends one JSONL line per skill activation to
// $HOME/.claude/logs/skills.jsonl, for three payload shapes only: a
// PostToolUse Skill-tool call, a PostToolUse Read of a .../skills/*/SKILL.md
// path, and a UserPromptExpansion with expansion_type "slash_command". A
// substring pre-filter on the raw payload (SKILL.md / Skill / slash_command)
// rejects everything else before require_jq is even reached, so a missing
// jq never breaks an unrelated hook invocation.
//
// Each test fakes $HOME so the log never lands in the real
// ~/.claude/logs/skills.jsonl.
func TestLogSkills(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*Kit, string) {
		k := New(t)
		return k, filepath.Join(k.Claude, "logs", "skills.jsonl")
	}
	lineCount := func(t *testing.T, log string) int { return len(Lines(Read(t, log))) }

	// loggable shapes: each must append exactly one line

	t.Run("PostToolUse Skill-tool call is logged with skill_file from tool_input.skill", func(t *testing.T) {
		t.Parallel()
		k, log := setup(t)
		k.Hook("log-skills", `{"hook_event_name":"PostToolUse","tool_name":"Skill","tool_input":{"skill":"bash-patterns"},"session_id":"s1","cwd":"/x"}`)
		if n := lineCount(t, log); n != 1 {
			t.Fatalf("%d log lines, want 1", n)
		}
		body := Read(t, log)
		for _, want := range []string{`"skill_file":"bash-patterns"`, `"event":"PostToolUse"`} {
			if !strings.Contains(body, want) {
				t.Errorf("log lacks %q:\n%s", want, body)
			}
		}
	})

	t.Run("PostToolUse Read of a SKILL.md path is logged with skill_file from the file path", func(t *testing.T) {
		t.Parallel()
		k, log := setup(t)
		k.Hook("log-skills", `{"hook_event_name":"PostToolUse","tool_name":"Read","tool_input":{"file_path":"/Users/x/.claude/skills/bash-patterns/SKILL.md"},"session_id":"s1","cwd":"/x"}`)
		if n := lineCount(t, log); n != 1 {
			t.Fatalf("%d log lines, want 1", n)
		}
		if body := Read(t, log); !strings.Contains(body, "skills/bash-patterns/SKILL.md") {
			t.Errorf("log lacks the SKILL.md path:\n%s", body)
		}
	})

	t.Run("UserPromptExpansion with expansion_type slash_command is logged", func(t *testing.T) {
		t.Parallel()
		k, log := setup(t)
		k.Hook("log-skills", `{"hook_event_name":"UserPromptExpansion","expansion_type":"slash_command","command_name":"flow-test","session_id":"s1","cwd":"/x"}`)
		if n := lineCount(t, log); n != 1 {
			t.Fatalf("%d log lines, want 1", n)
		}
		if body := Read(t, log); !strings.Contains(body, `"command_name":"flow-test"`) {
			t.Errorf("log lacks command_name:\n%s", body)
		}
	})

	// non-loggable shapes: no log entry; pre-filter is intentionally
	// over-inclusive: correctness lives downstream
	for _, tc := range []struct{ name, payload string }{
		{"an ordinary Read of a non-SKILL.md file logs nothing",
			`{"hook_event_name":"PostToolUse","tool_name":"Read","tool_input":{"file_path":"/tmp/project/foo.ts"},"session_id":"s1","cwd":"/x"}`},
		{"a PostToolUse call for an unrelated tool logs nothing",
			`{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"ls -la"},"session_id":"s1","cwd":"/x"}`},
		{"UserPromptExpansion with a non-slash_command expansion_type logs nothing",
			`{"hook_event_name":"UserPromptExpansion","expansion_type":"keyword","command_name":"foo","session_id":"s1","cwd":"/x"}`},
		{"an unrelated hook_event_name (PreToolUse) logs nothing",
			`{"hook_event_name":"PreToolUse","tool_name":"Edit","tool_input":{"file_path":"/tmp/foo.md"},"session_id":"s1","cwd":"/x"}`},
		// PreToolUse Bash call whose command text happens to contain the word
		// 'Skill' -- passes the substring pre-filter but the event-type case
		// below still rejects it, proving the pre-filter is a fast-reject
		// optimization, not a replacement for the real routing logic.
		{"a payload that incidentally contains the substring 'Skill' but is not a loggable shape still logs nothing",
			`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo loading a Skill now"},"session_id":"s1","cwd":"/x"}`},
		{"a slash-command-shaped command_name passes the pre-filter but a non-slash_command expansion_type is still rejected downstream",
			`{"hook_event_name":"UserPromptExpansion","expansion_type":"keyword","command_name":"/some-Skill-name","session_id":"s1","cwd":"/x"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k, log := setup(t)
			k.Hook("log-skills", tc.payload)
			if n := lineCount(t, log); n != 0 {
				t.Errorf("%d log lines, want 0:\n%s", n, Read(t, log))
			}
		})
	}

	// No jq dependency: the hook is Go behind the bash launcher. These run the
	// real launcher against the freshly built binary, with a minimal PATH
	// holding `cat` and `dirname` but no `jq`, rather than an empty PATH.
	noJQ := func(t *testing.T, k *Kit) (bash, launcher string) {
		stubDir := filepath.Join(t.TempDir(), "stub_no_jq")
		Mkdir(t, stubDir)
		for _, tool := range []string{"cat", "dirname"} {
			p, err := exec.LookPath(tool)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(p, filepath.Join(stubDir, tool)); err != nil {
				t.Fatal(err)
			}
		}
		bash, err := exec.LookPath("bash")
		if err != nil {
			t.Fatal(err)
		}
		k.Setenv("PATH", stubDir)
		return bash, filepath.Join(Tree(t), "bin", "kit")
	}

	t.Run("a payload with none of the three substrings exits clean even without jq on PATH", func(t *testing.T) {
		t.Parallel()
		k, log := setup(t)
		bash, launcher := noJQ(t, k)
		r := k.exec(bash, `{"hook_event_name":"PostToolUse","tool_name":"Read","tool_input":{"file_path":"/tmp/foo.ts"}}`, launcher, "hook", "log-skills")
		r.Want(t, 0)
		r.Lacks(t, "jq not found")
		if n := lineCount(t, log); n != 0 {
			t.Errorf("%d log lines, want 0", n)
		}
	})

	t.Run("a loggable payload is logged even without jq on PATH", func(t *testing.T) {
		t.Parallel()
		k, log := setup(t)
		bash, launcher := noJQ(t, k)
		r := k.exec(bash, `{"hook_event_name":"PostToolUse","tool_name":"Skill","tool_input":{"skill":"bash-patterns"}}`, launcher, "hook", "log-skills")
		r.Want(t, 0)
		r.Empty(t)
		if n := lineCount(t, log); n != 1 {
			t.Errorf("%d log lines, want 1", n)
		}
	})
}
