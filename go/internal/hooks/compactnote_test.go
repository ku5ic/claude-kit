package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// noteEnv is a compaction in testproject: a session transcript, and the
// three things the note is made of, each written by the case that wants it.
type noteEnv struct {
	*contextEnv
	transcript string
}

func noteSetup(t *testing.T) *noteEnv {
	t.Helper()
	e := &noteEnv{contextEnv: contextSetup(t)}
	e.transcript = filepath.Join(e.tmp, "s1.jsonl")
	testutil.Write(t, e.transcript, "")
	return e
}

// plan approves a plan file, as ExitPlanMode does, and returns its path.
func (e *noteEnv) plan(body string) string {
	e.t.Helper()
	testutil.Write(e.t, filepath.Join(e.root, ".claude", "settings.json"), `{"plansDirectory": ".claude/plans"}`)
	path := filepath.Join(e.root, ".claude", "plans", "plan-x.md")
	testutil.Write(e.t, path, body)
	e.run("plan-mode-context", map[string]any{"hook_event_name": "PostToolUse", "tool_name": "ExitPlanMode", "session_id": "s1", "cwd": e.root}).Want(e.t, 0)
	return path
}

// edit adds an Edit of each file under the project to the transcript.
func (e *noteEnv) edit(files ...string) {
	e.t.Helper()
	for _, f := range files {
		testutil.AppendJSONL(e.t, e.transcript, testutil.ToolUse("", "Edit", map[string]any{"file_path": filepath.Join(e.root, f)}))
	}
}

// report writes the repo's last Stop report and returns its path.
func (e *noteEnv) report(body string) string {
	e.t.Helper()
	path := cache.StopReport(e.paths.CacheDir(), e.root)
	testutil.Write(e.t, path, body)
	return path
}

func (e *noteEnv) compact() result {
	e.t.Helper()
	return e.run("compact-note", map[string]any{"hook_event_name": "PreCompact", "trigger": "auto", "session_id": "s1", "cwd": e.root, "transcript_path": e.transcript})
}

func (e *noteEnv) start(source string) result {
	e.t.Helper()
	return e.run("inject-context", map[string]any{"hook_event_name": "SessionStart", "source": source, "session_id": "s1", "cwd": e.root, "transcript_path": e.transcript})
}

// everything fills the note past the cap: a plan, a long failing report,
// and more files than fit.
func (e *noteEnv) everything() (plan string) {
	e.t.Helper()
	plan = e.plan("## Steps\n\n- [x] 1. done\n- [ ] 2. **Next up.** " + strings.Repeat("detail ", 300) + "\n")
	e.report("FAIL test\n" + strings.Repeat("a failure line\n", 100))
	var files []string
	for i := range 400 {
		files = append(files, fmt.Sprintf("pkg/some/long/dir/file%03d.go", i))
	}
	e.edit(files...)
	return plan
}

func TestCompactNote(t *testing.T) {
	t.Parallel()
	const capped = 10_000
	t.Run("the note names the active plan, its first open step, the last Stop report, and each edited file once", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		plan := e.plan("## Steps\n\n- [x] 1. done\n- [ ] 2. **Next up.** More.\n- [ ] 3. later\n")
		e.report("PASS lint\nchecks: 1 passed, 0 failed, 0 skipped\n")
		e.edit("b.go", "a.go", "b.go")
		testutil.Write(t, e.paths.SessionFile(cache.Claims, "s1", bashEditsRecord), `{"id":"t1","files":["`+filepath.Join(e.root, "c.go")+`"]}`+"\n")
		r := e.compact()
		r.Want(t, 0)
		r.Has(t, "plan: "+plan+"\n", "next step: 2. **Next up.** More.\n", "PASS lint\nchecks: 1 passed", "\na.go\nb.go\nc.go\n")
		r.Lacks(t, "3. later", "{")
	})
	t.Run("a plan picked up from a global plans folder under git is named for the session's project", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		plans := e.paths.PlansHome()
		testutil.Mkdir(t, plans)
		testutil.Git(t, plans, "init", "-q")
		plan := filepath.Join(testutil.Physical(t, plans), "plan-x.md")
		testutil.Write(t, plan, "## Steps\n\n- [x] 1. done\n- [ ] 2. Next up.\n")
		e.run("plan-tick", map[string]any{"hook_event_name": "PostToolUse", "tool_name": "Edit", "session_id": "s1", "cwd": e.root, "transcript_path": e.transcript, "tool_input": map[string]any{"file_path": plan}}).Want(t, 0)
		e.compact().Has(t, "plan: "+plan+"\n", "next step: 2. Next up.\n")
	})
	t.Run("a file outside the project keeps its whole path", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		testutil.AppendJSONL(t, e.transcript, testutil.ToolUse("", "Write", map[string]any{"file_path": "/elsewhere/x.go"}))
		e.compact().Has(t, "\n/elsewhere/x.go\n")
	})
	t.Run("with nothing to say it prints nothing", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		r := e.compact()
		r.Want(t, 0)
		r.Empty(t)
	})
	t.Run("a plan with no open step, or another session's, isn't named", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		plan := e.plan("## Steps\n\n- [ ] one\n")
		e.run("compact-note", map[string]any{"hook_event_name": "PreCompact", "session_id": "s2", "cwd": e.root}).Empty(t)
		testutil.Write(t, plan, "## Steps\n\n- [x] one\n")
		e.compact().Empty(t)
	})
	t.Run("a Stop report from before the session isn't its own", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		prompt := testutil.UserPrompt("go")
		prompt["timestamp"] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
		testutil.AppendJSONL(t, e.transcript, prompt)
		path := e.report("PASS lint\n")
		e.compact().Has(t, "PASS lint")
		testutil.Age(t, 2*time.Hour, path)
		e.compact().Empty(t)
	})
	t.Run("past the cap the file list is cut first, then the report, never the plan", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		plan := e.everything()
		r := e.compact()
		r.Want(t, 0)
		if len(r.Stdout) >= capped {
			t.Errorf("stdout is %d bytes", len(r.Stdout))
		}
		r.Has(t, "plan: "+plan, "next step: 2. **Next up.**", "file000.go", " more)\n</compaction-note>")
		if n := strings.Count(r.Stdout, "a failure line"); n != 100 {
			t.Errorf("%d report lines, want all 100: the files are cut first", n)
		}

		e.report("FAIL test\n" + strings.Repeat("a failure line\n", 2000))
		r = e.compact()
		if len(r.Stdout) >= capped {
			t.Errorf("stdout is %d bytes", len(r.Stdout))
		}
		r.Has(t, "plan: "+plan, "FAIL test", " more)\n")
		r.Lacks(t, "file000.go")
	})
	t.Run("a transcript that can't be read and a bad payload never block", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		e.plan("## Steps\n\n- [ ] one\n")
		if err := os.Remove(e.transcript); err != nil {
			t.Fatal(err)
		}
		r := e.compact()
		r.Want(t, 0)
		r.Has(t, "next step: one")
		e.run("compact-note", "{not json").Want(t, 0)
	})
}

