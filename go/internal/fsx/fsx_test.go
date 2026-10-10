package fsx

import (
	"os"
	"path/filepath"
	"testing"
)

func put(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindUpStopsAtStop(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, "marker"))
	put(t, filepath.Join(dir, "repo/sub/x"))
	if got := FindUp(filepath.Join(dir, "repo/sub"), filepath.Join(dir, "repo"), "marker"); got != "" {
		t.Errorf("found above stop: %q", got)
	}
	if got := FindUp(filepath.Join(dir, "repo/sub"), dir, "marker"); got != filepath.Join(dir, "marker") {
		t.Errorf("got %q", got)
	}
}

func TestWritesCreateTheDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	atomic, appended := filepath.Join(dir, "a/b/state.json"), filepath.Join(dir, "c/log.txt")
	if err := WriteAtomic(atomic, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(atomic, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Append(appended, []byte("x\n")); err != nil {
			t.Fatal(err)
		}
	}
	for path, want := range map[string]string{atomic: "two", appended: "x\nx\n"} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", path, got, err, want)
		}
	}
	if info, err := os.Stat(atomic); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, %v; want 0600", info.Mode(), err)
	}
}
