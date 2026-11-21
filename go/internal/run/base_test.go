package run

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// archive is a tar of entries, each a symlink when link is set, else a
// file holding name.
func archive(t *testing.T, entries ...[2]string) *tar.Reader {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	for _, e := range entries {
		name, link := e[0], e[1]
		hdr := &tar.Header{Name: name, Mode: 0o644, Typeflag: tar.TypeReg, Size: int64(len(name))}
		if link != "" {
			hdr = &tar.Header{Name: name, Linkname: link, Typeflag: tar.TypeSymlink}
		}
		if err := w.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if link == "" {
			if _, err := w.Write([]byte(name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return tar.NewReader(&buf)
}

// A base commit may come from anyone, so its tree can't write outside the
// cache through a link it carries.
func TestUntarKeepsTheBaseTreeInsideItsDir(t *testing.T) {
	t.Parallel()
	for name, entries := range map[string]func(outside string) [][2]string{
		"a link to an absolute path":    func(outside string) [][2]string { return [][2]string{{"a", outside}, {"a/x", ""}} },
		"a link climbing out":           func(string) [][2]string { return [][2]string{{"a", "../outside"}, {"a/x", ""}} },
		"a file named outside the tree": func(string) [][2]string { return [][2]string{{"../outside/x", ""}} },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// outside sits beside the tree, so a red run writes nowhere else.
			base := t.TempDir()
			outside := filepath.Join(base, "outside")
			if err := os.Mkdir(outside, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := untar(archive(t, entries(outside)...), filepath.Join(base, "tree")); err == nil {
				t.Error("no error")
			}
			if left, _ := os.ReadDir(outside); len(left) != 0 {
				t.Errorf("wrote outside the tree: %v", left)
			}
		})
	}
	t.Run("a link inside the tree is made", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "tree")
		if err := untar(archive(t, [2]string{"x", ""}, [2]string{"y", "x"}), dir); err != nil {
			t.Fatal(err)
		}
		if got, err := os.Readlink(filepath.Join(dir, "y")); err != nil || got != "x" {
			t.Errorf("y -> %q, %v; want x", got, err)
		}
	})
}
