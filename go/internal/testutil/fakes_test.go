package testutil

import (
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestFakeToolRecordsEachRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tool, calls := filepath.Join(dir, "bin", "lint"), filepath.Join(dir, "calls")
	if got := Calls(t, calls); got != nil {
		t.Fatalf("before any run: %q", got)
	}
	FakeTool(t, tool, calls, "exit 3")
	cmd := exec.Command(tool, "--fix", "a b.go")
	cmd.Dir = dir
	err := cmd.Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 3 {
		t.Fatalf("exit = %v, want status 3", err)
	}
	if got, want := Calls(t, calls), []string{dir + "|--fix a b.go"}; !slices.Equal(got, want) {
		t.Errorf("calls = %q, want %q", got, want)
	}
}

func TestAppendJSONL(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sub", "t.jsonl")
	AppendJSONL(t, path, `{"raw":1}`)
	AppendJSONL(t, path, UserPrompt("hi"), ToolUse("", "Bash", map[string]any{"command": "ls"}))
	want := `{"raw":1}` + "\n" +
		`{"message":{"content":"hi"},"type":"user"}` + "\n" +
		`{"message":{"content":[{"input":{"command":"ls"},"name":"Bash","type":"tool_use"}]},"type":"assistant"}` + "\n"
	if got := Read(t, path); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
