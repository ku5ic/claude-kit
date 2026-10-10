package enforce

import (
	"path/filepath"
	"testing"
)

// commitMsg plans the commit-msg checks of a message file outside the repo.
func (e *env) commitMsg() Plan {
	e.t.Helper()
	e.plan()
	return CommitMsg(e.cfg, Options{Root: e.root, CacheDir: e.cache}, "/tmp/msg")
}

func TestCommitMsgRunsCommitlintsConfigWhenNoHookDoes(t *testing.T) {
	e := setup(t)
	e.write("package.json", `{"commitlint":{"extends":["@commitlint/config-conventional"]}}`)
	e.tool("node_modules/.bin/commitlint")
	if got := commands(e.commitMsg()); got != "commit-msg (package.json: commitlint): commitlint --edit /tmp/msg" {
		t.Errorf("plan:\n%s", got)
	}
}

// A hook manager's commit-msg hook decides; commitlint's config alone would
// run it a second time.
func TestCommitMsgRunsTheHookManagersHooks(t *testing.T) {
	e := setup(t)
	e.write("package.json", `{"commitlint":{}}`)
	e.tool("node_modules/.bin/commitlint")
	e.write(".husky/commit-msg", "commitlint --edit \"$1\"\n")
	e.write("lefthook.yml", "commit-msg:\n  commands:\n    lint:\n      run: commitlint --edit {1}\n")
	e.write(".pre-commit-config.yaml", "repos:\n  - repo: local\n    hooks:\n      - id: gitlint\n        name: gitlint\n        entry: gitlint\n        language: system\n        stages: [commit-msg]\n")
	e.tool(filepath.Join(e.path, "pre-commit"))
	want := "commit-msg (.pre-commit-config.yaml): pre-commit run --hook-stage commit-msg --commit-msg-filename /tmp/msg\n" +
		"commit-msg (lefthook.yml: lint): commitlint --edit /tmp/msg\n" +
		"commit-msg (.husky/commit-msg: commit-msg): set -- /tmp/msg\ncommitlint --edit \"$1\""
	if got := commands(e.commitMsg()); got != want {
		t.Errorf("plan:\n%s\nwant:\n%s", got, want)
	}
}
