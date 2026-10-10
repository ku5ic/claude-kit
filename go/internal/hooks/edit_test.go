package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// overlaySandbox is a sandbox whose overlay link points at a file in a fake
// dotfiles tree; it returns the sandbox and the link's source.
func overlaySandbox(t *testing.T) (*sandbox, string) {
	t.Helper()
	k := newSandbox(t)
	src := filepath.Join(testutil.Physical(t, t.TempDir()), "dotfiles/claude/claude-kit.local.yml")
	testutil.Touch(t, src)
	if err := os.Symlink(src, k.paths.Overlay); err != nil {
		t.Fatal(err)
	}
	return k, src
}

func TestGuardEdit(t *testing.T) {
	t.Parallel()
	// A leading ~/ is the sandbox HOME.
	for _, tc := range []struct {
		name   string
		path   string
		status int
	}{
		{"allow: ts source file", "/tmp/test.ts", 0},
		{"allow: py source file", "/tmp/test.py", 0},
		{"allow: markdown doc", "/tmp/foo.md", 0},
		{"allow: nested project file", "/tmp/some/nested/dir/file.tsx", 0},
		{"allow: package.json (not a lockfile)", "/tmp/package.json", 0},
		{"block: package-lock.json", "/tmp/package-lock.json", 2},
		{"block: pnpm-lock.yaml", "/tmp/pnpm-lock.yaml", 2},
		{"block: yarn.lock", "/tmp/yarn.lock", 2},
		{"block: Gemfile.lock", "/tmp/Gemfile.lock", 2},
		{"block: Cargo.lock", "/tmp/Cargo.lock", 2},
		{"block: poetry.lock", "/tmp/poetry.lock", 2},
		{"block: uv.lock", "/tmp/uv.lock", 2},
		{"block: edit inside .git/", "/tmp/repo/.git/HEAD", 2},
		{"block: edit nested inside .git/", "/tmp/repo/.git/refs/heads/main", 2},
		{"block: ~/.zshrc", "~/.zshrc", 2},
		{"block: ~/.zprofile", "~/.zprofile", 2},
		{"block: ~/.bashrc", "~/.bashrc", 2},
		// Guarded lockfiles come from kit.yml's lockfile_globs.
		{"block: bun.lock", "/tmp/project/bun.lock", 2},
		{"block: Pipfile.lock", "/tmp/project/Pipfile.lock", 2},
		{"block: Cargo.lock", "/tmp/project/Cargo.lock", 2},
		{"allow: requirements.txt is hand-edited", "/tmp/project/requirements.txt", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			path := tc.path
			if rest, ok := strings.CutPrefix(path, "~/"); ok {
				path = filepath.Join(k.home, rest)
			}
			k.run("guard-edit", editPayload("Edit", path, "", "")).Want(t, tc.status)
		})
	}

	// The kit overlay: a write gets a prompt through either path to it.
	t.Run("ask: Write to the overlay through its ~/.claude link", func(t *testing.T) {
		t.Parallel()
		k, _ := overlaySandbox(t)
		r := k.run("guard-edit", editPayload("Edit", k.paths.Overlay, "", ""))
		r.Want(t, 0)
		r.Has(t, `"permissionDecision":"ask"`)
	})
	t.Run("ask: Write to the overlay's source file in the dotfiles tree", func(t *testing.T) {
		t.Parallel()
		k, src := overlaySandbox(t)
		r := k.run("guard-edit", editPayload("Edit", src, "", ""))
		r.Want(t, 0)
		r.Has(t, `"permissionDecision":"ask"`)
	})
	t.Run("allow: a same-named file that is not the overlay", func(t *testing.T) {
		t.Parallel()
		k, _ := overlaySandbox(t)
		r := k.run("guard-edit", editPayload("Edit", filepath.Join(t.TempDir(), "elsewhere/claude-kit.local.yml"), "", ""))
		r.Want(t, 0)
		r.Empty(t)
	})
	t.Run("allow: Read a CI workflow", func(t *testing.T) {
		t.Parallel()
		r := newSandbox(t).run("guard-edit", editPayload("Read", "/tmp/project/.github/workflows/ci.yml", "", ""))
		r.Want(t, 0)
		r.Empty(t)
	})
	t.Run("ask: Write a new report-like file at the repo root; scratch is its place", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		root := repo(t, filepath.Join(t.TempDir(), "repo"))
		testutil.Touch(t, filepath.Join(root, "existing.md"))
		r := k.run("guard-edit", editPayload("Write", filepath.Join(root, "report.md"), "", root))
		r.Want(t, 0)
		r.Has(t, `"permissionDecision":"ask"`, "kit scratch-dir")
		for _, path := range []string{"existing.md", "docs/guide.md", "main.go"} {
			k.run("guard-edit", editPayload("Write", filepath.Join(root, path), "", root)).Empty(t)
		}
		k.run("guard-edit", editPayload("Edit", filepath.Join(root, "notes.md"), "", root)).Empty(t)
	})
}

