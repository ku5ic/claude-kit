package e2e

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"

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
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil || len(out) != 1 || out["systemMessage"] != "stop checks: 1 passed, 0 failed, 0 skipped\n  PASS lint (package.json: *.ts)" {
		t.Errorf("a pass prints more or other than its systemMessage:\n%s", r.Output)
	}

	Touch(t, path("fail"))
	r = stop()
	r.Want(t, 2)
	r.Has(t, "FAIL lint (package.json: *.ts)\n  cmd: fakelint --check a.ts\n", "lint noise", "checks: 0 passed, 1 failed, 0 skipped")
}
