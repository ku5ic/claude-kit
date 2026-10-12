package hooks

import (
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// log-skills logs three payload shapes only: a PostToolUse Skill-tool call,
// a PostToolUse Read of a .../skills/*/SKILL.md path, and a
// UserPromptExpansion of a slash_command. Its substring pre-filter is
// over-inclusive on purpose; the routing below it rejects the rest.
func TestLogSkills(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, payload string
		logged        []string // what the one logged line holds; nil logs nothing
	}{
		{"PostToolUse Skill-tool call is logged with skill_file from tool_input.skill",
			`{"hook_event_name":"PostToolUse","tool_name":"Skill","tool_input":{"skill":"bash-patterns"},"session_id":"s1","cwd":"/x"}`,
			[]string{`"skill_file":"bash-patterns"`, `"event":"PostToolUse"`}},
		{"PostToolUse Read of a SKILL.md path is logged with skill_file from the file path",
			`{"hook_event_name":"PostToolUse","tool_name":"Read","tool_input":{"file_path":"/Users/x/.claude/skills/bash-patterns/SKILL.md"},"session_id":"s1","cwd":"/x"}`,
			[]string{"skills/bash-patterns/SKILL.md"}},
		{"UserPromptExpansion with expansion_type slash_command is logged",
			`{"hook_event_name":"UserPromptExpansion","expansion_type":"slash_command","command_name":"flow-test","session_id":"s1","cwd":"/x"}`,
			[]string{`"command_name":"flow-test"`}},
		{"an ordinary Read of a non-SKILL.md file logs nothing",
			`{"hook_event_name":"PostToolUse","tool_name":"Read","tool_input":{"file_path":"/tmp/project/foo.ts"},"session_id":"s1","cwd":"/x"}`, nil},
		{"a PostToolUse call for an unrelated tool logs nothing",
			`{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"ls -la"},"session_id":"s1","cwd":"/x"}`, nil},
		{"UserPromptExpansion with a non-slash_command expansion_type logs nothing",
			`{"hook_event_name":"UserPromptExpansion","expansion_type":"keyword","command_name":"foo","session_id":"s1","cwd":"/x"}`, nil},
		{"an unrelated hook_event_name (PreToolUse) logs nothing",
			`{"hook_event_name":"PreToolUse","tool_name":"Edit","tool_input":{"file_path":"/tmp/foo.md"},"session_id":"s1","cwd":"/x"}`, nil},
		{"a payload that incidentally contains the substring 'Skill' but is not a loggable shape still logs nothing",
			`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo loading a Skill now"},"session_id":"s1","cwd":"/x"}`, nil},
		{"a slash-command-shaped command_name passes the pre-filter but a non-slash_command expansion_type is still rejected downstream",
			`{"hook_event_name":"UserPromptExpansion","expansion_type":"keyword","command_name":"/some-Skill-name","session_id":"s1","cwd":"/x"}`, nil},
		{"a payload with none of the three substrings exits clean",
			`{"hook_event_name":"PostToolUse","tool_name":"Read","tool_input":{"file_path":"/tmp/foo.ts"}}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			r := k.run("log-skills", tc.payload)
			r.Want(t, 0)
			r.Empty(t)
			log := read(t, k.paths.LogFile(kitlog.Skills))
			want := 0
			if tc.logged != nil {
				want = 1
			}
			if n := len(testutil.JSONLines(t, k.paths.LogFile(kitlog.Skills))); n != want {
				t.Fatalf("%d log lines, want %d:\n%s", n, want, log)
			}
			for _, s := range tc.logged {
				if !strings.Contains(log, s) {
					t.Errorf("log lacks %q:\n%s", s, log)
				}
			}
		})
	}
}
