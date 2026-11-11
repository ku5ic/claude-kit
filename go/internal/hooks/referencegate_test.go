package hooks

import (
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestReferenceGate(t *testing.T) {
	t.Parallel()
	const table = "# Demo\n\nBefore each action below, read its file.\n\n| Before | Read |\n| --- | --- |\n" +
		"| Writing | [reference/one.md](reference/one.md), [reference/two.md](reference/two.md) |\n" +
		"| Reviewing | [reference/one.md](reference/one.md) |\n\nSee also [reference/outside.md](reference/outside.md).\n"
	setup := func(t *testing.T, log string) *sandbox {
		k := newPlugin(t)
		k.kitYML("")
		for name, body := range map[string]string{
			"demo/SKILL.md":             table,
			"demo/reference/one.md":     "one",
			"demo/reference/two.md":     "two",
			"demo/reference/outside.md": "outside",
			"steps/SKILL.md":            "# Steps\n\n1. Read [reference/a.md](reference/a.md).\n",
			"steps/reference/a.md":      "a",
		} {
			testutil.Write(t, filepath.Join(k.paths.Root, "skills", name), body)
		}
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), log)
		return k
	}
	line := func(agent, field, value string) string {
		return `{"event":"PostToolUse","session_id":"s1","agent_id":"` + agent + `","` + field + `":"` + value + `"}` + "\n"
	}
	loads := func(agent string) string { return line(agent, "skill_file", "kit:demo") }
	reads := func(k *sandbox, agent, ref string) string {
		return line(agent, "skill_file", filepath.Join(k.paths.Root, "skills", "demo", "reference", ref))
	}
	const compacted = `{"event":"compacted","session_id":"s1"}` + "\n"
	call := func(k *sandbox, tool, agent string, input map[string]any) result {
		p := map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "tool_name": tool, "tool_input": input}
		if agent != "" {
			p["agent_id"] = agent
		}
		return k.run("reference-gate", p)
	}
	ls := map[string]any{"command": "ls"}

	t.Run("no skill loaded asks for nothing", func(t *testing.T) {
		t.Parallel()
		call(setup(t, ""), "Bash", "", ls).Allowed(t)
	})
	t.Run("a loaded skill's table references are all named in one deny", func(t *testing.T) {
		t.Parallel()
		r := call(setup(t, loads("")), "Bash", "", ls)
		r.Denied(t)
		r.Has(t, "reference/one.md", "reference/two.md", "(reference-gate)")
		r.Lacks(t, "outside.md")
	})
	t.Run("reading them lets the call through; a deny names only what's unread", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), loads("")+reads(k, "", "one.md"))
		r := call(k, "Bash", "", ls)
		r.Denied(t)
		r.Lacks(t, "one.md")
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), loads("")+reads(k, "", "one.md")+reads(k, "", "two.md"))
		call(k, "Bash", "", ls).Allowed(t)
	})
	t.Run("a Read of the SKILL.md and a typed /command load the skill too", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), line("", "skill_file", filepath.Join(k.paths.Root, "skills", "demo", "SKILL.md")))
		call(k, "Bash", "", ls).Denied(t)
		call(setup(t, line("", "command_name", "/kit:demo")), "Bash", "", ls).Denied(t)
	})
	t.Run("a procedure skill, another plugin's skill, and a surfaced skill ask for nothing", func(t *testing.T) {
		t.Parallel()
		call(setup(t, line("", "skill_file", "kit:steps")), "Bash", "", ls).Allowed(t)
		call(setup(t, line("", "skill_file", "other:demo")), "Bash", "", ls).Allowed(t)
		call(setup(t, `{"event":"required-skill","session_id":"s1","skill_file":"demo"}`+"\n"), "Bash", "", ls).Allowed(t)
	})
	t.Run("skill files can always be read, and Skill and a hand-back pass", func(t *testing.T) {
		t.Parallel()
		k := setup(t, loads(""))
		call(k, "Read", "", map[string]any{"file_path": filepath.Join(k.paths.Root, "skills", "demo", "reference", "one.md")}).Allowed(t)
		call(k, "Skill", "", map[string]any{"skill": "kit:demo"}).Allowed(t)
		call(k, "SubagentHandback", "", map[string]any{"message": "done"}).Allowed(t)
		call(k, "Read", "", map[string]any{"file_path": "/repo/main.go"}).Denied(t)
	})
	t.Run("a subagent's loads and reads count for it alone", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), loads("a1")+loads("")+reads(k, "a1", "one.md")+reads(k, "a1", "two.md"))
		call(k, "Bash", "a1", ls).Allowed(t)
		call(k, "Bash", "", ls).Denied(t)
		call(k, "Bash", "a2", ls).Allowed(t)
	})
	t.Run("a compaction forgets every read before it, not the load", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), loads("")+reads(k, "", "one.md")+reads(k, "", "two.md")+compacted)
		call(k, "Bash", "", ls).Denied(t)
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), loads("")+compacted+reads(k, "", "one.md")+reads(k, "", "two.md"))
		call(k, "Bash", "", ls).Allowed(t)
	})
	t.Run("session start after a compaction logs the line the gate reads", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), loads("")+reads(k, "", "one.md")+reads(k, "", "two.md"))
		call(k, "Bash", "", ls).Allowed(t)
		k.run("inject-context", map[string]any{"hook_event_name": "SessionStart", "session_id": "s1", "source": "compact", "cwd": t.TempDir()})
		call(k, "Bash", "", ls).Denied(t)
	})
	t.Run("no skills log fails open", func(t *testing.T) {
		t.Parallel()
		k := newPlugin(t)
		k.kitYML("")
		call(k, "Bash", "", ls).Allowed(t)
	})
	t.Run("reference-gate in disabled_rules lets it through", func(t *testing.T) {
		t.Parallel()
		k := setup(t, loads(""))
		k.overlay("disabled_rules: [reference-gate]\n")
		call(k, "Bash", "", ls).Allowed(t)
	})
}
