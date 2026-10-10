package kitlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// line is a log line about as long as a hook writes.
func line(n int) []byte {
	return Line("ts", "2026-01-01T00:00:00Z", "hook", "guard-bash", "event", "block", "n", fmt.Sprint(n))
}

func lines(t *testing.T, path string) []string {
	t.Helper()
	var out []string
	if err := Each(path, func(l []byte) { out = append(out, strings.TrimSuffix(string(l), "\n")) }); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAppendTrimsToTheNewestPastATenth(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "logs", "guards.jsonl")
	for n := 1; n <= 11; n++ {
		if err := Append(path, line(n), 10); err != nil {
			t.Fatal(err)
		}
	}
	if got := lines(t, path); len(got) != 11 {
		t.Fatalf("within the margin: %d lines, want 11", len(got))
	}
	if err := Append(path, line(12), 10); err != nil {
		t.Fatal(err)
	}
	got := lines(t, path)
	if len(got) != 10 || got[0] != strings.TrimSpace(string(line(3))) || got[9] != strings.TrimSpace(string(line(12))) {
		t.Errorf("after the trim: %d lines, %q .. %q; want lines 3 to 12", len(got), got[0], got[len(got)-1])
	}
}

func TestAppendWaitsForATrimAndLandsAfterIt(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "skills.jsonl")
	if err := os.WriteFile(path, []byte("old\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	done := make(chan error)
	go func() { done <- Append(path, []byte("new\n"), 0) }()
	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("Append didn't wait for the exclusive lock: %v", err)
	default:
	}
	// The rewrite a trim does, while appenders wait.
	if _, err := f.WriteAt([]byte("kept\n"), 0); err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(5); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "kept\nnew\n" {
		t.Errorf("log = %q, want the appended line after the rewrite", got)
	}
}

func TestEachOnAMissingLogHasNoLines(t *testing.T) {
	t.Parallel()
	if got := lines(t, filepath.Join(t.TempDir(), "none.jsonl")); got != nil {
		t.Errorf("got %q", got)
	}
}

func TestLineEncodesEmptyAsNull(t *testing.T) {
	t.Parallel()
	if got := string(Line("a", "<b>", "c", "")); got != `{"a":"<b>","c":null}`+"\n" {
		t.Errorf("got %q", got)
	}
}
