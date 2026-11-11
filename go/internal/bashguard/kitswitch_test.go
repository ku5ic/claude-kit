package bashguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// A write to one of the kit's off switches asks through a symlink in
// either direction: a guarded name linked out to a dotfiles tree, and a
// path elsewhere linked into a guarded place.
func TestKitSwitchThroughSymlinks(t *testing.T) {
	t.Parallel()
	link := func(t *testing.T, target, name string) {
		t.Helper()
		testutil.Mkdir(t, filepath.Dir(name))
		if err := os.Symlink(target, name); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, want string
		setup      func(t *testing.T, k *sandbox, elsewhere string) string
	}{
		{"settings.json linked out to a dotfiles tree", "ask", func(t *testing.T, k *sandbox, elsewhere string) string {
			src := filepath.Join(elsewhere, "dotfiles/claude/settings.json")
			testutil.Touch(t, src)
			link(t, src, filepath.Join(k.claude, "settings.json"))
			return `echo '{}' > ~/.claude/settings.json`
		}},
		{"the logs directory linked out", "ask", func(t *testing.T, k *sandbox, elsewhere string) string {
			if err := os.RemoveAll(filepath.Join(k.claude, "logs")); err != nil {
				t.Fatal(err)
			}
			testutil.Mkdir(t, filepath.Join(elsewhere, "logs"))
			link(t, filepath.Join(elsewhere, "logs"), filepath.Join(k.claude, "logs"))
			return `echo x >> ~/.claude/logs/guards.jsonl`
		}},
		{"a path elsewhere linked into the logs", "ask", func(t *testing.T, k *sandbox, elsewhere string) string {
			target := filepath.Join(k.claude, "logs", "guards.jsonl")
			testutil.Touch(t, target)
			link(t, target, filepath.Join(elsewhere, "g"))
			return "echo x >> " + filepath.Join(elsewhere, "g")
		}},
		{"a path elsewhere linked into the overlay", "ask", func(t *testing.T, k *sandbox, elsewhere string) string {
			target := filepath.Join(k.claude, "claude-kit.local.yml")
			testutil.Touch(t, target)
			link(t, target, filepath.Join(elsewhere, "o.yml"))
			return "cp /tmp/x " + filepath.Join(elsewhere, "o.yml")
		}},
		{"an unrelated link", "pass", func(t *testing.T, _ *sandbox, elsewhere string) string {
			testutil.Touch(t, filepath.Join(elsewhere, "a"))
			link(t, filepath.Join(elsewhere, "a"), filepath.Join(elsewhere, "b"))
			return "echo x > " + filepath.Join(elsewhere, "b")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			cmd := tc.setup(t, k, testutil.Physical(t, t.TempDir()))
			r := k.hook("", "", cmd)
			got := "pass"
			if strings.Contains(r.Output, `"ask"`) {
				got = "ask"
			}
			if r.Status != 0 || got != tc.want {
				t.Errorf("status %d, got %s want %s: %s\n%s", r.Status, got, tc.want, cmd, r.Output)
			}
		})
	}
}
