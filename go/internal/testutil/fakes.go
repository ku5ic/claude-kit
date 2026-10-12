package testutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// FakeTool writes an executable at path that appends one line per run to
// calls, "<cwd>|<args>", then runs the shell in extra, whose exit status
// is the tool's.
func FakeTool(t *testing.T, path, calls, extra string) {
	t.Helper()
	Put(t, filepath.Dir(path), filepath.Base(path), fmt.Sprintf("#!/bin/sh\nprintf '%%s|%%s\\n' \"$PWD\" \"$*\" >>%q\n%s\n", calls, extra))
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// Envelope is a classifier reply, as Replayer replays it: entries'
// verdicts, managers, and proposals, each a list (empty when nil).
func Envelope(t *testing.T, entries, managers, proposals []map[string]any) string {
	t.Helper()
	orEmpty := func(m []map[string]any) []map[string]any {
		if m == nil {
			return []map[string]any{}
		}
		return m
	}
	data, err := json.Marshal(map[string]any{"is_error": false, "subtype": "success", "structured_output": map[string]any{
		"entries": orEmpty(entries), "managers": orEmpty(managers), "proposals": orEmpty(proposals)}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// FixerProposal is a classifier's proposed formatter: command, run on
// files matching globs, as evidence states it.
func FixerProposal(command, evidence string, globs ...string) map[string]any {
	return map[string]any{"role": "fixer", "command": command, "file_form": command + " {files}", "globs": globs, "dir": ".", "evidence": evidence}
}

// Replayer is a classifier command answering every request with the
// envelope recorded at path.
func Replayer(t *testing.T, path string) []string {
	t.Helper()
	stub := filepath.Join(t.TempDir(), "classifier")
	FakeTool(t, stub, filepath.Join(t.TempDir(), "calls"), fmt.Sprintf("cat >/dev/null\ncat %q", path))
	return []string{stub}
}

// Calls is every run a FakeTool recorded in calls, oldest first; none when
// it never ran.
func Calls(t *testing.T, calls string) []string {
	t.Helper()
	data, err := os.ReadFile(calls)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// Age sets each path's access and modification times to d ago; a negative
// d is the future.
func Age(t *testing.T, d time.Duration, paths ...string) {
	t.Helper()
	at := time.Now().Add(-d)
	for _, p := range paths {
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}
}

// AppendJSONL appends one line per value to path, creating it: a string as
// is, anything else as JSON.
func AppendJSONL(t *testing.T, path string, lines ...any) {
	t.Helper()
	var b strings.Builder
	for _, l := range lines {
		s, ok := l.(string)
		if !ok {
			raw, err := json.Marshal(l)
			if err != nil {
				t.Fatal(err)
			}
			s = string(raw)
		}
		b.WriteString(s + "\n")
	}
	if err := fsx.Append(path, []byte(b.String())); err != nil {
		t.Fatal(err)
	}
}

// UserPrompt is a transcript line for a prompt the user typed.
func UserPrompt(text string) map[string]any {
	return map[string]any{"type": "user", "message": map[string]any{"content": text}}
}

// AssistantText is a transcript line for an assistant reply's text.
func AssistantText(text string) map[string]any {
	return assistant(map[string]any{"type": "text", "text": text})
}

// ToolUse is a transcript line for an assistant tool call; an empty id is
// left out.
func ToolUse(id, name string, input map[string]any) map[string]any {
	block := map[string]any{"type": "tool_use", "name": name, "input": input}
	if id != "" {
		block["id"] = id
	}
	return assistant(block)
}

// ToolResult is a transcript line for call id's result; content is a string
// or a list of content blocks.
func ToolResult(id string, content any) map[string]any {
	return map[string]any{"type": "user", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_result", "tool_use_id": id, "content": content},
	}}}
}

func assistant(block map[string]any) map[string]any {
	return map[string]any{"type": "assistant", "message": map[string]any{"content": []any{block}}}
}
