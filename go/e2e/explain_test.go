package e2e

import (
	"path/filepath"
	"testing"
)

// `kit explain` wiring: the binary prints a guard's parse and decision with
// its rule, and logs, blocks, and runs nothing. The other verdicts are
// tested in process, in internal/explain; explain stop's cases are with the
// Stop hook's, in stop_checks_test.go.
func TestExplain(t *testing.T) {
	t.Parallel()
	k := New(t)
	k.Setenv("CLAUDE_CONFIG_DIR", k.Claude)
	repo := filepath.Join(t.TempDir(), "repo")
	Mkdir(t, repo)
	repo = Physical(t, repo)
	k.Git(repo, "init", "-q", "-b", "main")
	k.Dir = repo
	r := k.Run("", "explain", "bash", "cd /tmp && echo x | git push --force origin main")
	r.Want(t, 0)
	r.Has(t, `"echo" "x"  |  "git" "push" "--force" "origin" "main"`, "guard-bash: block  git-force-push")
	if Exists(filepath.Join(k.Claude, "logs/guards.jsonl")) {
		t.Error("explain wrote guards.jsonl")
	}
}
