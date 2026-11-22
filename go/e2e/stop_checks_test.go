package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// `kit hook stop-checks` wiring: a repo whose package.json runs fakelint on
// .ts files through lint-staged, classified as a lint check by kit enforce
// classify with a stub classifier. A turn that edits a.ts passes with the
// report in a systemMessage alone, and blocks with exit 2 and the check's
// output once fakelint fails. The turn reading, the plan gate, and the
// report are tested in process, in internal/hooks.
func TestStopChecks(t *testing.T) {
	t.Parallel()
	k := New(t)
	tmp := t.TempDir()
	Mkdir(t, filepath.Join(tmp, "repo/node_modules/.bin"))
	repo := k.Repo(filepath.Join(tmp, "repo"))
	path := func(rel string) string { return filepath.Join(repo, rel) }
	Stub(t, path("node_modules/.bin/fakelint"), fmt.Sprintf("echo \"$PWD|$*\" >>%q\necho \"lint noise\"\n[[ ! -e %q ]]\n", filepath.Join(tmp, "calls"), path("fail")))
	Write(t, path(".gitignore"), "node_modules\n")
	Write(t, path("package.json"), `{"lint-staged":{"*.ts":"fakelint --check"}}`)
	Write(t, path("a.ts"), "x\n")
	lint := sources.Entry{Source: "lint-staged", File: "package.json", Name: "*.ts", Dir: ".", Body: sources.Body{Text: "fakelint --check"}, Files: []string{"*.ts"}, PassFiles: true, Stage: "pre-commit"}
	answer := `{"is_error":false,"subtype":"success","structured_output":{"entries":[{"id":"` + gapfill.Key(lint) + `","role":"check","kind":"lint","mutates":false}],"managers":[],"proposals":[]}}`
	classifier := filepath.Join(tmp, "classifier")
	Stub(t, classifier, "cat >/dev/null\necho '"+answer+"'\n")
	k.Overlay("classifier: [" + strconv.Quote(classifier) + "]\n")
	k.Dir = repo
	k.Run("", "enforce", "classify").Want(t, 0)
	k.Dir = k.Home

	transcript := filepath.Join(tmp, "transcript.jsonl")
	testutil.AppendJSONL(t, transcript, testutil.UserPrompt("do it"),
		testutil.ToolUse("", "Edit", map[string]any{"file_path": path("a.ts")}),
		map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result"}}}})
	stop := func() Result {
		return k.Hook("stop-checks", map[string]any{
			"hook_event_name": "Stop", "session_id": "s1", "cwd": repo,
			"transcript_path": transcript, "stop_hook_active": false,
		})
	}

	r := stop()
	r.Want(t, 0)
	var out map[string]string
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil || len(out) != 1 || out["systemMessage"] != "kit stop: 0 failed, 0 skipped, 1 passed\n\n  pass  lint" {
		t.Errorf("a pass prints more or other than its systemMessage:\n%s", r.Output)
	}

	Touch(t, path("fail"))
	r = stop()
	r.Want(t, 0)
	out = nil
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil || out["decision"] != "block" || !strings.HasPrefix(out["systemMessage"], "kit stop: 1 failed, 0 skipped, 0 passed\n\n  FAIL  lint (package.json: *.ts)  -> ") {
		t.Errorf("a failure blocks with its detail, and the receipt keeps the counts and names the failure:\n%s", r.Output)
	}
	for _, s := range []string{"kit stop: BLOCKED 1 of 1 checks failed", "FAIL  lint (package.json: *.ts)\n", "lint noise", "\nmore  "} {
		if !strings.Contains(out["reason"], s) {
			t.Errorf("the block's reason lacks %q:\n%s", s, out["reason"])
		}
	}

	// receipt: failures hides a clean pass, and the gate still runs; a
	// failure still shows, with the overrides in force.
	k.Overlay("classifier: [" + strconv.Quote(classifier) + "]\nreceipt: failures\ndisabled_rules: [plan-critic]\n")
	calls := func() int { return len(testutil.Calls(t, filepath.Join(tmp, "calls"))) }
	if err := os.Remove(path("fail")); err != nil {
		t.Fatal(err)
	}
	// A new tree: on the first pass's tree, Stop would reuse that pass.
	Write(t, path("a.ts"), "y\n")
	before := calls()
	r = stop()
	r.Want(t, 0)
	if r.Stdout != "" || calls() != before+1 {
		t.Errorf("a hidden receipt printed, or its gate didn't run (%d calls, want %d):\n%s", calls(), before+1, r.Output)
	}
	Touch(t, path("fail"))
	r = stop()
	out = nil
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil || out["decision"] != "block" ||
		!strings.HasPrefix(out["systemMessage"], "kit stop: 1 failed, 0 skipped, 0 passed\n\n  FAIL  lint (package.json: *.ts)  -> ") ||
		!strings.HasSuffix(out["systemMessage"], "\n  override  disabled_rules: plan-critic") {
		t.Errorf("a failure with receipt: failures still blocks and shows its receipt:\n%s", r.Output)
	}
}

// A check edited mid-session is classified once: the edit starts a
// background classify, and the Stop after it finds the answer and runs the
// check without asking again.
func TestAChangedCheckIsClassifiedOnceBetweenTheEditAndStop(t *testing.T) {
	t.Parallel()
	k := New(t)
	tmp := t.TempDir()
	Mkdir(t, filepath.Join(tmp, "repo/node_modules/.bin"))
	repo := k.Repo(filepath.Join(tmp, "repo"))
	path := func(rel string) string { return filepath.Join(repo, rel) }
	checked := filepath.Join(tmp, "checked")
	Stub(t, path("node_modules/.bin/fakelint"), fmt.Sprintf("echo \"$*\" >>%q\n", checked))
	Write(t, path(".gitignore"), "node_modules\n")
	Write(t, path("package.json"), `{"lint-staged":{"*.ts":"fakelint --check"}}`)
	Write(t, path("a.ts"), "x\n")
	var ids []string
	for _, body := range []string{"fakelint --check", "fakelint --check --strict"} {
		e := sources.Entry{Source: "lint-staged", File: "package.json", Name: "*.ts", Dir: ".", Body: sources.Body{Text: body}, Files: []string{"*.ts"}, PassFiles: true, Stage: "pre-commit"}
		ids = append(ids, `{"id":"`+gapfill.Key(e)+`","role":"check","kind":"lint","mutates":false}`)
	}
	answer := `{"is_error":false,"subtype":"success","structured_output":{"entries":[` + strings.Join(ids, ",") + `],"managers":[],"proposals":[]}}`
	classifier, asked := filepath.Join(tmp, "classifier"), filepath.Join(tmp, "asked")
	Stub(t, classifier, fmt.Sprintf("echo x >>%q\ncat >/dev/null\necho '%s'\n", asked, answer))
	k.Overlay("classifier: [" + strconv.Quote(classifier) + "]\n")
	k.Dir = repo
	k.Run("", "enforce", "classify").Want(t, 0)
	k.Dir = k.Home
	// The changed body asks as the first classify did: the facts, then the entries.
	once := len(testutil.Calls(t, asked))

	Write(t, path("package.json"), `{"lint-staged":{"*.ts":"fakelint --check --strict"}}`)
	k.Hook("format-dispatch", map[string]any{"tool_input": map[string]any{"file_path": path("package.json")}}).Want(t, 0)
	for deadline := time.Now().Add(20 * time.Second); len(testutil.Calls(t, asked)) < 2*once; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the edit started no classify run")
		}
	}

	transcript := filepath.Join(tmp, "transcript.jsonl")
	testutil.AppendJSONL(t, transcript, testutil.UserPrompt("do it"),
		testutil.ToolUse("", "Edit", map[string]any{"file_path": path("a.ts")}),
		map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result"}}}})
	k.Hook("stop-checks", map[string]any{
		"hook_event_name": "Stop", "session_id": "s1", "cwd": repo,
		"transcript_path": transcript, "stop_hook_active": false,
	}).Want(t, 0)
	if got := strings.TrimSpace(Read(t, checked)); got != "--check --strict a.ts" {
		t.Errorf("checked %q, want the changed check on a.ts", got)
	}
	if got := len(testutil.Calls(t, asked)); got != 2*once {
		t.Errorf("the classifier ran %d times, want %d: one classify before the edit, one after", got, 2*once)
	}
}
