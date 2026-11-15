package guard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

func TestGlobBashSemantics(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"*.ts", "a.ts", true},
		{"*.ts", "dir/a.ts", true}, // * crosses / in [[ == ]]
		{"*.test.*", "a.test.tsx", true},
		{"test_*.py", "test_x.py", true},
		{".env.*", ".env.local", true},
		{".env.*", ".env", false},
		{"?.go", "a.go", true},
		{"[ab].go", "b.go", true},
		{"[!ab].go", "c.go", true},
		{"[!ab].go", "a.go", false},
		{"[]x].go", "].go", true},
		{`\*.go`, "*.go", true},
		{`\*.go`, "a.go", false},
		{"a.(b)", "a.(b)", true},
		{"id_*", "id_ed25519", true},
	}
	for _, c := range cases {
		if got := Glob(c.pattern, c.s); got != c.want {
			t.Errorf("Glob(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}

func TestGlobConcurrent(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() { Glob(fmt.Sprintf("*.%d", i), "a.0") })
	}
	wg.Wait()
}

func TestIsSensitive(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{SensitivePaths: []string{"~/.ssh/", "~/.netrc", ".env", ".env.*", "id_*"}}
	cases := map[string]bool{
		"/h/.ssh/id_rsa":          true,
		`"$HOME/.ssh/config"`:     true,
		"~/.netrc":                true,
		"${HOME}/.netrc":          true,
		"/h/.netrc.bak":           false,
		"/p/.env":                 true,
		"/p/.env.production":      true,
		"/p/env.ts":               false,
		"/p/keys/id_ed25519":      true,
		"'/p/.env'":               true,
		"/other/.ssh/known_hosts": false,
		"/h/.aws/../.netrc":       true,
		"/h//.ssh/id_rsa":         true,
		"/h/.ssh":                 true,
		"/h/.sshx":                false,
		"/p/.ENV":                 true,
		"/p/.Env.Production":      true,
		"/p/keys/ID_ED25519":      true,
	}
	for path, want := range cases {
		if got := IsSensitive(cfg, "/h", path); got != want {
			t.Errorf("IsSensitive(%q) = %v, want %v", path, got, want)
		}
	}
}

// A ~/ entry folds case only where the home filesystem does, as on macOS.
// Not parallel: it sets foldHome, which the parallel tests read.
func TestIsSensitiveFoldsHomeOnlyWhereTheFilesystemDoes(t *testing.T) {
	cfg := &config.Config{SensitivePaths: []string{"~/.ssh/", "~/.netrc"}}
	saved := foldHome
	t.Cleanup(func() { foldHome = saved })
	for _, fold := range []bool{true, false} {
		foldHome = fold
		for _, path := range []string{"/h/.SSH/config", "~/.NETRC", "/H/.ssh/id_rsa"} {
			if got := IsSensitive(cfg, "/h", path); got != fold {
				t.Errorf("foldHome %v: IsSensitive(%q) = %v", fold, path, got)
			}
		}
	}
}

// rg itself skips a credential file of any case under the exclusion.
func TestSensitiveExclusionSkipsAnyCase(t *testing.T) {
	t.Parallel()
	rg, err := exec.LookPath("rg")
	if err != nil {
		t.Skip("rg isn't on PATH") // kit: allow the exclusion is only ever passed to rg
	}
	dir := t.TempDir()
	for _, name := range []string{"prod.PEM", "a.Key", ".ENV", ".env.Local", "ID_RSA", "ok.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("foo\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{SensitivePaths: []string{".env", ".env.*", "*.pem", "*.key", "id_rsa", "~/.ssh/"}}
	out, err := exec.Command(rg, "-l", "--hidden", "-g", SensitiveExclusion(cfg), "foo", dir).Output()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, line := range strings.Fields(string(out)) {
		got = append(got, filepath.Base(line))
	}
	if !slices.Equal(got, []string{"ok.txt"}) {
		t.Errorf("rg -g %s found %q, want only ok.txt", SensitiveExclusion(cfg), got)
	}
}

func TestIsRCFileAndLockfile(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		RCFiles:       []string{"~/.zshrc", ".bashrc"},
		LockfileGlobs: []string{"pnpm-lock.yaml", "Gemfile.lock"},
	}
	if !IsRCFile(cfg, "/h", "/h/.zshrc") || !IsRCFile(cfg, "/h", "~/.bashrc") || IsRCFile(cfg, "/h", "/p/.zshrc") {
		t.Error("IsRCFile")
	}
	if !IsGuardedLockfile(cfg, "/p/pnpm-lock.yaml") || !IsGuardedLockfile(cfg, "Gemfile.lock") || IsGuardedLockfile(cfg, "/p/requirements.txt") {
		t.Error("IsGuardedLockfile")
	}
}
