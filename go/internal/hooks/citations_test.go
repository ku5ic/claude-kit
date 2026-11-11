package hooks

import (
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestCheckCitations(t *testing.T) {
	t.Parallel()
	project := func(t *testing.T) string {
		dir := repo(t, t.TempDir())
		testutil.Put(t, dir, "src/stop.go", "a\nb\nc\n")
		return dir
	}
	report := func(dir, event, agent, agentType, text string, retry bool) map[string]any {
		p := map[string]any{"session_id": "s", "cwd": dir, "agent_id": agent, "agent_type": agentType}
		if event == "SubagentStop" {
			p["hook_event_name"], p["last_assistant_message"], p["stop_hook_active"] = event, text, retry
		} else {
			p["hook_event_name"], p["tool_name"], p["tool_input"] = "PreToolUse", "SubagentHandback", map[string]any{"message": text}
		}
		return p
	}
	const good, bad = "- src/stop.go:2 returns early", "- src/stop.go:40 returns early"

	t.Run("a gatherer handback with a bad citation is sent back once", func(t *testing.T) {
		t.Parallel()
		k, dir := newSandbox(t), project(t)
		r := k.run("handback-dispatch", report(dir, "PreToolUse", "a1", "kit:gatherer", bad, false))
		r.Denied(t)
		r.Has(t, "src/stop.go:40", "the file has 3 lines", "(citations)")
		k.run("check-citations", report(dir, "PreToolUse", "a1", "kit:gatherer", bad, false)).Allowed(t)
		// The stop after a sent-back handback is let through too.
		k.run("check-citations", report(dir, "SubagentStop", "a1", "kit:gatherer", bad, false)).Allowed(t)
	})
	t.Run("a gatherer's closing message with a bad citation blocks, the second stop goes through", func(t *testing.T) {
		t.Parallel()
		k, dir := newSandbox(t), project(t)
		k.run("check-citations", report(dir, "SubagentStop", "a1", "kit:gatherer", bad, false)).Want(t, 2)
		k.run("check-citations", report(dir, "SubagentStop", "a1", "kit:gatherer", bad, true)).Want(t, 0)
	})
	for name, tc := range map[string]struct{ agentType, text string }{
		"good citations go through":           {"kit:gatherer", good},
		"another agent's report goes through": {"Explore", bad},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k, dir := newSandbox(t), project(t)
			k.run("check-citations", report(dir, "PreToolUse", "a1", tc.agentType, tc.text, false)).Allowed(t)
			k.run("check-citations", report(dir, "SubagentStop", "a1", tc.agentType, tc.text, false)).Want(t, 0)
		})
	}
	t.Run("a plan with a bad citation is sent back once per text", func(t *testing.T) {
		t.Parallel()
		k, dir := newSandbox(t), project(t)
		exit := func(text string) map[string]any {
			return map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "cwd": dir, "tool_name": "ExitPlanMode",
				"tool_input": map[string]any{"plan": text, "planFilePath": filepath.Join(dir, ".claude/plans/p.md")}}
		}
		r := k.run("check-citations", exit("# Plan\n\n"+bad))
		r.Denied(t)
		r.Has(t, "the plan p.md", "ExitPlanMode again")
		k.run("check-citations", exit("# Plan\n\n"+bad)).Allowed(t)
		k.run("check-citations", exit("# Plan v2\n\n"+bad)).Denied(t)
		k.run("check-citations", exit("# Plan\n\n"+good)).Allowed(t)
	})
	t.Run("disabled_rules: [citations] lets it through", func(t *testing.T) {
		t.Parallel()
		k, dir := newSandbox(t), project(t)
		testutil.Write(t, k.paths.Overlay, "disabled_rules: [citations]\n")
		k.run("check-citations", report(dir, "PreToolUse", "a1", "kit:gatherer", bad, false)).Allowed(t)
	})
}
