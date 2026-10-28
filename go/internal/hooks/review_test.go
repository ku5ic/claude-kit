package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// reviewEnv is one review-checks case: a git repo whose package.json has a
// lint script classified as a lint check, a recording npm first on PATH,
// and the reviewer's transcript.
type reviewEnv struct {
	k                          *sandbox
	project, calls, transcript string
}

// Not parallel: each case puts its recording npm on PATH.
func TestReviewChecks(t *testing.T) {
	// setup writes the fork's .forked-skill.json sidecar unless it is "";
	// npm runs body, so "exit 1" fails the lint gate.
	setup := func(t *testing.T, sidecar, body string) reviewEnv {
		tmp := t.TempDir()
		e := reviewEnv{k: newSandbox(t), project: filepath.Join(tmp, "project"), calls: filepath.Join(tmp, "npm.calls"),
			transcript: filepath.Join(tmp, "agent-x.jsonl")}
		testutil.Mkdir(t, e.project)
		testutil.Git(t, e.project, "init", "-q", "-b", "main")
		testutil.Write(t, filepath.Join(e.project, "package.json"), `{"scripts":{"lint":"eslint ."}}`+"\n")
		testutil.FakeTool(t, filepath.Join(tmp, "stubs", "npm"), e.calls, body)
		t.Setenv("PATH", filepath.Join(tmp, "stubs")+":"+os.Getenv("PATH"))
		lint := sources.Entry{Source: "package-scripts", File: "package.json", Name: "lint", Dir: ".", Body: sources.Body{Text: "eslint ."}}
		e.k.classify(e.project, envelope(t, []map[string]any{verdict(lint, map[string]any{"role": "check", "kind": "lint"})}, nil, nil))
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
	// ran wants the full gate to have run the lint script and shown the user
	// every verdict.
	ran := func(t *testing.T, e reviewEnv, r result) map[string]string {
		t.Helper()
		r.Want(t, 0)
		var out map[string]string
		if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil || !strings.HasPrefix(out["systemMessage"], "run-checks checks: ") ||
			!strings.Contains(out["systemMessage"], "\n  ") {
			t.Errorf("want every verdict for the user:\n%s", r.Output)
		}
		if calls := testutil.Calls(t, e.calls); !strings.Contains(strings.Join(calls, "\n"), "run lint") {
			t.Errorf("npm calls %q, want run lint", calls)
		}
		return out
	}
	quiet := func(t *testing.T, e reviewEnv, r result) {
		t.Helper()
		r.Want(t, 0)
		r.Empty(t)
		if calls := testutil.Calls(t, e.calls); calls != nil {
			t.Errorf("ran: %s", calls)
		}
	}

	t.Run("a review of the checkout runs the full gate and keeps the tree it checked", func(t *testing.T) {
		e := setup(t, `{"skillName":"code-review"}`, "")
		out := ran(t, e, stop(e, e.project, ""))
		if out["decision"] != "" || !strings.HasPrefix(out["systemMessage"], "run-checks checks: 1 passed, 0 failed") {
			t.Errorf("a pass lets the reviewer stop: %+v", out)
		}
		if _, err := os.Stat(cache.FullGateTree(e.k.paths.CacheDir(), testutil.Physical(t, e.project))); err != nil {
			t.Errorf("the checked tree wasn't kept: %v", err)
		}
	})
	t.Run("a failing gate sends the reviewer back with it", func(t *testing.T) {
		e := setup(t, `{"skillName":"code-review"}`, "exit 1")
		out := ran(t, e, stop(e, e.project, ""))
		if out["decision"] != "block" || !strings.Contains(out["reason"], "FAIL lint (package.json: lint)") ||
			!strings.HasPrefix(out["systemMessage"], "run-checks checks: 0 passed, 1 failed") {
			t.Errorf("want a block naming the failure: %+v", out)
		}
	})
	t.Run("a review outside any project stops freely", func(t *testing.T) {
		e := setup(t, `{"skillName":"code-review"}`, "")
		quiet(t, e, stop(e, t.TempDir(), ""))
	})
	t.Run("another forked skill stops freely", func(t *testing.T) {
		e := setup(t, `{"skillName":"simplify"}`, "")
		quiet(t, e, stop(e, e.project, ""))
	})
	t.Run("a plain subagent, with no sidecar, stops freely", func(t *testing.T) {
		e := setup(t, "", "")
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
		e := setup(t, "", "")
		ran(t, e, stop(e, e.project, parent(t, skillCall)))
	})
	t.Run("a subagent after a finished review call stops freely", func(t *testing.T) {
		e := setup(t, "", "")
		done := testutil.ToolResult("s1", `Skill "code-review" completed (forked execution).`)
		quiet(t, e, stop(e, e.project, parent(t, skillCall, done)))
	})
}
