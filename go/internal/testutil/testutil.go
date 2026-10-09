// Package testutil holds the helpers the internal packages' tests share.
// The config package's own tests can't use it: it imports config.
package testutil

import (
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
