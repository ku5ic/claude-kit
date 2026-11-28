package explain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// Each case runs `kit explain` from a fresh repo on main and wants a
// guard's verdict line in stdout. HOME is the sandbox's, since guard-edit
// expands ~ from it, so no t.Parallel.
func TestExplain(t *testing.T) {
	home := testutil.Physical(t, t.TempDir())
	// $HOME elsewhere: explain reads only the home Run is given.
	t.Setenv("HOME", t.TempDir())
	kitYML, err := filepath.Abs("../../../kit.yml")
	if err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(home, ".claude")
	paths := config.Paths{Root: filepath.Dir(kitYML), ClaudeDir: claude, Base: kitYML, Overlay: filepath.Join(claude, "claude-kit.local.yml")}
	cfg := testutil.KitConfig(t)
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, repo, "init", "-q", "-b", "main")
	// A Grep for .env* blocks only when the walk finds one.
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("TOKEN=x\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"bash: an ordinary command passes", []string{"bash", "git status"}, "guard-bash: pass"},
		{"bash: a lone kit script is allowed", []string{"bash", "kit scratch-dir"}, "guard-bash: allow"},
		{"bash: a write to the given home's rc file blocks", []string{"bash", "echo x >> " + filepath.Join(home, ".zshrc")}, "guard-bash: block  rc-redirect"},
		{"bash: an AI co-author line blocks, as the hook does", []string{"bash", "git commit -m \"x\n\nCo-Authored-By: Claude <noreply@anthropic.com>\""}, "guard-attribution: block  ai-attribution"},
		{"edit: a credential read blocks", []string{"edit", filepath.Join(home, ".ssh/id_rsa"), "Read"}, "guard-edit: block  sensitive-read"},
		{"edit: a Grep of a credential dir blocks", []string{"edit", filepath.Join(home, ".ssh"), "Grep"}, "guard-edit: block  sensitive-read"},
		{"edit: a Grep of the repo is allowed with credential files excluded", []string{"edit", repo, "Grep"}, "guard-edit: allow"},
		{"edit: a Grep of the repo for .env files blocks", []string{"edit", repo, "Grep", ".env*"}, "guard-edit: block  sensitive-read"},
		{"edit: a plain write passes", []string{"edit", filepath.Join(repo, "docs/notes.md")}, "guard-edit: pass"},
		{"edit: a new report at the repo root asks", []string{"edit", filepath.Join(repo, "notes.md")}, "guard-edit: ask"},
		{"stop: a scratch file says scratch is why", []string{"stop", filepath.Join(repo, ".claude/scratch/probe.go")}, "unclaimed  " + filepath.Join(repo, ".claude/scratch/probe.go") + " (in scratch: no check or formatter runs there)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			if status := Run(paths, cfg, repo, home, "", c.args, &stdout, &stderr); status != 0 {
				t.Fatalf("status %d, want 0; stderr:\n%s", status, stderr.String())
			}
			if !strings.Contains(stdout.String(), c.want) {
				t.Errorf("stdout lacks %q:\n%s", c.want, stdout.String())
			}
		})
	}
}