const bashSkillMap = `skill_file_map:
  - on: basename
    globs: ["*.sh"]
    skills: [bash-patterns]
`

// bashSkillLoaded is a skills.jsonl line recording bash-patterns loaded via
// the Skill tool in session s1.
const bashSkillLoaded = `{"ts":"2026-01-01T00:00:00Z","hook":"log-skills","event":"PreToolUse","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"bash-patterns","tool_name":"Skill"}`

// Not parallel: guard-dispatch runs the skills gate only with
// CLAUDE_GUARD_SKILLS=1 in the environment.
func TestGuardDispatch(t *testing.T) {
	t.Setenv("CLAUDE_GUARD_SKILLS", "1")
	// dispatch sends a Write-shaped payload (tool_input.content) with the
	// fields every check the dispatcher runs reads.
	dispatch := func(k *sandbox, path, content, tool string) result {
		k.t.Helper()
		return k.run("guard-dispatch", map[string]any{
			"tool_input": map[string]any{"file_path": path, "content": content},
			"session_id": "s1",
			"tool_name":  tool,
		})
	}
	skillsLog := func(k *sandbox, body string) { testutil.Write(k.t, k.paths.LogFile(kitlog.Skills), body) }

	t.Run("clean write with no kit.yml and no risky path passes both checks", func(t *testing.T) {
		dispatch(newPlugin(t), "/tmp/project/notes.md", "A normal sentence with nothing wrong.", "Write").Want(t, 0)
	})
	t.Run("guard-skills' check blocks when a required skill has not been loaded", func(t *testing.T) {
		k := newPlugin(t)
		k.kitYML(bashSkillMap)
		skillsLog(k, "")
		r := dispatch(k, "/tmp/project/deploy.sh", "harmless content", "Write")
		r.Want(t, 2)
		r.Has(t, "bash-patterns")
	})
	t.Run("guard-skills' check is skipped unless CLAUDE_GUARD_SKILLS=1", func(t *testing.T) {
		k := newPlugin(t)
		k.kitYML(bashSkillMap)
		skillsLog(k, "")
		t.Setenv("CLAUDE_GUARD_SKILLS", "0")
		dispatch(k, "/tmp/project/deploy.sh", "harmless content", "Write").Want(t, 0)
	})
	t.Run("ordering: a lockfile edit that would also trip the skills-gate surfaces only guard-edit's message", func(t *testing.T) {
		k := newPlugin(t)
		k.kitYML(`lockfile_globs: [yarn.lock]
skill_file_map:
  - on: basename
    globs: ["*.lock"]
    skills: [bash-patterns]
`)
		skillsLog(k, "")
		r := dispatch(k, "/tmp/project/yarn.lock", "harmless content", "Write")
		r.Want(t, 2)
		r.Has(t, "Blocked by guard-edit:")
		// The first block ends the run, so the skills gate never adds its message.
		r.Lacks(t, "bash-patterns")
	})
	t.Run("a required skill already loaded this session allows a clean write through both checks", func(t *testing.T) {
		k := newPlugin(t)
		k.kitYML(bashSkillMap)
		skillsLog(k, bashSkillLoaded+"\n")
		dispatch(k, "/tmp/project/deploy.sh", "A perfectly ordinary comment.", "Write").Want(t, 0)
	})

	// Read: guard-edit's credential check applies; the skills gate does not.
	// readGuards is sensitive_paths plus a skill map that gates *.tsx and
	// *.sh edits.
	readGuards := func(t *testing.T) *sandbox {
		k := newPlugin(t)
		k.kitYML(`sensitive_paths: [".env", ".env.*", "~/.ssh/"]
lockfile_globs: [package-lock.json]
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
		dispatch(k, filepath.Join(k.home, ".ssh/id_ed25519"), "", "Read").Want(t, 2)
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
		k.overlay("disabled_rules: [sensitive-read]\n")
		dispatch(k, "/tmp/project/.env", "", "Read").Want(t, 0)
	})
}

func TestGuardSkills(t *testing.T) {
	t.Parallel()
	// skillsSandbox reads kitYML, or the real kit.yml when it is "", with
	// skillsLog as the skills log.
	skillsSandbox := func(t *testing.T, kitYML, skillsLog string) *sandbox {
		var k *sandbox
		if kitYML == "" {
			k = newSandbox(t)
		} else {
			k = newPlugin(t)
			k.kitYML(kitYML)
		}
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), skillsLog)
		return k
	}
	guard := func(k *sandbox, path, session string) result {
		k.t.Helper()
		return k.run("guard-skills", editPayload("Edit", path, session, ""))
	}
	chmod := func(t *testing.T, path string, mode os.FileMode) {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("every skill_file_map entry in the real kit.yml blocks until its skill is loaded", func(t *testing.T) {
		t.Parallel()
		rules := testutil.KitConfig(t).SkillFileMap
		if len(rules) == 0 {
			t.Fatal("real kit.yml has no skill_file_map entries")
		}
		k := skillsSandbox(t, "", "")
		for _, rule := range rules {
			if len(rule.Globs) == 0 {
				t.Fatalf("skill_file_map entry with no globs: %+v", rule)
			}
			r := guard(k, "/tmp/project/"+strings.ReplaceAll(rule.Globs[0], "*", "sample"), "s1")
			r.Want(t, 2)
			r.Has(t, rule.Skills...)
		}
	})
	t.Run("declaration order: a .test.tsx file picks up test-patterns before typescript-patterns", func(t *testing.T) {
		t.Parallel()
		r := guard(skillsSandbox(t, "", ""), "/tmp/project/foo.test.tsx", "s1")
		r.Want(t, 2)
		// Position, not membership: a missing name counts as at the end.
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
	t.Run("cumulative matching: a .test.tsx file requires skills from every matching entry, not just one", func(t *testing.T) {
		t.Parallel()
		r := guard(skillsSandbox(t, "", ""), "/tmp/project/foo.test.tsx", "s1")
		r.Want(t, 2)
		r.Has(t, "test-patterns", "typescript-patterns", "react-patterns")
	})
	t.Run("only a .claude/scratch file skips the gate, not any dir named scratch", func(t *testing.T) {
		t.Parallel()
		k := skillsSandbox(t, bashSkillMap, "")
		guard(k, "/tmp/project/src/scratch/x.sh", "s1").Want(t, 2)
		guard(k, "/tmp/project/.claude/scratch/x.sh", "s1").Want(t, 0)
	})
	t.Run("disabled_rules: skills-gate turns the gate off", func(t *testing.T) {
		t.Parallel()
		guard(skillsSandbox(t, bashSkillMap+"disabled_rules: [skills-gate]\n", ""), "/tmp/project/x.sh", "s1").Want(t, 0)
	})
	// The real kit.yml has no catch-all row, so a synthetic map exercises
	// the composing itself.
	t.Run("a catch-all glob entry composes with a specific entry rather than displacing it", func(t *testing.T) {
		t.Parallel()
		k := skillsSandbox(t, `skill_file_map:
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
		t.Parallel()
		k := skillsSandbox(t, `skill_file_map:
  - on: path
    globs: ["*/widgets/*/CONFIG.md"]
    skills: [widget-patterns]
`, "")
		guard(k, "/tmp/random/CONFIG.md", "s1").Lacks(t, "widget-patterns")
		guard(k, "/tmp/project/widgets/foo/CONFIG.md", "s1").Has(t, "widget-patterns")
	})
	t.Run("an upper-case extension matches like its lower-case glob", func(t *testing.T) {
		t.Parallel()
		r := guard(skillsSandbox(t, bashSkillMap, ""), "/tmp/project/FOO.SH", "s1")
		r.Want(t, 2)
		r.Has(t, "bash-patterns")
	})
	t.Run("blocks when the required skill has not been loaded this session", func(t *testing.T) {
		t.Parallel()
		r := guard(skillsSandbox(t, bashSkillMap, ""), "/tmp/project/foo.sh", "s1")
		r.Want(t, 2)
		r.Has(t, "bash-patterns")
	})
	t.Run("Edit tool_name produces an edit-verb block message", func(t *testing.T) {
		t.Parallel()
		r := guard(skillsSandbox(t, bashSkillMap, ""), "/tmp/project/foo.sh", "s1")
		r.Want(t, 2)
		r.Has(t, "This edit touches")
	})

	for _, tc := range []struct {
		name, log, session string
		status             int
	}{
		{"allows when the required skill was loaded this session via the Skill tool", bashSkillLoaded, "s1", 0},
		{"allows a plugin-namespaced skill name (kit:bash-patterns)",
			strings.Replace(bashSkillLoaded, `"skill_file":"bash-patterns"`, `"skill_file":"kit:bash-patterns"`, 1), "s1", 0},
		{"allows when the required skill's SKILL.md was read this session (Read fallback)",
			`{"ts":"2026-01-01T00:00:00Z","hook":"log-skills","event":"PostToolUse","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"/Users/x/.claude/skills/bash-patterns/SKILL.md","tool_name":"Read"}`, "s1", 0},
		{"a session_id mismatch does not count as loaded",
			`{"ts":"2026-01-01T00:00:00Z","hook":"log-skills","event":"PreToolUse","session_id":"other-session","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"bash-patterns","tool_name":"Skill"}`, "s1", 2},
		{"an oversized log line doesn't switch the gate off",
			`{"command_name":"` + strings.Repeat("x", 5<<20) + `"}`, "s1", 2},
		{"an oversized log line doesn't hide a later load",
			`{"command_name":"` + strings.Repeat("x", 5<<20) + `"}` + "\n" + bashSkillLoaded, "s1", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			guard(skillsSandbox(t, bashSkillMap, tc.log+"\n"), "/tmp/project/foo.sh", tc.session).Want(t, tc.status)
		})
	}

	// A skill can be both surfaced by inject-context and required by
	// skill_file_map; a surfaced marker is not a load.
	for _, tc := range []struct{ name, glob, file, event, skill string }{
		{"a suggested-skill marker alone does not satisfy the required-skill check", "*.jsx", "foo.jsx", "suggested-skill", "react-patterns"},
		{"a required-skill marker alone does not satisfy the required-skill check", "*", "foo.txt", "required-skill", "fix-sizing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := skillsSandbox(t, "skill_file_map:\n  - on: basename\n    globs: [\""+tc.glob+"\"]\n    skills: ["+tc.skill+"]\n",
				`{"ts":"2026-01-01T00:00:00Z","hook":"inject-context","event":"`+tc.event+`","session_id":"s1","cwd":"/x","expansion_type":null,"command_name":null,"command_args":null,"command_source":null,"skill_file":"`+tc.skill+`","tool_name":null}`+"\n")
			r := guard(k, "/tmp/project/"+tc.file, "s1")
			r.Want(t, 2)
			r.Has(t, tc.skill)
		})
	}

	// The per-skill marker cache: <kit home>/cache/skills-loaded/<session>-<skill>.
	t.Run("an allowed session/skill pair writes a marker file to the cache", func(t *testing.T) {
		t.Parallel()
		k := skillsSandbox(t, bashSkillMap, bashSkillLoaded+"\n")
		guard(k, "/tmp/project/foo.sh", "s1").Want(t, 0)
		marker := filepath.Join(k.claude, "cache/skills-loaded/s1-bash-patterns")
		if info, err := os.Stat(marker); err != nil || !info.Mode().IsRegular() {
			t.Errorf("no marker file at %s: %v", marker, err)
		}
	})
	t.Run("a cached marker allows a second call even when the skills log becomes unreadable", func(t *testing.T) {
		t.Parallel()
		k := skillsSandbox(t, bashSkillMap, bashSkillLoaded+"\n")
		guard(k, "/tmp/project/foo.sh", "s1").Want(t, 0)
		log := k.paths.LogFile(kitlog.Skills)
		chmod(t, log, 0)
		r := guard(k, "/tmp/project/bar.sh", "s1")
		chmod(t, log, 0o644)
		r.Want(t, 0)
	})
	t.Run("a marker for one session does not satisfy a different session's check", func(t *testing.T) {
		t.Parallel()
		k := skillsSandbox(t, bashSkillMap, bashSkillLoaded+"\n")
		guard(k, "/tmp/project/foo.sh", "s1").Want(t, 0)
		guard(k, "/tmp/project/foo.sh", "s2").Want(t, 2)
	})
	// No skill-map cache stands between a kit.yml edit and its effect.
	t.Run("a kit.yml change takes effect on the next call", func(t *testing.T) {
		t.Parallel()
		k := skillsSandbox(t, bashSkillMap, "")
		r := guard(k, "/tmp/project/foo.sh", "s1")
		r.Want(t, 2)
		r.Has(t, "bash-patterns")
		k.kitYML(strings.ReplaceAll(bashSkillMap, "bash-patterns", "python-patterns"))
		r = guard(k, "/tmp/project/bar.sh", "s1")
		r.Want(t, 2)
		r.Has(t, "python-patterns")
		r.Lacks(t, "bash-patterns")
	})
}
