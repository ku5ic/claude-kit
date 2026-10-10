// Package testutil holds the helpers the internal packages' tests share.
// The config package's own tests can't use it: it imports config.
package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/git"
)

// KitConfig is the repo's kit.yml with no overlay, for a test in
// go/internal/<package>.
func KitConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, _, err := config.Load(config.Paths{Base: "../../../kit.yml", Overlay: filepath.Join(t.TempDir(), "none.yml")})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// Put writes body to dir/name, creating its directories.
func Put(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Fixtures maps each project under the testdata directory dir to the
// lockfiles Fixture writes into its copy, as dir/fixtures.json lists them:
// a lockfile is never hand-written in the repo.
func Fixtures(t *testing.T, dir string) map[string][]string {
	t.Helper()
	var fixtures map[string][]string
	if err := json.Unmarshal([]byte(Read(t, filepath.Join(dir, "fixtures.json"))), &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

// Fixture copies project name from dir into a fresh git repository, with
// its lockfiles, all staged (subprojects are found among tracked files),
// and returns the repository's root.
func Fixture(t *testing.T, dir, name string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join(dir, name))); err != nil {
		t.Fatal(err)
	}
	for _, lock := range Fixtures(t, dir)[name] {
		Put(t, root, lock, "")
	}
	Git(t, root, "init", "-q")
	Git(t, root, "add", "-A")
	return root
}

// Read is path's contents.
func Read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Git runs git in dir, failing the test on an error.
func Git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := git.Command(dir, args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
