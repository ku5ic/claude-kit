package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// `kit hook log-skills` wiring, through the real launcher with no jq on
// PATH: the hook is Go behind bin/kit, so a minimal PATH holding only `cat`
// and `dirname` still logs. Which payloads log is tested in process, in
// internal/hooks.
func TestLogSkills(t *testing.T) {
	t.Parallel()
	t.Run("a loggable payload is logged even without jq on PATH", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		stubDir := filepath.Join(t.TempDir(), "stub_no_jq")
		Mkdir(t, stubDir)
		for _, tool := range []string{"cat", "dirname"} {
			p, err := exec.LookPath(tool)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(p, filepath.Join(stubDir, tool)); err != nil {
				t.Fatal(err)
			}
		}
		bash, err := exec.LookPath("bash")
		if err != nil {
			t.Fatal(err)
		}
		k.Setenv("PATH", stubDir)
		launcher := filepath.Join(Tree(t), "bin", "kit")
		r := k.exec(bash, `{"hook_event_name":"PostToolUse","tool_name":"Skill","tool_input":{"skill":"bash-patterns"}}`, launcher, "hook", "log-skills")
		r.Want(t, 0)
		r.Empty(t)
		if n := len(Lines(Read(t, filepath.Join(k.Claude, "logs", "skills.jsonl")))); n != 1 {
			t.Errorf("%d log lines, want 1", n)
		}
	})
}
