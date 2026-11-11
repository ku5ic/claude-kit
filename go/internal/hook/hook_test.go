package hook

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

func newHook(t *testing.T, payload string) (*Hook, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	h := &Hook{
		Name:    "test.sh",
		Payload: ParsePayload([]byte(payload)),
		Paths:   config.Paths{Home: t.TempDir()},
		Stdout:  &stdout,
		Stderr:  &stderr,
		Now:     func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) },
	}
	h.SetConfig(&config.Config{})
	return h, &stdout, &stderr
}

func blocker(h *Hook) error { return h.Block("nope", "some-rule") }

// The dispatcher contract: a check that panics or errors fails open with a
// notice under its own name, and the next check still runs.
func TestRunIsolatesAFailingCheck(t *testing.T) {
	for name, faulty := range map[string]Check{
		"panic": func(*Hook) error { panic("boom") },
		"error": func(*Hook) error { return errors.New("broken") },
	} {
		t.Run(name, func(t *testing.T) {
			h, _, stderr := newHook(t, `{"session_id":"s1"}`)
			status := Run(h, NamedCheck{"first.sh", faulty}, NamedCheck{"second.sh", blocker})
			if status != 2 {
				t.Errorf("status = %d, want 2 from the second check", status)
			}
			out := stderr.String()
			if !strings.Contains(out, "first.sh: unexpected error, failing open") || !strings.Contains(out, "kit second.sh: BLOCKED nope (some-rule)") {
				t.Errorf("stderr = %q", out)
			}
		})
	}
}

func TestRunStopsAtTheFirstBlock(t *testing.T) {
	h, _, _ := newHook(t, `{}`)
	ran := false
	status := Run(h, NamedCheck{"a.sh", blocker}, NamedCheck{"b.sh", func(*Hook) error { ran = true; return nil }})
	if status != 2 || ran {
		t.Errorf("status=%d, second ran=%v", status, ran)
	}
}

// A block after a notice prints as JSON so the notice still reaches the
// user: exit 2 would show Claude the reason and drop it.
func TestABlockKeepsTheNotice(t *testing.T) {
	h, stdout, stderr := newHook(t, `{}`)
	notifyThenBlock := func(h *Hook) error { h.Notify("checks: 0 passed, 1 failed, 0 skipped"); return blocker(h) }
	if status := Run(h, NamedCheck{"a.sh", notifyThenBlock}); status != 0 || stderr.Len() > 0 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	if got, want := stdout.String(), `{"decision":"block","reason":"kit a.sh: BLOCKED nope (some-rule)","systemMessage":"checks: 0 passed, 1 failed, 0 skipped"}`+"\n"; got != want {
		t.Errorf("stdout %s, want %s", got, want)
	}
}

