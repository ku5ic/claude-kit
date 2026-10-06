package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// guardDispatchPayload is a Write-shaped payload (tool_input.content) with
// the fields every check the dispatcher runs reads.
func guardDispatchPayload(path, content, session, tool string) map[string]any {
	return map[string]any{
		"tool_input": map[string]any{"file_path": path, "content": content},
		"session_id": session,
		"tool_name":  tool,
	}
}

const guardDispatchSkillMap = `skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
`

func TestGuardDispatch(t *testing.T) {
	// guard-dispatch is the single PreToolUse hook wired to Edit|Write|
	// MultiEdit|Read. It runs guard-edit's and guard-skills' checks in
	// declared order (edit-safety, then skills-gate), each isolated so one
	// check's fail-open never masks a different check's genuine violation.
	// Each test fakes $HOME so the kit.yml/skills.jsonl/cache reads never
	// touch real machine state.
	sandbox := func(t *testing.T) *Kit {
		k := NewPlugin(t)
		k.Setenv("CLAUDE_GUARD_SKILLS", "1")
		return k
	}
	dispatch := func(k *Kit, path, content, tool string) Result {
		k.t.Helper()
		return k.Hook("guard-dispatch", guardDispatchPayload(path, content, "s1", tool))
	}
	skillsLog := func(k *Kit, body string) { Write(k.t, filepath.Join(k.Claude, "logs/skills.jsonl"), body) }

	t.Run("clean write with no kit.yml and no risky path passes both checks", func(t *testing.T) {
		dispatch(sandbox(t), "/tmp/project/notes.md", "A normal sentence with nothing wrong.", "Write").Want(t, 0)
	})

	t.Run("guard-edit's check fires first and blocks a lockfile edit", func(t *testing.T) {
		k := sandbox(t)
		// The guarded lockfile list comes from kit.yml.
		k.KitYML("extra_lockfiles: [package-lock.json]\n")
		r := dispatch(k, "/tmp/project/package-lock.json", "harmless content", "Write")
		r.Want(t, 2)
		r.Has(t, "Blocked by guard-edit:")
	})

	t.Run("guard-skills' check blocks when a required skill has not been loaded", func(t *testing.T) {
		k := sandbox(t)
		k.KitYML(guardDispatchSkillMap)
		skillsLog(k, "")
		r := dispatch(k, "/tmp/project/deploy.sh", "harmless content", "Write")
		r.Want(t, 2)
		r.Has(t, "bash-patterns")
	})

	t.Run("guard-skills' check is skipped unless CLAUDE_GUARD_SKILLS=1", func(t *testing.T) {
		k := sandbox(t)
		k.KitYML(guardDispatchSkillMap)
		skillsLog(k, "")
		k.Setenv("CLAUDE_GUARD_SKILLS", "0")
		dispatch(k, "/tmp/project/deploy.sh", "harmless content", "Write").Want(t, 0)
	})

	t.Run("ordering: a lockfile edit that would also trip the skills-gate surfaces only guard-edit's message", func(t *testing.T) {
		k := sandbox(t)
		k.KitYML(`extra_lockfiles: [yarn.lock]
skill_file_map:
  - on: basename
    globs: ["*.lock"]
    skills: [bash-patterns]
`)
		skillsLog(k, "")
		r := dispatch(k, "/tmp/project/yarn.lock", "harmless content", "Write")
		r.Want(t, 2)
		r.Has(t, "Blocked by guard-edit:")
		// The dispatcher exits at the first blocking check instead of running
		// the remaining one, so the skills-gate message never appears
		// alongside it.
		r.Lacks(t, "bash-patterns")
	})

	t.Run("a required skill already loaded this session allows a clean write through both checks", func(t *testing.T) {
		k := sandbox(t)
		k.KitYML(guardDispatchSkillMap)
		skillsLog(k, `{"ts":"2026-01-01T00:00:00Z","hook":"log-skills","event":"PreToolUse","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"bash-patterns","tool_name":"Skill"}`+"\n")
		dispatch(k, "/tmp/project/deploy.sh", "A perfectly ordinary comment.", "Write").Want(t, 0)
	})

	// Resilience: malformed JSON breaks every check's payload read
	// identically, so this cannot isolate one check's failure from another's
	// success, but it proves the dispatcher survives and fails open
	// end-to-end, printing a distinct "unexpected error, failing open" notice
	// per check along the way.
	t.Run("malformed JSON payload fails open through both checks instead of erroring out", func(t *testing.T) {
		r := sandbox(t).Hook("guard-dispatch", "not valid json")
		r.Want(t, 0)
		r.Has(t, "guard-edit: unexpected error, failing open", "guard-skills: unexpected error, failing open")
	})

	t.Run("each fail-open is logged to guards.jsonl, since its stderr reaches no one", func(t *testing.T) {
		k := sandbox(t)
		k.Hook("guard-dispatch", "not valid json").Want(t, 0)
		log := Read(t, filepath.Join(k.Claude, "logs/guards.jsonl"))
		for _, hook := range []string{"guard-edit", "guard-skills"} {
			if !strings.Contains(log, `"hook":"`+hook+`","event":"fail-open"`) {
				t.Errorf("no fail-open line for %s:\n%s", hook, log)
			}
		}
	})

	t.Run("empty stdin payload fails open cleanly", func(t *testing.T) {
		sandbox(t).Hook("guard-dispatch", "").Want(t, 0)
	})

	// Isolation (a check that panics or errors fails open without skipping
	// the next check) is covered in Go: go/internal/hook/hook_test.go.

	// Read: guard-edit's credential check applies; the skills gate does not.
	// readGuards is sensitive_paths plus a skill map that gates *.tsx and
	// *.sh edits.
	readGuards := func(t *testing.T) *Kit {
		k := sandbox(t)
		k.KitYML(`sensitive_paths: [".env", ".env.*", "~/.ssh/"]
extra_lockfiles: [package-lock.json]
skill_file_map:
  - on: basename
    globs: ["*.tsx", "*.sh"]
    skills: [react-patterns]
`)
		skillsLog(k, "")
		return k
	}

	t.Run("Read of .env is blocked", func(t *testing.T) {
		r := dispatch(readGuards(t), "/tmp/project/.env", "", "Read")
		r.Want(t, 2)
		r.Has(t, "reading a credential or key file")
	})

	t.Run("Read of .env.local is blocked", func(t *testing.T) {
		dispatch(readGuards(t), "/tmp/project/.env.local", "", "Read").Want(t, 2)
	})

	t.Run("Read under ~/.ssh is blocked", func(t *testing.T) {
		k := readGuards(t)
		dispatch(k, filepath.Join(k.Home, ".ssh/id_ed25519"), "", "Read").Want(t, 2)
	})

	t.Run("Write of .env is blocked", func(t *testing.T) {
		r := dispatch(readGuards(t), "/tmp/project/.env", "KEY=1", "Write")
		r.Want(t, 2)
		r.Has(t, "writing a credential or key file")
	})

	t.Run("Read of a .tsx passes with the skills gate on and no skills loaded", func(t *testing.T) {
		r := dispatch(readGuards(t), "/tmp/project/App.tsx", "", "Read")
		r.Want(t, 0)
		r.Empty(t)
	})

	t.Run("the same .tsx is still gated for an Edit", func(t *testing.T) {
		r := dispatch(readGuards(t), "/tmp/project/App.tsx", "x", "Edit")
		r.Want(t, 2)
		r.Has(t, "react-patterns")
	})

	t.Run("Read of a lockfile passes; the lockfile check guards writes only", func(t *testing.T) {
		dispatch(readGuards(t), "/tmp/project/package-lock.json", "", "Read").Want(t, 0)
	})

	t.Run("disabling sensitive-read lets the Read through", func(t *testing.T) {
		k := readGuards(t)
		k.Overlay("disabled_rules: [sensitive-read]\n")
		dispatch(k, "/tmp/project/.env", "", "Read").Want(t, 0)
	})
}
