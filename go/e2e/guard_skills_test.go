package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

const guardSkillsBashMap = `skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
`

// guardSkillsLoaded is a skills.jsonl line recording bash-patterns loaded
// via the Skill tool in session s1.
const guardSkillsLoaded = `{"ts":"2026-01-01T00:00:00Z","hook":"log-skills","event":"PreToolUse","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"bash-patterns","tool_name":"Skill"}`

func TestGuardSkills(t *testing.T) {
	// guard-skills reads kit.yml (skill_file_map) and logs/skills.jsonl (what
	// has been loaded this session) on every Edit/Write/MultiEdit. Each test
	// fakes $HOME so real machine state never leaks into the assertions;
	// some copy the real kit.yml in so the production map itself is
	// exercised.
	sandbox := func(t *testing.T, kitYML, skillsLog string) *Kit {
		k := NewPlugin(t)
		k.KitYML(kitYML)
		Write(t, filepath.Join(k.Claude, "logs/skills.jsonl"), skillsLog)
		return k
	}
	realKitYML := func(t *testing.T) string { return Read(t, filepath.Join(kitRoot, "kit.yml")) }
	guard := func(k *Kit, path, session string) Result {
		k.t.Helper()
		return k.Hook("guard-skills", Payload("Edit", path, session, ""))
	}
	logPath := func(k *Kit) string { return filepath.Join(k.Claude, "logs/skills.jsonl") }
	chmod := func(t *testing.T, path string, mode os.FileMode) {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("every skill_file_map entry in the real kit.yml blocks until its skill is loaded", func(t *testing.T) {
		body := realKitYML(t)
		var cfg struct {
			SkillFileMap []struct {
				Globs  []string `yaml:"globs"`
				Skills []string `yaml:"skills"`
			} `yaml:"skill_file_map"`
		}
		if err := yaml.Unmarshal([]byte(body), &cfg); err != nil {
			t.Fatal(err)
		}
		if len(cfg.SkillFileMap) == 0 {
			t.Fatal("real kit.yml has no skill_file_map entries")
		}
		k := sandbox(t, body, "")
		for _, entry := range cfg.SkillFileMap {
			if len(entry.Globs) == 0 {
				t.Fatalf("skill_file_map entry with no globs: %+v", entry)
			}
			filename := strings.ReplaceAll(entry.Globs[0], "*", "sample")
			r := guard(k, "/tmp/project/"+filename, "s1")
			r.Want(t, 2)
			r.Has(t, entry.Skills...)
		}
	})

	t.Run("declaration order: a .test.tsx file picks up test-patterns before typescript-patterns", func(t *testing.T) {
		r := guard(sandbox(t, realKitYML(t), ""), "/tmp/project/foo.test.tsx", "s1")
		r.Want(t, 2)
		// Position, not membership: test-patterns must appear earlier in the
		// message than typescript-patterns, matching kit.yml's declared order.
		// A missing name sits at the end, as bats' ${output%%pat*} has it.
		test, ts := strings.Index(r.Output, "test-patterns"), strings.Index(r.Output, "typescript-patterns")
		if ts < 0 {
			ts = len(r.Output)
		}
		if test < 0 {
			test = len(r.Output)
		}
		if test >= ts {
			t.Errorf("test-patterns at %d, typescript-patterns at %d:\n%s", test, ts, r.Output)
		}
	})

	t.Run("a block names the hook and is logged like every other guard", func(t *testing.T) {
		k := sandbox(t, guardSkillsBashMap, "")
		r := guard(k, "/tmp/project/x.sh", "s1")
		r.Want(t, 2)
		r.Has(t, "Blocked by guard-skills:", "bash-patterns")
		if log := Read(t, filepath.Join(k.Claude, "logs/guards.jsonl")); !strings.Contains(log, `"rule":"skills-gate"`) {
			t.Errorf("guards.jsonl has no skills-gate block:\n%s", log)
		}
	})

	t.Run("only a .claude/scratch file skips the gate, not any dir named scratch", func(t *testing.T) {
		k := sandbox(t, guardSkillsBashMap, "")
		guard(k, "/tmp/project/src/scratch/x.sh", "s1").Want(t, 2)
		guard(k, "/tmp/project/.claude/scratch/x.sh", "s1").Want(t, 0)
	})

	t.Run("disabled_rules: skills-gate turns the gate off", func(t *testing.T) {
		k := sandbox(t, guardSkillsBashMap+"disabled_rules: [skills-gate]\n", "")
		guard(k, "/tmp/project/x.sh", "s1").Want(t, 0)
	})

	t.Run("cumulative matching: a .test.tsx file requires skills from every matching entry, not just one", func(t *testing.T) {
		r := guard(sandbox(t, realKitYML(t), ""), "/tmp/project/foo.test.tsx", "s1")
		r.Want(t, 2)
		r.Has(t, "test-patterns", "typescript-patterns", "react-patterns")
	})

	// kit.yml's skill_file_map no longer carries a catch-all globs: ["*"] row
	// (removed deliberately; those skills moved to rules/*.md), so this
	// exercises the composing mechanism itself via a synthetic map.
	t.Run("a catch-all glob entry composes with a specific entry rather than displacing it", func(t *testing.T) {
		k := sandbox(t, `skill_file_map:
  - on: basename
    globs: ["*"]
    skills: [fix-sizing]
  - on: basename
    globs: ["*.ts"]
    skills: [typescript-patterns]
`, "")
		r := guard(k, "/tmp/project/foo.ts", "s1")
		r.Want(t, 2)
		r.Has(t, "typescript-patterns", "fix-sizing")
	})

	t.Run("on: path entries match the full path, not just the basename", func(t *testing.T) {
		// Synthetic map: the real kit.yml has no on:path entries, but the
		// matching mode is still supported and needs coverage.
		k := sandbox(t, `skill_file_map:
  - on: path
    globs: ["*/widgets/*/CONFIG.md"]
    skills: [widget-patterns]
`, "")
		guard(k, "/tmp/random/CONFIG.md", "s1").Lacks(t, "widget-patterns")
		guard(k, "/tmp/project/widgets/foo/CONFIG.md", "s1").Has(t, "widget-patterns")
	})

	t.Run("blocks when the required skill has not been loaded this session", func(t *testing.T) {
		r := guard(sandbox(t, guardSkillsBashMap, ""), "/tmp/project/foo.sh", "s1")
		r.Want(t, 2)
		r.Has(t, "bash-patterns")
	})

	for _, tc := range []struct {
		name, log, session string
		status             int
	}{
		{"allows when the required skill was loaded this session via the Skill tool", guardSkillsLoaded, "s1", 0},
		{"allows a plugin-namespaced skill name (kit:bash-patterns)",
			strings.Replace(guardSkillsLoaded, `"skill_file":"bash-patterns"`, `"skill_file":"kit:bash-patterns"`, 1), "s1", 0},
		{"allows when the required skill's SKILL.md was read this session (Read fallback)",
			`{"ts":"2026-01-01T00:00:00Z","hook":"log-skills","event":"PostToolUse","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"/Users/x/.claude/skills/bash-patterns/SKILL.md","tool_name":"Read"}`, "s1", 0},
		{"a session_id mismatch does not count as loaded",
			`{"ts":"2026-01-01T00:00:00Z","hook":"log-skills","event":"PreToolUse","session_id":"other-session","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"bash-patterns","tool_name":"Skill"}`, "s1", 2},
		{"an oversized log line doesn't switch the gate off",
			`{"command_name":"` + strings.Repeat("x", 5<<20) + `"}`, "s1", 2},
		{"an oversized log line doesn't hide a later load",
			`{"command_name":"` + strings.Repeat("x", 5<<20) + `"}` + "\n" + guardSkillsLoaded, "s1", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			guard(sandbox(t, guardSkillsBashMap, tc.log+"\n"), "/tmp/project/foo.sh", tc.session).Want(t, tc.status)
		})
	}

	t.Run("Edit tool_name produces an edit-verb block message", func(t *testing.T) {
		r := guard(sandbox(t, guardSkillsBashMap, ""), "/tmp/project/foo.sh", "s1")
		r.Want(t, 2)
		r.Has(t, "This edit touches")
	})

	// Regression coverage for a fixed enforcement-floor gap: a skill can be
	// both stack-suggested (inject-context logs a "suggested-skill" marker,
	// meaning "surfaced", not "loaded") and skill_file_map-required. The
	// compliance query restricts to real invocation events (PreToolUse/
	// PostToolUse/UserPromptExpansion), so a bare "suggested-skill" or
	// "required-skill" marker can't satisfy a required-skill check on its own.
	t.Run("a suggested-skill marker alone does not satisfy the required-skill check", func(t *testing.T) {
		k := sandbox(t, `skill_file_map:
  - on: basename
    globs: ["*.jsx"]
    skills: [react-patterns]
`, `{"ts":"2026-01-01T00:00:00Z","hook":"inject-context","event":"suggested-skill","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"react-patterns","tool_name":null}`+"\n")
		r := guard(k, "/tmp/project/foo.jsx", "s1")
		r.Want(t, 2)
		r.Has(t, "react-patterns")
	})

	t.Run("a required-skill marker alone does not satisfy the required-skill check", func(t *testing.T) {
		k := sandbox(t, `skill_file_map:
  - on: basename
    globs: ["*"]
    skills: [fix-sizing]
`, `{"ts":"2026-01-01T00:00:00Z","hook":"inject-context","event":"required-skill","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"fix-sizing","tool_name":null}`+"\n")
		r := guard(k, "/tmp/project/foo.txt", "s1")
		r.Want(t, 2)
		r.Has(t, "fix-sizing")
	})

	// per-skill marker cache ($HOME/.claude/cache/skills-loaded/<session>-<skill>)

	t.Run("an allowed session/skill pair writes a marker file to the cache", func(t *testing.T) {
		k := sandbox(t, guardSkillsBashMap, guardSkillsLoaded+"\n")
		guard(k, "/tmp/project/foo.sh", "s1").Want(t, 0)
		marker := filepath.Join(k.Claude, "cache/skills-loaded/s1-bash-patterns")
		if info, err := os.Stat(marker); err != nil || !info.Mode().IsRegular() {
			t.Errorf("no marker file at %s: %v", marker, err)
		}
	})

	t.Run("a cached marker allows a second call even when the skills log becomes unreadable", func(t *testing.T) {
		k := sandbox(t, guardSkillsBashMap, guardSkillsLoaded+"\n")
		guard(k, "/tmp/project/foo.sh", "s1").Want(t, 0)
		// Simulate the log becoming unreadable after the marker was cached; a
		// fresh (uncached) skill for a session with no readable log would fail
		// open per the next test, but this session/skill pair should never
		// need to consult the log again at all.
		chmod(t, logPath(k), 0)
		r := guard(k, "/tmp/project/bar.sh", "s1")
		chmod(t, logPath(k), 0o644)
		r.Want(t, 0)
	})

	t.Run("an uncached skill still fails open when the skills log is unreadable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a chmod 000 file, so the log is never unreadable")
		}
		k := sandbox(t, guardSkillsBashMap, "")
		chmod(t, logPath(k), 0)
		r := guard(k, "/tmp/project/foo.sh", "s1")
		chmod(t, logPath(k), 0o644)
		r.Want(t, 0)
	})

	t.Run("a marker for one session does not satisfy a different session's check", func(t *testing.T) {
		k := sandbox(t, guardSkillsBashMap, guardSkillsLoaded+"\n")
		guard(k, "/tmp/project/foo.sh", "s1").Want(t, 0)
		guard(k, "/tmp/project/foo.sh", "s2").Want(t, 2)
	})

	// kit.yml is read on every call (the Go loader parses it in under a
	// millisecond), so no skill-map cache stands between an edit and its
	// effect.
	t.Run("a kit.yml change takes effect on the next call", func(t *testing.T) {
		k := sandbox(t, guardSkillsBashMap, "")
		r := guard(k, "/tmp/project/foo.sh", "s1")
		r.Want(t, 2)
		r.Has(t, "bash-patterns")
		k.KitYML(strings.ReplaceAll(guardSkillsBashMap, "bash-patterns", "python-patterns"))
		r = guard(k, "/tmp/project/bar.sh", "s1")
		r.Want(t, 2)
		r.Has(t, "python-patterns")
		r.Lacks(t, "bash-patterns")
	})

	t.Run("an unparsable kit.yml fails open", func(t *testing.T) {
		guard(sandbox(t, "not: [valid, yaml, skill_file_map", ""), "/tmp/project/foo.sh", "s1").Want(t, 0)
	})
}