func TestBlockLogsAndHonorsDisabledRules(t *testing.T) {
	h, _, _ := newHook(t, `{"session_id":"s1","cwd":"/nonexistent/proj","tool_input":{"command":"rm -rf /"}}`)
	h.SetContext("path  /x")
	err := h.Block("nope\nwhy   because", "some-rule")
	var b *Blocked
	if !errors.As(err, &b) || b.Reason != "kit test.sh: BLOCKED nope (some-rule)\nwhy   because\npath  /x" {
		t.Fatalf("err = %v", err)
	}
	h.SetConfig(&config.Config{DisabledRules: []string{"some-rule"}})
	if err := h.Block("nope", "some-rule"); err != nil {
		t.Errorf("disabled rule still blocked: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(h.Paths.LogDir(), "guards.jsonl"))
	// The block names a hash of the command, never the command, and the project.
	want := `{"ts":"2026-09-28T12:00:00Z","hook":"test.sh","event":"block","session_id":"s1","rule":"some-rule","target":"5c7923bd67b0","project":"proj"}` + "\n" +
		`{"ts":"2026-09-28T12:00:00Z","hook":"test.sh","event":"disabled","session_id":"s1","rule":"some-rule"}` + "\n"
	if string(data) != want {
		t.Errorf("guards.jsonl =\n%s\nwant\n%s", data, want)
	}
	if info, err := os.Stat(filepath.Join(h.Paths.LogDir(), "guards.jsonl")); err != nil {
		t.Error(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Errorf("guards.jsonl is %v, want 0600", info.Mode())
	}
}

func TestAFailOpenLogsItsError(t *testing.T) {
	h, _, stderr := newHook(t, `{"session_id":"s1"}`)
	RunCheck(h, "first.sh", func(*Hook) error { return errors.New("broken pipe") })
	if !strings.Contains(stderr.String(), "first.sh: unexpected error, failing open: broken pipe") {
		t.Errorf("stderr = %q", stderr)
	}
	data, _ := os.ReadFile(filepath.Join(h.Paths.LogDir(), "guards.jsonl"))
	if !strings.Contains(string(data), `"event":"fail-open","session_id":"s1","error":"broken pipe"`) {
		t.Errorf("guards.jsonl = %s", data)
	}
}

func TestLogEmptyValuesAreNull(t *testing.T) {
	h, _, _ := newHook(t, `{}`)
	h.Log("skills", "PostToolUse", "skill_file", "a<b>&", "tool_name", "")
	data, _ := os.ReadFile(filepath.Join(h.Paths.LogDir(), "skills.jsonl"))
	want := `{"ts":"2026-09-28T12:00:00Z","hook":"test.sh","event":"PostToolUse","session_id":null,"skill_file":"a<b>&","tool_name":null}` + "\n"
	if string(data) != want {
		t.Errorf("got %s", data)
	}
}

func TestDecide(t *testing.T) {
	decide := func(d, reason string) NamedCheck {
		return NamedCheck{"c", func(h *Hook) error { h.Decide(d, reason); return nil }}
	}
	tests := map[string]struct {
		checks []NamedCheck
		want   string
	}{
		"one ask": {[]NamedCheck{decide("ask", "confirm <this>")},
			`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"confirm <this>"}}` + "\n"},
		"ask outranks allow": {[]NamedCheck{decide("allow", "safe"), decide("ask", "confirm")},
			`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"confirm"}}` + "\n"},
		"asks join": {[]NamedCheck{decide("ask", "a"), decide("ask", "b")},
			`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"a; b"}}` + "\n"},
		"deny outranks ask": {[]NamedCheck{decide("ask", "confirm"), decide("deny", "load x"), decide("ask", "later")},
			`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"load x"}}` + "\n"},
		"context rides with a decision": {[]NamedCheck{
			{"n", func(h *Hook) error { h.AddContext("PreToolUse", "dense"); return nil }}, decide("ask", "confirm")},
			`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"confirm","additionalContext":"dense"}}` + "\n"},
		"none": {[]NamedCheck{{"c", func(*Hook) error { return nil }}}, ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			h, stdout, _ := newHook(t, `{}`)
			Run(h, tc.checks...)
			if stdout.String() != tc.want {
				t.Errorf("got %s", stdout)
			}
		})
	}
}

func TestPayload(t *testing.T) {
	p := ParsePayload([]byte(`{"tool_input":{"file_path":"/a","n":3}}`))
	if got := p.FilePath(); got != "/a" {
		t.Errorf("FilePath = %q", got)
	}
	if got := p.String("tool_input.n"); got != "" {
		t.Errorf("String of a number = %q, want empty", got)
	}
	nested := ParsePayload([]byte(`{"in":{"code":"x","b":"y","c":[{"code":"z","d":"w"}]}}`))
	if got := nested.Strings("in", "code"); !slices.Equal(got, []string{"y", "w"}) {
		t.Errorf("Strings skipping code = %q, want [y w]", got)
	}
	if ParsePayload([]byte("not json")).Err == nil || ParsePayload(nil).Err == nil {
		t.Error("bad payloads must set Err")
	}
}
