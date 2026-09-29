package guard

import (
	"testing"

	"github.com/ku5ic/dotfiles/claude-kit/go/internal/config"
)

func TestGlobBashSemantics(t *testing.T) {
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

func TestIsSensitive(t *testing.T) {
	t.Setenv("HOME", "/h")
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
	}
	for path, want := range cases {
		if got := IsSensitive(cfg, path); got != want {
			t.Errorf("IsSensitive(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestIsRCFileAndLockfile(t *testing.T) {
	t.Setenv("HOME", "/h")
	cfg := &config.Config{
		RCFiles:         []string{"~/.zshrc", ".bashrc"},
		PackageManagers: []config.PackageManager{{Lockfile: "pnpm-lock.yaml"}, {Lockfile: "requirements.txt", HandEdited: true}},
		ExtraLockfiles:  []string{"Gemfile.lock"},
	}
	if !IsRCFile(cfg, "/h/.zshrc") || !IsRCFile(cfg, "~/.bashrc") || IsRCFile(cfg, "/p/.zshrc") {
		t.Error("IsRCFile")
	}
	if !IsGuardedLockfile(cfg, "/p/pnpm-lock.yaml") || !IsGuardedLockfile(cfg, "Gemfile.lock") || IsGuardedLockfile(cfg, "/p/requirements.txt") {
		t.Error("IsGuardedLockfile")
	}
}
