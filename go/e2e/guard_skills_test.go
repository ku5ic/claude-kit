package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const guardSkillsBashMap = `skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
`

// `kit hook guard-skills` wiring: a block exits 2 under the hook's name and
// is logged like every other guard; a log or kit.yml it can't read fails
// open. The gate's matching and caching are tested in process, in
// internal/hooks.
func TestGuardSkills(t *testing.T) {
	t.Parallel()
	sandbox := func(t *testing.T, kitYML string) *Kit {
		k := NewPlugin(t)
		k.KitYML(kitYML)
		Write(t, filepath.Join(k.Claude, "logs/skills.jsonl"), "")
		return k
	}
	guard := func(k *Kit, path string) Result {
		k.t.Helper()
		return k.Hook("guard-skills", Payload("Edit", path, "s1", ""))
	}

	t.Run("a block names the hook and is logged like every other guard", func(t *testing.T) {
		k := sandbox(t, guardSkillsBashMap)
		r := guard(k, "/tmp/project/x.sh")
		r.Want(t, 2)
		r.Has(t, "Blocked by guard-skills:", "bash-patterns")
		if log := Read(t, filepath.Join(k.Claude, "logs/guards.jsonl")); !strings.Contains(log, `"rule":"skills-gate"`) {
			t.Errorf("guards.jsonl has no skills-gate block:\n%s", log)
		}
	})

	t.Run("an uncached skill still fails open when the skills log is unreadable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a chmod 000 file, so the log is never unreadable")
		}
		k := sandbox(t, guardSkillsBashMap)
		log := filepath.Join(k.Claude, "logs/skills.jsonl")
		if err := os.Chmod(log, 0); err != nil {
			t.Fatal(err)
		}
		r := guard(k, "/tmp/project/foo.sh")
		if err := os.Chmod(log, 0o644); err != nil {
			t.Fatal(err)
		}
		r.Want(t, 0)
	})

	t.Run("an unparsable kit.yml fails open", func(t *testing.T) {
		guard(sandbox(t, "not: [valid, yaml, skill_file_map"), "/tmp/project/foo.sh").Want(t, 0)
	})
}
