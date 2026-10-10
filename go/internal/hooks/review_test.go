package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// reviewEnv is one review-checks case: a git repo whose package.json has a
// lint script, a recording npm first on PATH, and the reviewer's transcript.
type reviewEnv struct {
	k                          *sandbox
	project, calls, transcript string
}

// Not parallel: each case puts its recording npm on PATH, to show the hook
// never runs the suite itself.
func TestReviewChecks(t *testing.T) {
	// setup writes the fork's .forked-skill.json sidecar unless it is "".
	setup := func(t *testing.T, sidecar string) reviewEnv {
		tmp := t.TempDir()
		e := reviewEnv{k: newSandbox(t), project: filepath.Join(tmp, "project"), calls: filepath.Join(tmp, "npm.calls"),
			transcript: filepath.Join(tmp, "agent-x.jsonl")}
		testutil.Mkdir(t, e.project)
		testutil.Git(t, e.project, "init", "-q", "-b", "main")
		testutil.Write(t, filepath.Join(e.project, "package.json"), `{"scripts":{"lint":"eslint ."}}`+"\n")
		testutil.FakeTool(t, filepath.Join(tmp, "stubs", "npm"), e.calls, "")
		t.Setenv("PATH", filepath.Join(tmp, "stubs")+":"+os.Getenv("PATH"))
		testutil.Write(t, e.transcript, "")
		if sidecar != "" {
			testutil.Write(t, filepath.Join(tmp, "agent-x.forked-skill.json"), sidecar)
		}
		return e
	}
	// stop is the SubagentStop payload; a parent transcript is sent when set.
	stop := func(e reviewEnv, cwd, parent string) result {
		p := map[string]any{"hook_event_name": "SubagentStop", "agent_type": "general-purpose",
			"agent_transcript_path": e.transcript, "stop_hook_active": false, "cwd": cwd}
		if parent != "" {
			p["transcript_path"] = parent
		}
		return e.k.run("review-checks", p)
	}
	quiet := func(t *testing.T, e reviewEnv, r result) {
		t.Helper()
		r.Want(t, 0)
		r.Empty(t)
		if calls := testutil.Calls(t, e.calls); calls != nil {
			t.Errorf("ran: %s", calls)
		}
	}
	bash := func(command string) any { return testutil.ToolUse("", "Bash", map[string]any{"command": command}) }

	t.Run("a review whose tool output shows a run-checks summary stops freely", func(t *testing.T) {
		e := setup(t, `{"skillName":"code-review"}`)
		testutil.AppendJSONL(t, e.transcript, testutil.UserPrompt("review this"),
			testutil.ToolUse("t1", "Bash", map[string]any{"command": "if time kit run-checks; then :; fi"}),
			testutil.ToolResult("t1", []any{map[string]any{"type": "text", "text": "PASS go: vet\nchecks: 5 passed, 0 failed, 1 skipped"}}))
		quiet(t, e, stop(e, e.project, ""))
	})
	t.Run("a run-checks summary in persisted output counts", func(t *testing.T) {
		e := setup(t, `{"skillName":"code-review"}`)
		saved := filepath.Join(t.TempDir(), "out.txt")
		testutil.Write(t, saved, "PASS go: vet\n...\nchecks: 5 passed, 0 failed, 1 skipped\n")
		testutil.AppendJSONL(t, e.transcript, testutil.ToolResult("t1",
			"<persisted-output>\nOutput too large (84.2KB). Full output saved to: "+saved+"\n\nPreview (first 2KB):\nPASS go: vet\n</persisted-output>"))
		quiet(t, e, stop(e, e.project, ""))
	})
	// What the transcript shows short of a summary in a tool result.
	for name, lines := range map[string][]any{
		"a review that only listed the checks with --plan is still sent back": {bash("kit run-checks --plan | head")},
		"a search or echo of the text, or --plan after a line continuation, is still sent back": {
			bash(`rg -n "kit run-checks" rules/`),
			bash("echo next: kit run-checks && git log --grep 'wire kit run-checks'"),
			bash("kit run-checks \\\n  --plan"),
		},
		"a review that only mentioned kit run-checks is still sent back": {testutil.AssistantText("I'll run kit run-checks later")},
	} {
		t.Run(name, func(t *testing.T) {
			e := setup(t, `{"skillName":"code-review"}`)
			testutil.AppendJSONL(t, e.transcript, lines...)
			stop(e, e.project, "").Want(t, 2)
		})
	}
	t.Run("a review outside any project stops freely", func(t *testing.T) {
		e := setup(t, `{"skillName":"code-review"}`)
		quiet(t, e, stop(e, t.TempDir(), ""))
	})
	t.Run("another forked skill stops freely", func(t *testing.T) {
		e := setup(t, `{"skillName":"simplify"}`)
		quiet(t, e, stop(e, e.project, ""))
	})
	t.Run("a plain subagent, with no sidecar, stops freely", func(t *testing.T) {
		e := setup(t, "")
		quiet(t, e, stop(e, e.project, ""))
	})

	// A model-invoked foreground fork writes no .forked-skill.json; the
	// parent's pending Skill call names it.
	parent := func(t *testing.T, lines ...any) string {
		path := filepath.Join(t.TempDir(), "parent.jsonl")
		testutil.AppendJSONL(t, path, lines...)
		return path
	}
	skillCall := testutil.ToolUse("s1", "Skill", map[string]any{"skill": "code-review"})
	t.Run("a fork with no sidecar is a review when the parent's code-review Skill call is pending", func(t *testing.T) {
		e := setup(t, "")
		stop(e, e.project, parent(t, skillCall)).Want(t, 2)
	})
	t.Run("a subagent after a finished review call stops freely", func(t *testing.T) {
		e := setup(t, "")
		done := testutil.ToolResult("s1", `Skill "code-review" completed (forked execution).`)
		quiet(t, e, stop(e, e.project, parent(t, skillCall, done)))
	})
}