func TestInjectContextPrintsTheCompactionNote(t *testing.T) {
	t.Parallel()
	const capped = 10_000
	// blocks turns on every context block: rules, suggestions, repo, tooling.
	blocks := func(e *noteEnv, more string) {
		e.kitYML(fileSkillsYML[strings.Index(fileSkillsYML, "skill_triggers"):] + "global_skills: [slop-patterns]\ntools: [git]\nskip_dirs: [node_modules]\nreply_limits:\n  modes: {terse: {chat: 25}}\n" + more)
		testutil.Write(e.t, filepath.Join(e.root, "main.go"), "package main\n")
		testutil.Write(e.t, filepath.Join(e.root, "go.mod"), "module x\n")
	}
	all := []string{"<repo-context>", "<required-skills>", "<suggested-skills>", "<tooling>", "Reply target"}
	t.Run("only after a compaction", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		blocks(e, "")
		plan := e.plan("## Steps\n\n- [ ] one\n")
		for _, source := range []string{"startup", "resume", "clear"} {
			e.start(source).Lacks(t, "compaction-note")
		}
		e.start("compact").Has(t, append(all, "<compaction-note>", "plan: "+plan)...)
	})
	t.Run("a long note beside every block stays under the cap as plain stdout, the plan kept", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		blocks(e, "")
		plan := e.everything()
		r := e.start("compact")
		r.Want(t, 0)
		r.Has(t, append(all, "plan: "+plan, " more)\n")...)
		if len(r.Stdout) >= capped || len(r.Stdout) < capped/2 {
			t.Errorf("stdout is %d bytes, want it filled to just under %d", len(r.Stdout), capped)
		}
	})
	t.Run("and inside the JSON string, beside an install problem", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		blocks(e, "protected_branches: [main]\n")
		plan := e.everything()
		e.overlay("protected_branches: main\n")
		r := e.start("compact")
		r.Want(t, 0)
		ctx := parseSessionStart(t, r.Stdout).HookSpecificOutput.AdditionalContext
		testutil.Has(t, ctx, append(all, "ignored", "plan: "+plan, " more)\n")...)
		if len(ctx) >= capped || len(ctx) < capped/2 {
			t.Errorf("additionalContext is %d bytes, want it filled to just under %d", len(ctx), capped)
		}
	})
	t.Run("a plan line with no room is dropped, and the files that fit stay", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		plan := e.plan("## Steps\n\n- [ ] 1. next\n")
		e.edit("a.go")
		payload, err := json.Marshal(map[string]any{"session_id": "s1", "cwd": e.root, "transcript_path": e.transcript})
		if err != nil {
			t.Fatal(err)
		}
		h := &hook.Hook{Payload: hook.ParsePayload(payload), Paths: e.paths, Home: e.home}
		const frame, files = "<compaction-note>\nx\n</compaction-note>\n", "files this session edited:\na.go\n(1 more)\n"
		room := len(frame) + len(files)
		if len("plan: "+plan) < room-len(frame) {
			t.Fatalf("the plan path %s fits the room; the case needs a longer one", plan)
		}
		note := compactNote(h, "x", room)
		testutil.Has(t, note, "<compaction-note>", "a.go")
		testutil.Lacks(t, note, "plan: ")
	})
	t.Run("no room for any line leaves the note out", func(t *testing.T) {
		t.Parallel()
		e := noteSetup(t)
		e.kitYML("skip_dirs: [" + strings.Repeat("a-long-directory-name, ", 430) + "z]\n")
		testutil.Write(t, filepath.Join(e.root, "go.mod"), "module x\n")
		e.everything()
		e.start("compact").Lacks(t, "compaction-note")
	})
}
