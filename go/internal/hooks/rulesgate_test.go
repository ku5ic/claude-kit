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
	// mainLoad is a skills.jsonl line: kit-rules loaded in session s1 by the
	// Skill tool in the main session.
	const mainLoad = `{"event":"PostToolUse","session_id":"s1","skill_file":"kit:kit-rules","tool_name":"Skill"}`
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
		r.Has(t, "the kit-rules skill isn't loaded yet", "rule  rules-gate")
	})
	t.Run("the Skill tool, a Read of the skill, and a hand-back pass before the load", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		call(k, "Skill", "", map[string]any{"skill": "kit:kit-rules"}).Allowed(t)
		call(k, "Read", "a1", map[string]any{"file_path": filepath.Join(k.paths.Root, "skills", "kit-rules", "reference", "output.md")}).Allowed(t)
		call(k, "SubagentHandback", "a1", map[string]any{"message": "done"}).Allowed(t)
		call(k, "Read", "", map[string]any{"file_path": filepath.Join(k.paths.Root, "skills", "go-patterns", "SKILL.md")}).Denied(t)
	})
	t.Run("a logged Read of a SKILL.md outside the skills dirs loads nothing", func(t *testing.T) {
		t.Parallel()
		k := setup(t, `{"event":"PostToolUse","session_id":"s1","skill_file":"/Users/foo/notes/skills/kit-rules/SKILL.md","tool_name":"Read"}`+"\n")
		call(k, "Bash", "", bash).Denied(t)
	})
	t.Run("a Read of a kit-rules folder outside the skills dirs is denied", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		call(k, "Read", "", map[string]any{"file_path": "/Users/foo/notes/skills/kit-rules/secret.txt"}).Denied(t)
		call(k, "Read", "", map[string]any{"file_path": filepath.Join(k.paths.Root, "skills", "kit-rules", "..", "..", "secret.txt")}).Denied(t)
	})
	t.Run("the main session's load lets it through, and not a subagent that didn't load it", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n")
		call(k, "Bash", "", bash).Allowed(t)
		call(k, "Bash", "a1", bash).Denied(t)
	})
	t.Run("a subagent with no Skill tool loads by Read, for itself alone", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), `{"event":"PostToolUse","session_id":"s1","agent_id":"a1","skill_file":"`+filepath.Join(k.paths.Root, "skills", "kit-rules", "SKILL.md")+`","tool_name":"Read"}`+"\n")
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
	t.Run("no session_id lets the call through, telling the user", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		r := k.run("rules-gate", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": bash})
		r.Allowed(t)
		r.Has(t, "kit rules-gate: FAILED OPEN")
	})
	t.Run("no skills log counts as nothing loaded", func(t *testing.T) {
		t.Parallel()
		k := newPlugin(t)
		k.kitYML("")
		r := call(k, "Bash", "", bash)
		r.Denied(t)
		r.Has(t, "the kit-rules skill isn't loaded yet")
	})
	t.Run("no skills log after a Skill call lets the call through, telling the user", func(t *testing.T) {
		t.Parallel()
		k := newPlugin(t)
		k.kitYML("")
		main := filepath.Join(t.TempDir(), "s1.jsonl")
		line, _ := json.Marshal(testutil.ToolUse("toolu_s1", "Skill", map[string]any{"skill": "kit:kit-rules"}))
		testutil.Write(t, main, string(line)+"\n")
		r := k.run("rules-gate", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "transcript_path": main, "tool_name": "Bash", "tool_input": bash})
		r.Allowed(t)
		r.Has(t, "kit rules-gate: FAILED OPEN", "the skills log is missing")
	})
	t.Run("a skills log that can't be read lets the call through, telling the user", func(t *testing.T) {
		t.Parallel()
		k := newPlugin(t)
		k.kitYML("")
		testutil.Mkdir(t, k.paths.LogFile(kitlog.Skills))
		r := call(k, "Bash", "", bash)
		r.Allowed(t)
		r.Has(t, "kit rules-gate: FAILED OPEN", "the skills log can't be read")
	})
	t.Run("every global_skills skill must load too, named in the same deny", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n")
		testutil.Write(t, filepath.Join(k.paths.Root, "skills", "slop-patterns", "SKILL.md"), "# Slop\n")
		k.overlay("global_skills: [slop-patterns]\n")
		r := call(k, "Bash", "", bash)
		r.Denied(t)
		r.Has(t, "the slop-patterns skill isn't loaded yet")
		call(k, "Read", "", map[string]any{"file_path": filepath.Join(k.paths.Root, "skills", "slop-patterns", "SKILL.md")}).Allowed(t)
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), mainLoad+"\n"+`{"event":"PostToolUse","session_id":"s1","skill_file":"kit:slop-patterns","tool_name":"Skill"}`+"\n")
		call(k, "Bash", "", bash).Allowed(t)
	})
	t.Run("a user skill is found in the Claude config dir, wherever HOME is", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n")
		// CLAUDE_CONFIG_DIR moves the config dir away from ~/.claude.
		k.paths.Root, k.home = t.TempDir(), t.TempDir()
		testutil.Write(t, filepath.Join(k.claude, "skills", "mine", "SKILL.md"), "# Mine\n")
		k.overlay("global_skills: [mine]\n")
		r := call(k, "Bash", "", bash)
		r.Denied(t)
		r.Has(t, "the mine skill isn't loaded yet")
	})
	t.Run("a global_skills skill with no SKILL.md is skipped, logged", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n")
		k.overlay("global_skills: [slop-pattern]\n")
		call(k, "Bash", "", bash).Allowed(t)
		if log := testutil.Read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, `"rules-gate-unknown-skill"`) {
			t.Errorf("guards log lacks the skip:\n%s", log)
		}
	})
	t.Run("a subagent's load of a mapped global skill stays its own once the main session reads a mapped file", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n"+`{"event":"PostToolUse","session_id":"s1","agent_id":"a1","skill_file":"kit:go-patterns","tool_name":"Skill"}`+"\n")
		testutil.Write(t, filepath.Join(k.paths.Root, "skills", "go-patterns", "SKILL.md"), "# Go\n")
		k.overlay("global_skills: [go-patterns]\nskill_file_map: [{on: basename, globs: [\"*.go\"], skills: [go-patterns]}]\n")
		call(k, "Bash", "", bash).Denied(t)
		k.run("guard-dispatch", map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "tool_name": "Read",
			"tool_input": map[string]any{"file_path": filepath.Join(t.TempDir(), "main.go")}}).Allowed(t)
		call(k, "Bash", "", bash).Denied(t)
		call(k, "Bash", "a1", bash).Denied(t) // kit-rules is still missing
	})
	start := func(k *sandbox, source string) {
		k.run("inject-context", map[string]any{"hook_event_name": "SessionStart", "session_id": "s1", "source": source, "cwd": k.home}).Want(k.t, 0)
	}
	reload := func(k *sandbox) {
		k.run("log-skills", map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "tool_name": "Skill", "tool_input": map[string]any{"skill": "kit:kit-rules"}}).Want(k.t, 0)
	}
	agentLoad := `{"event":"PostToolUse","session_id":"s1","agent_id":"a1","skill_file":"kit:kit-rules","tool_name":"Skill"}`
	t.Run("a compaction denies the main session until it loads every required skill again", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n"+`{"event":"PostToolUse","session_id":"s1","skill_file":"kit:slop-patterns","tool_name":"Skill"}`+"\n")
		testutil.Write(t, filepath.Join(k.paths.Root, "skills", "slop-patterns", "SKILL.md"), "# Slop\n")
		k.overlay("global_skills: [slop-patterns]\n")
		call(k, "Bash", "", bash).Allowed(t)
		start(k, "compact")
		r := call(k, "Bash", "", bash)
		r.Denied(t)
		r.Has(t, "the kit-rules and slop-patterns skills aren't loaded yet")
		reload(k)
		call(k, "Bash", "", bash).Has(t, "the slop-patterns skill isn't loaded yet")
		k.run("log-skills", map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "tool_name": "Skill", "tool_input": map[string]any{"skill": "kit:slop-patterns"}})
		call(k, "Bash", "", bash).Allowed(t)
	})
	t.Run("a compaction before any gated call still voids the load", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n")
		start(k, "compact")
		call(k, "Bash", "", bash).Denied(t)
		reload(k)
		call(k, "Bash", "", bash).Allowed(t)
	})
	t.Run("a second compaction denies again", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n")
		start(k, "compact")
		reload(k)
		call(k, "Bash", "", bash).Allowed(t)
		start(k, "compact")
		call(k, "Bash", "", bash).Denied(t)
	})
	t.Run("a startup, resume, or clear resets nothing", func(t *testing.T) {
		t.Parallel()
		for _, source := range []string{"startup", "resume", "clear"} {
			k := setup(t, mainLoad+"\n")
			start(k, source)
			call(k, "Bash", "", bash).Allowed(t)
		}
	})
	t.Run("another session's compaction resets nothing", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n")
		call(k, "Bash", "", bash).Allowed(t)
		k.run("inject-context", map[string]any{"hook_event_name": "SessionStart", "session_id": "s2", "source": "compact", "cwd": k.home})
		call(k, "Bash", "", bash).Allowed(t)
	})
	t.Run("the session's compaction leaves a subagent's load alone, marker or log", func(t *testing.T) {
		t.Parallel()
		k := setup(t, agentLoad+"\n")
		call(k, "Bash", "a1", bash).Allowed(t)
		start(k, "compact")
		call(k, "Bash", "a1", bash).Allowed(t)

		k = setup(t, agentLoad+"\n")
		start(k, "compact")
		call(k, "Bash", "a1", bash).Allowed(t)
	})
	t.Run("a fork started before the compaction keeps its own marker; one first calling after it counts the parent's loads since", func(t *testing.T) {
		t.Parallel()
		fork := `{"type":"fork-context-ref","agentId":"a1"}`
		k := setup(t, mainLoad+"\n")
		main := started(t, fork, prompt)
		inAgent(k, main).Allowed(t)
		start(k, "compact")
		inAgent(k, main).Allowed(t)

		k = setup(t, mainLoad+"\n")
		start(k, "compact")
		inAgent(k, main).Denied(t)
		reload(k)
		inAgent(k, main).Allowed(t)
	})
	t.Run("a log trimmed past the compaction line holds only what came after it", func(t *testing.T) {
		t.Parallel()
		k := setup(t, mainLoad+"\n")
		start(k, "compact")
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), "")
		call(k, "Bash", "", bash).Denied(t)
		reload(k)
		call(k, "Bash", "", bash).Allowed(t)
		testutil.Write(t, k.paths.LogFile(kitlog.Skills), "")
		call(k, "Bash", "", bash).Allowed(t) // by its marker
	})
	t.Run("rules-gate in disabled_rules lets it through", func(t *testing.T) {
		t.Parallel()
		k := setup(t, "")
		k.overlay("disabled_rules: [rules-gate]\n")
		call(k, "Bash", "", bash).Allowed(t)
	})
}
