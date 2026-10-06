package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

// Claude Code runs subagentStatusLine once per render with {columns, tasks:[...]}
// on stdin, and parses stdout as one {"id","content"} JSON object per line.
// Tests therefore assert on the decoded content field, not on raw stdout, and
// cover the version-gated fields (model, contextWindowSize, effort) that older
// builds omit.
func TestSubagentStatusline(t *testing.T) {
	k := New(t)
	// envelope wraps task objects in the payload Claude Code sends.
	envelope := func(tasks ...string) string {
		return `{"session_id":"s1","columns":120,"tasks":[` + strings.Join(tasks, ",") + `]}`
	}
	run := func(stdin string) Result { return k.Run(stdin, "subagent-statusline") }
	// content decodes the content rendered for one task; "" when absent.
	content := func(t *testing.T, r Result, id string) string {
		t.Helper()
		for _, l := range Lines(r.Stdout) {
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
	lineCount := func(r Result) int { return len(strings.Split(strings.TrimRight(r.Output, "\n"), "\n")) }
	is := func(t *testing.T, r Result, id, want string) {
		t.Helper()
		if got := content(t, r, id); got != want {
			t.Errorf("content %q, want %q", got, want)
		}
	}
	has := func(t *testing.T, r Result, id, sub string) {
		t.Helper()
		if got := content(t, r, id); !strings.Contains(got, sub) {
			t.Errorf("content %q lacks %q", got, sub)
		}
	}
	lacks := func(t *testing.T, r Result, id, sub string) {
		t.Helper()
		if got := content(t, r, id); strings.Contains(got, sub) {
			t.Errorf("content %q has %q", got, sub)
		}
	}

	t.Run("emits one JSON object per task, keyed by task id", func(t *testing.T) {
		r := run(envelope(
			`{"id":"t1","name":"scout","status":"running"}`,
			`{"id":"t2","name":"tester","status":"pending"}`))
		r.Want(t, 0)
		if n := lineCount(r); n != 2 {
			t.Errorf("%d lines, want 2:\n%s", n, r.Output)
		}
		is(t, r, "t1", "scout [running]")
		is(t, r, "t2", "tester [pending]")
	})

	t.Run("every emitted line is JSON with a string id and string content", func(t *testing.T) {
		r := run(envelope(`{"id":"t1","name":"scout","status":"running"}`))
		var row map[string]any
		if err := json.Unmarshal([]byte(r.Output), &row); err != nil {
			t.Fatalf("%v: %s", err, r.Output)
		}
		if _, ok := row["id"].(string); !ok {
			t.Errorf("id is not a string: %s", r.Output)
		}
		if _, ok := row["content"].(string); !ok {
			t.Errorf("content is not a string: %s", r.Output)
		}
	})

	t.Run("renders model when present", func(t *testing.T) {
		is(t, run(envelope(`{"id":"t1","name":"scout","status":"running","model":"claude-opus-4-8"}`)), "t1", "scout [running]  claude-opus-4-8")
	})

	t.Run("omits model when absent (pre-v2.1.205 payload)", func(t *testing.T) {
		lacks(t, run(envelope(`{"id":"t1","name":"scout","status":"running"}`)), "t1", "claude-")
	})

	t.Run("renders effort when present", func(t *testing.T) {
		has(t, run(envelope(`{"id":"t1","name":"scout","status":"running","effort":"high"}`)), "t1", "effort:high")
	})

	t.Run("omits effort when absent (pre-v2.1.214 payload)", func(t *testing.T) {
		lacks(t, run(envelope(`{"id":"t1","name":"scout","status":"running"}`)), "t1", "effort:")
	})

	t.Run("renders a numeric token-budget effort value verbatim", func(t *testing.T) {
		has(t, run(envelope(`{"id":"t1","name":"scout","status":"running","effort":12000}`)), "t1", "effort:12000")
	})

	t.Run("renders context percentage from tokenCount over contextWindowSize", func(t *testing.T) {
		has(t, run(envelope(`{"id":"t1","name":"scout","status":"running","contextWindowSize":200000,"tokenCount":45000}`)), "t1", " 22%")
	})

	t.Run("omits context percentage when contextWindowSize is absent", func(t *testing.T) {
		lacks(t, run(envelope(`{"id":"t1","name":"scout","status":"running","tokenCount":45000}`)), "t1", "%")
	})

	t.Run("omits context percentage when tokenCount is absent", func(t *testing.T) {
		lacks(t, run(envelope(`{"id":"t1","name":"scout","status":"running","contextWindowSize":200000}`)), "t1", "%")
	})

	t.Run("omits context percentage when contextWindowSize is explicitly zero (no divide by zero)", func(t *testing.T) {
		r := run(envelope(`{"id":"t1","name":"scout","status":"running","contextWindowSize":0,"tokenCount":5000}`))
		r.Want(t, 0)
		lacks(t, r, "t1", "%")
	})

	t.Run("renders zero percent when tokenCount is zero", func(t *testing.T) {
		has(t, run(envelope(`{"id":"t1","name":"scout","status":"running","contextWindowSize":200000,"tokenCount":0}`)), "t1", " 0%")
	})

	t.Run("full payload renders every segment in order", func(t *testing.T) {
		is(t, run(envelope(`{"id":"t1","name":"scout","status":"running","model":"claude-opus-4-8","contextWindowSize":200000,"tokenCount":45000,"effort":"high"}`)), "t1", "scout [running]  claude-opus-4-8  effort:high  22%")
	})

	t.Run("falls back to label when name is absent", func(t *testing.T) {
		is(t, run(envelope(`{"id":"t1","label":"explore repo","status":"running"}`)), "t1", "explore repo [running]")
	})

	t.Run("falls back to description when name and label are absent", func(t *testing.T) {
		is(t, run(envelope(`{"id":"t1","description":"find the bug","status":"running"}`)), "t1", "find the bug [running]")
	})

	t.Run("falls back to a placeholder when name, label and description are all absent", func(t *testing.T) {
		is(t, run(envelope(`{"id":"t1","status":"running"}`)), "t1", "task [running]")
	})

	t.Run("skips a task with no id, since the id keys the render", func(t *testing.T) {
		r := run(envelope(
			`{"name":"orphan","status":"running"}`,
			`{"id":"t2","name":"tester","status":"running"}`))
		r.Want(t, 0)
		if n := lineCount(r); n != 1 {
			t.Errorf("%d lines, want 1:\n%s", n, r.Output)
		}
		is(t, r, "t2", "tester [running]")
	})

	t.Run("empty task list produces no output", func(t *testing.T) {
		r := run(`{"columns":120,"tasks":[]}`)
		r.Want(t, 0)
		r.Empty(t)
	})

	t.Run("payload without a tasks key produces no output", func(t *testing.T) {
		r := run(`{"columns":120}`)
		r.Want(t, 0)
		r.Empty(t)
	})

	// Claude Code reads stdout and ignores stderr on a zero exit, so a parse
	// error is allowed to surface for anyone running this by hand; only stdout
	// has to stay empty.
	t.Run("invalid JSON exits clean with empty stdout rather than a partial line", func(t *testing.T) {
		r := run("not json")
		r.Want(t, 0)
		if r.Stdout != "" {
			t.Errorf("want empty stdout, got:\n%s", r.Stdout)
		}
	})

	t.Run("renders without jq on PATH", func(t *testing.T) {
		// Runs the built binary rather than bin/kit, which would pick the
		// installed one.
		k := New(t)
		k.Setenv("PATH", t.TempDir())
		r := k.Run(`{"columns":120,"tasks":[{"id":"t1","name":"scout"}]}`, "subagent-statusline")
		r.Want(t, 0)
		if got := strings.TrimRight(r.Output, "\n"); got != `{"id":"t1","content":"scout"}` {
			t.Errorf("output %q", got)
		}
	})
}
