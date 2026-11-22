package run

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
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
		// Lexically tree/outside; y is tree, so its .. is tree's parent.
		"a link climbing out through another link": func(string) [][2]string { return [][2]string{{"y", "."}, {"x", "y/../outside"}} },
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
	t.Run("a link up through a real directory is made", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "tree")
		if err := untar(archive(t, [2]string{"x", ""}, [2]string{"d/f", ""}, [2]string{"d/up", "../x"}), dir); err != nil {
			t.Fatal(err)
		}
	})
}

// Two runs on one base, a main-session Stop and a subagent's, share its
// tree: the second waits for the first's unpack instead of deleting it.
func TestUnpackWaitsForAnotherRunsUnpack(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Put(t, root, "a.txt", "a")
	testutil.Git(t, root, "init", "-q")
	testutil.Git(t, root, "add", ".")
	testutil.GitID(t, root, "commit", "-qm", "a")
	sha, err := git.Output(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	b := &Base{Root: root, SHA: strings.TrimSpace(sha), CacheDir: t.TempDir()}
	unlock, err := cache.Lock(t.Context(), b.dir()+".lock")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- b.unpack() }()
	select {
	case err := <-done:
		unlock()
		t.Fatalf("unpack ran while another run held the base (err %v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b.dir(), "a.txt")); err != nil {
		t.Error(err)
	}
}
