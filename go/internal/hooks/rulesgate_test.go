package hooks

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestRulesGate(t *testing.T) {
	t.Parallel()
	// loaded is a skills.jsonl line: kit-rules loaded in session s1, by the
	// Skill tool in the main session, or by agent through a Read.
	const mainLoad = `{"event":"PostToolUse","session_id":"s1","skill_file":"kit:kit-rules","tool_name":"Skill"}`
	const agentRead = `{"event":"PostToolUse","session_id":"s1","agent_id":"a1","skill_file":"/plugin/skills/kit-rules/SKILL.md","tool_name":"Read"}`
	setup := func(t *testing.T, log string) *sandbox {
		k := newPlugin(t)
		k.kitYML("")
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), log)
		return k
	}
	call := func(k *sandbox, tool, agent string, input map[string]any) result {
		p := map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "tool_name": tool, "tool_input": input}
		if agent != "" {
			p["agent_id"] = agent
		}
		return k.run("rules-gate", p)
	}
	bash := map[string]any{"command": "ls"}

	t.Run("a tool call before the load is denied, naming how to load it", func(t *testing.T) {
		t.Parallel()
		r := call(setup(t, ""), "Bash", "", bash)
		r.Denied(t)
		r.Has(t, "load the kit-rules skill first", "(rules-gate)")
	})
	t.Run("the Skill tool, a Read of the skill, and a hand-back pass before the load", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		call(k, "Skill", "", map[string]any{"skill": "kit:kit-rules"}).Allowed(t)
		call(k, "Read", "a1", map[string]any{"file_path": "/plugin/skills/kit-rules/reference/output.md"}).Allowed(t)
		call(k, "SubagentHandback", "a1", map[string]any{"message": "done"}).Allowed(t)
		call(k, "Read", "", map[string]any{"file_path": "/plugin/skills/go-patterns/SKILL.md"}).Denied(t)
	})
	t.Run("the main session's load lets it through, and not a subagent that didn't load it", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n")
		call(k, "Bash", "", bash).Allowed(t)
		call(k, "Bash", "a1", bash).Denied(t)
	})
	t.Run("a subagent with no Skill tool loads by Read, for itself alone", func(t *testing.T) {
		t.Parallel()
		k := setup(t, agentRead+"\n")
		call(k, "Grep", "a1", map[string]any{"pattern": "x"}).Allowed(t)
		call(k, "Bash", "", bash).Denied(t)
		call(k, "Bash", "a2", bash).Denied(t)
	})
	// started is a session transcript whose subagent a1 began with lines.
	started := func(t *testing.T, lines ...string) string {
		main := filepath.Join(t.TempDir(), "s1.jsonl")
		testutil.Write(t, main, "")
		testutil.Write(t, filepath.Join(strings.TrimSuffix(main, ".jsonl"), "subagents", "agent-a1.jsonl"), strings.Join(lines, "\n")+"\n")
		return main
	}
	inAgent := func(k *sandbox, main string) result {
		return k.run("rules-gate", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "agent_id": "a1",
			"agent_type": "web-fetch", "transcript_path": main, "tool_name": "WebFetch", "tool_input": map[string]any{"url": "https://example.com"}})
	}
	shape := func(tools ...string) string {
		var named []map[string]any
		for _, tool := range tools {
			named = append(named, map[string]any{"name": tool})
		}
		line, _ := json.Marshal(map[string]any{"type": "api-request-shape", "shape": map[string]any{"system": []any{}, "tools": named}})
		return string(line)
	}
	const prompt = `{"type":"user","message":{"role":"user","content":"Fetch it."}}`
	t.Run("an agent Claude Code gave neither Skill nor Read passes, logged", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		inAgent(k, started(t, prompt, shape("WebFetch", "SubagentHandback"))).Allowed(t)
		if log := testutil.Read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, "rules-gate-unloadable") {
			t.Errorf("guards log lacks the skip:\n%s", log)
		}
	})
	t.Run("an agent given Read or Skill is still gated", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		inAgent(k, started(t, prompt, shape("WebFetch", "Read"))).Denied(t)
		inAgent(k, started(t, prompt, shape("WebFetch", "Skill"))).Denied(t)
	})
	t.Run("an agent whose tools can't be read is gated", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		inAgent(k, started(t, prompt)).Denied(t)
		inAgent(k, filepath.Join(t.TempDir(), "none.jsonl")).Denied(t)
	})
	t.Run("a fork counts its parent's load", func(t *testing.T) {
		t.Parallel()
		fork := `{"type":"fork-context-ref","agentId":"a1"}`
		inAgent(setup(t, mainLoad+"\n"), started(t, fork, prompt)).Allowed(t)
		inAgent(setup(t, ""), started(t, fork, prompt)).Denied(t)
	})
	t.Run("no skills log fails open", func(t *testing.T) {
		t.Parallel()
		k := newPlugin(t)
		k.kitYML("")
		call(k, "Bash", "", bash).Allowed(t)
	})
	t.Run("every global_skills skill must load too, named in the same deny", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n")
		k.overlay("global_skills: [slop-patterns]\n")
		r := call(k, "Bash", "", bash)
		r.Denied(t)
		r.Has(t, "load the slop-patterns skill first")
		call(k, "Read", "", map[string]any{"file_path": "/plugin/skills/slop-patterns/SKILL.md"}).Allowed(t)
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), mainLoad+"\n"+`{"event":"PostToolUse","session_id":"s1","skill_file":"kit:slop-patterns","tool_name":"Skill"}`+"\n")
		call(k, "Bash", "", bash).Allowed(t)
	})
	t.Run("rules-gate in disabled_rules lets it through", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		k.overlay("disabled_rules: [rules-gate]\n")
		call(k, "Bash", "", bash).Allowed(t)
	})
}
