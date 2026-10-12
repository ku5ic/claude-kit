package status_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/status"
)

// subagent is SubagentStatusline's output for the payload wrapping tasks.
func subagent(tasks ...string) string {
	return subagentRaw(`{"session_id":"s1","columns":120,"tasks":[` + strings.Join(tasks, ",") + `]}`)
}

func subagentRaw(stdin string) string {
	var out bytes.Buffer
	status.SubagentStatusline(strings.NewReader(stdin), &out)
	return out.String()
}

// content decodes the content rendered for task id; "" when absent.
func content(t *testing.T, out, id string) string {
	t.Helper()
	for l := range strings.Lines(out) {
		var row struct{ ID, Content string }
		if err := json.Unmarshal([]byte(l), &row); err != nil {
			t.Fatalf("%v: %s", err, l)
		}
		if row.ID == id {
			return row.Content
		}
	}
	return ""
}

func lineCount(out string) int {
	return len(strings.Split(strings.TrimRight(out, "\n"), "\n"))
}

// Claude Code parses stdout as one {"id","content"} JSON object per line,
// so each case renders one task, t1, and checks its decoded content: equal
// to is, holding has, and without lacks, each skipped when empty. Cases
// cover the version-gated fields (model, contextWindowSize, effort) that
// older builds omit.
func TestSubagentStatuslineContent(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, task     string
		is, has, lacks string
	}{
		{
			name: "renders model when present",
			task: `{"id":"t1","name":"scout","status":"running","model":"claude-opus-4-8"}`,
			is:   "scout [running]  claude-opus-4-8",
		},
		{
			name:  "omits model when absent (pre-v2.1.205 payload)",
			task:  `{"id":"t1","name":"scout","status":"running"}`,
			lacks: "claude-",
		},
		{
			name: "renders effort when present",
			task: `{"id":"t1","name":"scout","status":"running","effort":"high"}`,
			has:  "effort:high",
		},
		{
			name:  "omits effort when absent (pre-v2.1.214 payload)",
			task:  `{"id":"t1","name":"scout","status":"running"}`,
			lacks: "effort:",
		},
		{
			name: "renders a numeric token-budget effort value verbatim",
			task: `{"id":"t1","name":"scout","status":"running","effort":12000}`,
			has:  "effort:12000",
		},
		{
			name: "renders context percentage from tokenCount over contextWindowSize",
			task: `{"id":"t1","name":"scout","status":"running","contextWindowSize":200000,"tokenCount":45000}`,
			has:  " 22%",
		},
		{
			name:  "omits context percentage when contextWindowSize is absent",
			task:  `{"id":"t1","name":"scout","status":"running","tokenCount":45000}`,
			lacks: "%",
		},
		{
			name:  "omits context percentage when tokenCount is absent",
			task:  `{"id":"t1","name":"scout","status":"running","contextWindowSize":200000}`,
			lacks: "%",
		},
		{
			name:  "omits context percentage when contextWindowSize is explicitly zero (no divide by zero)",
			task:  `{"id":"t1","name":"scout","status":"running","contextWindowSize":0,"tokenCount":5000}`,
			lacks: "%",
		},
		{
			name: "renders zero percent when tokenCount is zero",
			task: `{"id":"t1","name":"scout","status":"running","contextWindowSize":200000,"tokenCount":0}`,
			has:  " 0%",
		},
		{
			name: "full payload renders every segment in order",
			task: `{"id":"t1","name":"scout","status":"running","model":"claude-opus-4-8","contextWindowSize":200000,"tokenCount":45000,"effort":"high"}`,
			is:   "scout [running]  claude-opus-4-8  effort:high  22%",
		},
		{
			name: "falls back to label when name is absent",
			task: `{"id":"t1","label":"explore repo","status":"running"}`,
			is:   "explore repo [running]",
		},
		{
			name: "falls back to description when name and label are absent",
			task: `{"id":"t1","description":"find the bug","status":"running"}`,
			is:   "find the bug [running]",
		},
		{
			name: "falls back to a placeholder when name, label and description are all absent",
			task: `{"id":"t1","status":"running"}`,
			is:   "task [running]",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := content(t, subagent(c.task), "t1")
			if c.is != "" && got != c.is {
				t.Errorf("content %q, want %q", got, c.is)
			}
			if !strings.Contains(got, c.has) {
				t.Errorf("content %q lacks %q", got, c.has)
			}
			if c.lacks != "" && strings.Contains(got, c.lacks) {
				t.Errorf("content %q has %q", got, c.lacks)
			}
		})
	}
}

func TestSubagentStatuslineRows(t *testing.T) {
	t.Parallel()
	t.Run("emits one JSON object per task, keyed by task id", func(t *testing.T) {
		t.Parallel()
		out := subagent(
			`{"id":"t1","name":"scout","status":"running"}`,
			`{"id":"t2","name":"tester","status":"pending"}`)
		if n := lineCount(out); n != 2 {
			t.Errorf("%d lines, want 2:\n%s", n, out)
		}
		if got := content(t, out, "t1"); got != "scout [running]" {
			t.Errorf("t1 content %q", got)
		}
		if got := content(t, out, "t2"); got != "tester [pending]" {
			t.Errorf("t2 content %q", got)
		}
	})

	t.Run("every emitted line is JSON with a string id and string content", func(t *testing.T) {
		t.Parallel()
		out := subagent(`{"id":"t1","name":"scout","status":"running"}`)
		var row map[string]any
		if err := json.Unmarshal([]byte(out), &row); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		if _, ok := row["id"].(string); !ok {
			t.Errorf("id is not a string: %s", out)
		}
		if _, ok := row["content"].(string); !ok {
			t.Errorf("content is not a string: %s", out)
		}
	})

	t.Run("skips a task with no id, since the id keys the render", func(t *testing.T) {
		t.Parallel()
		out := subagent(
			`{"name":"orphan","status":"running"}`,
			`{"id":"t2","name":"tester","status":"running"}`)
		if n := lineCount(out); n != 1 {
			t.Errorf("%d lines, want 1:\n%s", n, out)
		}
		if got := content(t, out, "t2"); got != "tester [running]" {
			t.Errorf("t2 content %q", got)
		}
	})
}

// Claude Code discards stdout lines not shaped {"id","content"}, so a
// payload with nothing to render must print nothing, not a partial line.
func TestSubagentStatuslineSilent(t *testing.T) {
	t.Parallel()
	for name, stdin := range map[string]string{
		"empty task list produces no output":                                    `{"columns":120,"tasks":[]}`,
		"payload without a tasks key produces no output":                        `{"columns":120}`,
		"invalid JSON exits clean with empty stdout rather than a partial line": "not json",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if out := subagentRaw(stdin); out != "" {
				t.Errorf("want no output, got:\n%s", out)
			}
		})
	}
}
