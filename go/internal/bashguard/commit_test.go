package bashguard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestCommitting(t *testing.T) {
	t.Parallel()
	for cmd, want := range map[string]bool{
		`git commit -m "x"`:                           true,
		`git -C repo -c user.name=x commit -qm x`:     true,
		"git -C . \\\ncommit -m x":                    true,
		`FOO=1 command git commit --amend`:            true,
		`echo $(git commit -m x)`:                     true,
		`git status && make commit`:                   false,
		`echo "git commit -m x"`:                      false,
		`git log --grep commit`:                       false,
		"cat > n.md <<EOF\ngit commit is a word\nEOF": false,
		// mvdan can't parse the unterminated heredoc: the raw text decides.
		"git commit -m \"$(cat <<EOF\nfeat: x\n)\"": true,
	} {
		if got := commitCalls(cmd, "", "/")[0].commits(cmd); got != want {
			t.Errorf("committing(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestMessageIsWhatTheCommitMsgHookWouldRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.Write(t, filepath.Join(dir, "sub", "msg.txt"), "fix: from a file\n")
	for cmd, want := range map[string]string{
		"git commit -m \"$(cat <<'EOF'\nfeat: x\n\nbody\nEOF\n)\"":     "feat: x\n\nbody\n",
		"git commit -F - <<'EOF'\nfix: y\nEOF":                         "fix: y\n",
		`git commit -m "feat: a" -m "body"`:                            "feat: a\n\nbody\n",
		`git commit -am "feat: cluster"`:                               "feat: cluster\n",
		"git commit -m \"feat: a\" -m \"$(cat <<'EOF'\nbody\nEOF\n)\"": "feat: a\n\nbody\n",
		`git commit --message="feat: long"`:                            "feat: long\n",
		`git -C sub commit -F msg.txt`:                                 "fix: from a file\n",
		// Written by the command itself: what's on disk now is stale.
		`echo "feat: new" > sub/msg.txt && git -C sub commit -F msg.txt`: "",
		// Named only after the commit, as a cleanup: the file on disk is the message.
		`git -C sub commit -F msg.txt && rm sub/msg.txt`: "fix: from a file\n",
		`git commit --amend --no-edit`:                   "",
	} {
		c := commitCalls(cmd, "", dir)[0]
		if got, _ := message(cmd, c, c.repo, ""); got != want {
			t.Errorf("message(%q) = %q, want %q", cmd, got, want)
		}
	}
	// An expansion: the text as written, but not the message git gets.
	cmd := `git commit -m "fix: $MSG"`
	c := commitCalls(cmd, "", dir)[0]
	if got, unread := message(cmd, c, c.repo, ""); got != "fix: $MSG\n" || unread == "" {
		t.Errorf("message(%q) = %q, %q, want the text as written and unread", cmd, got, unread)
	}
}

// A message the guard can't read blocks, since no check sees it; a commit
// that gives none (--no-edit) goes on.
func TestGuardCommitBlocksAnUnreadMessage(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	cwd := testutil.Physical(t, t.TempDir())
	testutil.Write(t, filepath.Join(cwd, "msg.txt"), "fix: from a file\n")
	for cmd, status := range map[string]int{
		`git commit -m "$msg"`: 2,
		"git commit -m \"fix: x\" -m \"$(git log -1 --format=%b)\"": 2,
		`printf 'fix: x' | git commit -F -`:                         2,
		`echo "fix: new" > msg.txt && git commit -F msg.txt`:        2,
		`S=/tmp && git commit -F $S/msg.txt`:                        2,
		`git commit -F missing.txt`:                                 2,
		"git commit -F - <<'EOF'\nfix: y\nEOF":                      0,
		"git commit -m \"$(cat <<'EOF'\nfix: y\nEOF\n)\"":           0,
		`git commit -F msg.txt`:                                     0,
		`git commit -m "fix: costs \$5"`:                            0,
		`git commit --amend --no-edit`:                              0,
	} {
		t.Run(cmd, func(t *testing.T) {
			r := k.commit(cwd, cmd)
			r.Want(t, status)
			if status == 2 {
				r.Has(t, "can't read the commit message", "rule  unread-text")
			}
		})
	}
}

// commitlint's config, and a stub commitlint taking only feat: and fix:.
func commitlintRepo(t *testing.T, k *sandbox) string {
	t.Helper()
	repo := gitRepo(t, filepath.Join(k.home, "repo"), "main")
	testutil.Write(t, filepath.Join(repo, "package.json"), `{"commitlint":{"extends":["@commitlint/config-conventional"]}}`)
	testutil.Write(t, filepath.Join(repo, ".gitignore"), "node_modules\n")
	bin := filepath.Join(repo, "node_modules", ".bin", "commitlint")
	testutil.Write(t, bin, "#!/bin/sh\nhead -1 \"$2\" | grep -Eq '^(feat|fix)(\\(.+\\))?: ' || { echo 'type must be one of [feat, fix] [type-enum]'; exit 1; }\n")
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestTheProjectsCommitMsgCheckBlocksAMessageItRejects(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	repo := commitlintRepo(t, k)
	r := k.commit(repo, `git commit -m "updated stuff"`)
	r.Want(t, 2)
	r.Has(t, "the project's commit-msg checks reject this message", "FAIL commit-msg (package.json: commitlint)", "type must be one of [feat, fix]")
	k.commit(repo, `git commit -m "feat: add the thing"`).Want(t, 0)
	k.commit(repo, `git commit -m "$MSG"`).Has(t, "rule  unread-text")
}

func TestTheCommitMsgCheckIsTheRepoGitDashCNames(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	repo := commitlintRepo(t, k)
	plain := gitRepo(t, filepath.Join(k.home, "plain"), "main")
	k.commit(plain, `git -C ../repo commit -m "updated stuff"`).Want(t, 2)
	k.commit(repo, `git -C ../plain commit -m "updated stuff"`).Want(t, 0)
	// A cd earlier in the line moves the repo too; one inside a pipeline doesn't.
	k.commit(plain, `cd ../repo && git commit -m "updated stuff"`).Want(t, 2)
	k.commit(repo, `cd ../plain; git commit -m "updated stuff"`).Want(t, 0)
	k.commit(plain, `cd ../repo | true && git commit -m "updated stuff"`).Want(t, 0)
	k.commit(plain, `cd .. && git -C repo commit -m "updated stuff"`).Want(t, 2)
	// A cd in a substitution runs in a subshell: the commit stays put.
	for _, cmd := range []string{`git commit -m "$(cd /tmp && echo x)"`, `(cd /tmp) && git commit -m x`} {
		if c := commitCalls(cmd, "", repo)[0]; c.repo != repo {
			t.Errorf("%s: repo = %q, want %q", cmd, c.repo, repo)
		}
	}
}

// prose is n consecutive prose lines.
func prose(n int) string { return strings.Repeat("a line of plain prose\n", n) }

// gitleaksStub puts a gitleaks first on PATH for the rest of t, one that
// exits code and records its runs in the file it returns. PATH is the
// process's, so t can't run in parallel.
func gitleaksStub(t *testing.T, code int) string {
	t.Helper()
	dir := t.TempDir()
	calls := filepath.Join(dir, "gitleaks.calls")
	testutil.FakeTool(t, filepath.Join(dir, "gitleaks"), calls, fmt.Sprintf("exit %d", code))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return calls
}

// skills/kit-rules/reference/workflow.md section 1: a commit request is the
// go, so a commit gets a prompt unless the current prompt asks for it or an
// earlier turn showed its subject.
func TestGuardCommitAsksForAnUnshownMessage(t *testing.T) {
	gitleaksStub(t, 0)
	k := newSandbox(t)
	commit := func(transcript ...any) result {
		t.Helper()
		k.transcript = ""
		if transcript != nil {
			k.transcript = filepath.Join(t.TempDir(), "t.jsonl")
			testutil.AppendJSONL(t, k.transcript, transcript...)
		}
		return k.commit("", `git commit -m "fix: add sums"`)
	}
	for name, transcript := range map[string][]any{
		"no transcript":                {nil},
		"never shown, never asked":     {testutil.UserPrompt("fix it")},
		"shown only in this turn":      {testutil.UserPrompt("fix it"), testutil.AssistantText("Committing `fix: add sums`.")},
		"asked not to commit":          {testutil.UserPrompt("fix it, but don't commit yet")},
		"asked not to, plainly":        {testutil.UserPrompt("Do not commit this.")},
		"not ready yet":                {testutil.UserPrompt("I'm not ready to commit.")},
		"a contracted refusal":         {testutil.UserPrompt("We aren't going to commit this")},
		"a curly-quoted refusal":       {testutil.UserPrompt("We aren’t going to commit this")},
		"asks whether to":              {testutil.UserPrompt("Is it ok to commit?")},
		"asks what's left":             {testutil.UserPrompt("what's left to commit?")},
		"the user's own plan":          {testutil.UserPrompt("let me review the diff first, then commit")},
		"only names a commit message":  {testutil.UserPrompt("the commit message prompt is annoying")},
		"an earlier prompt asked":      {testutil.UserPrompt("commit"), testutil.AssistantText("done"), testutil.UserPrompt("now fix the docs")},
		"only mentions a commit":       {testutil.UserPrompt("review the last commit")},
		"asks about a commit":          {testutil.UserPrompt("what changed in commit abc123?")},
		"a pasted git log":             {testutil.UserPrompt("commit 35c7066\nAuthor: someone\n\n    fix: x")},
		"a remark on commit hooks":     {testutil.UserPrompt("commit hooks keep failing")},
		"a remark on commit style":     {testutil.UserPrompt("commit style is wrong here")},
		"a remark on commit-msg":       {testutil.UserPrompt("commit-msg check keeps failing, look into it")},
		"commit/push in a remark":      {testutil.UserPrompt("Hmm, commit/push are blocked")},
		"a conditional":                {testutil.UserPrompt("if you commit this, CI breaks")},
		"a complaint":                  {testutil.UserPrompt("stop, you commit everything without asking")},
		"pasted git status":            {testutil.UserPrompt("why?\n\nOn branch feat\nnothing to commit, working tree clean")},
		"commit after a review":        {testutil.UserPrompt("Commit it after I review the diff")},
		"commit once CI passes":        {testutil.UserPrompt("commit it once CI is green")},
		"commit when it's done":        {testutil.UserPrompt("fix the docs and commit it when the tests pass")},
		"commit until told":            {testutil.UserPrompt("commit them until the release is cut")},
		"a deferral clause first":      {testutil.UserPrompt("When the tests pass, commit it.")},
		"a deferral clause after":      {testutil.UserPrompt("Commit it, but only after I review the diff")},
		"commit if":                    {testutil.UserPrompt("commit it if the tests pass")},
		"commit later":                 {testutil.UserPrompt("commit it later")},
		"commit as soon as":            {testutil.UserPrompt("commit it as soon as CI is green")},
		"commit tomorrow":              {testutil.UserPrompt("commit it tomorrow")},
		"commit unless":                {testutil.UserPrompt("commit it unless the build fails")},
		"an object, then later":        {testutil.UserPrompt("commit the docs fix later")},
		"the subject only mid-reply":   {testutil.UserPrompt("fix it"), testutil.AssistantText("Nothing to fix: add sums of the totals first."), testutil.UserPrompt("go")},
		"commit before a check":        {testutil.UserPrompt("commit it before I review it")},
		"later, after a comma":         {testutil.UserPrompt("Commit it, but later.")},
		"tomorrow, after a comma":      {testutil.UserPrompt("commit it, tomorrow")},
		"not yet, after a comma":       {testutil.UserPrompt("commit it, but not yet")},
		"a deferred pair":              {testutil.UserPrompt("commit and push tomorrow")},
		"someone else said it":         {testutil.UserPrompt("the reviewer said commit it")},
		"I said not to":                {testutil.UserPrompt("I said don't commit")},
		"not now":                      {testutil.UserPrompt("commit it, but not now")},
		"not right now":                {testutil.UserPrompt("commit it, but not right now")},
		"a standing rule":              {testutil.UserPrompt("commit every time tests pass")},
		"permission for any time":      {testutil.UserPrompt("you can commit any time you want")},
		"I said, in narration":         {testutil.UserPrompt("Earlier I said commit, now I'm not sure")},
		"I said, after a comma":        {testutil.UserPrompt("Earlier, I said commit, now I'm not sure")},
		"I said, mid-sentence":         {testutil.UserPrompt("the bug is that I said commit and it ran")},
		"how to, as a question":        {testutil.UserPrompt("Explain how to commit")},
		"a negation before a comma":    {testutil.UserPrompt("Don't, under any circumstances, commit this.")},
		"a negation on the line above": {testutil.UserPrompt("Don't\ncommit this")},
		"a file, after a comma":        {testutil.UserPrompt("commit notes.txt, but only after I review it")},
		"a file, then a deferred pair": {testutil.UserPrompt("commit main.go and push tomorrow")},
		"a remark on a hook path":      {testutil.UserPrompt("commit hooks/pre-commit keeps failing")},
		"an uppercase extension":       {testutil.UserPrompt("commit README.MD")},
		"a negation before a name":     {testutil.UserPrompt("Never, per CONTRIBUTING.MD, commit it.")},
		"a negation around a name":     {testutil.UserPrompt("Don't, as Foo.Bar says, commit this.")},
		"a condition holding a name":   {testutil.UserPrompt("If the tests in Main.Go pass, commit it")},
		"a task notification":          {testutil.UserPrompt("fix it"), map[string]any{"type": "user", "origin": map[string]any{"kind": "task-notification"}, "message": map[string]any{"content": "Commit and continue."}}},
		"a peer session's message":     {testutil.UserPrompt("fix it"), map[string]any{"type": "user", "origin": map[string]any{"kind": "peer"}, "message": map[string]any{"content": "commit it"}}},
	} {
		if transcript[0] == nil {
			transcript = nil
		}
		r := commit(transcript...)
		r.Want(t, 0)
		if !strings.Contains(r.Output, `"permissionDecision":"ask"`) {
			t.Errorf("%s: no ask; output:\n%s", name, r.Output)
		}
	}
	commit(testutil.UserPrompt("fix it"), testutil.AssistantText("Commit as `fix: add sums`?"), testutil.UserPrompt("go")).Empty(t)
	// The proposal sits further back than the transcript's last 4 MB.
	filler := testutil.AssistantText(strings.Repeat("x", 1<<20))
	commit(testutil.UserPrompt("fix it"), testutil.AssistantText("Commit as `fix: add sums`?"),
		filler, filler, filler, filler, filler, testutil.UserPrompt("go")).Empty(t)
	// A subject with quotes sits JSON-escaped in the transcript.
	k.transcript = filepath.Join(t.TempDir(), "quoted.jsonl")
	testutil.AppendJSONL(t, k.transcript, testutil.UserPrompt("fix it"), testutil.AssistantText("Commit as `fix: say \"hi\"`?"), testutil.UserPrompt("go"))
	k.commit("", `git commit -m 'fix: say "hi"'`).Empty(t)
	commit(testutil.UserPrompt("fix it"), testutil.AssistantText("The message:\n\n```text\nfix: add sums\n```\n\nCommit it?"), testutil.UserPrompt("go")).Empty(t)
	for _, item := range []string{"1. fix: add sums", "2) fix: add sums", "+ fix: add sums", "> 1. **fix: add sums**"} {
		if r := commit(testutil.UserPrompt("fix it"), testutil.AssistantText("Proposed:\n\n"+item+"\n\nCommit it?"), testutil.UserPrompt("go")); r.Output != "" {
			t.Errorf("%q shows the message, yet the guard asked:\n%s", item, r.Output)
		}
	}
	for _, prompt := range []string{"Commit and continue.", "Yes, commit.", "fix it and commit", "Commit, then start step 13", "Yes, commit 21, apply both fixes", "can you commit this?", "could you commit it", "please commit", "commit both and continue", "Commit it.", "commit the docs fix", "commit it, then review the next step", "nothing else to fix so commit it", "commit it, but don't push", "commit and push", "commit it, I'll check the diff afterwards", "commit it now, push it tomorrow", "commit it, you can push tomorrow", "I said commit and continue.", "I already said commit it", "Yes, I said commit.", "Fine. I just said commit it", "But first, commit any changes.", "commit every fix", "commit each step", "Ugh, I said commit it", "Yeah, I said commit", "no; I said commit it", "commit it, do not push now", "commit it, not the lockfile, and push now", "commit the time zone fix", "commit the time parsing fix", "Commit notes.txt and .env.", "commit README.md", "commit go/internal/git/git.go and push", "fix it and commit `notes.txt`", "Commit this.Then push"} {
		if r := commit(map[string]any{"type": "user", "origin": map[string]any{"kind": "human"}, "message": map[string]any{"content": prompt}}); strings.Contains(r.Output, `"permissionDecision":"ask"`) {
			t.Errorf("%q asks for the commit, yet the guard asked again:\n%s", prompt, r.Output)
		}
	}
}

// Each commit in a command is approved and scanned on its own: showing
// the first message approves no second one chained after it.
func TestGuardCommitChecksEveryCommit(t *testing.T) {
	gitleaksStub(t, 0)
	k := newSandbox(t)
	k.transcript = filepath.Join(t.TempDir(), "t.jsonl")
	testutil.AppendJSONL(t, k.transcript, testutil.UserPrompt("fix it"), testutil.AssistantText("Commit as `fix: add sums`?"), testutil.UserPrompt("go"))
	k.commit("", `git commit -m "fix: add sums"`).Empty(t)
	r := k.commit("", `git commit -m "fix: add sums" && git commit --allow-empty -m "fix: other"`)
	r.Want(t, 0)
	r.Has(t, `"permissionDecision":"ask"`)
	k.commit("", `git commit -m "fix: add sums"; git commit -m "fix: x" -m "I hope this helps."`).Want(t, 2)
	k.commit("", `git commit -m "fix: add sums" && git commit -m "$MSG"`).Has(t, "rule  unread-text")
}

// Each case feeds guard-commit a command and asserts its status: 0 allow,
// 2 block. gitleaks is whatever PATH holds.
func TestGuardCommit(t *testing.T) {
	t.Parallel()
	shared := newSandbox(t)
	for _, c := range []struct {
		name, cmd string
		status    int
	}{
		// passthrough: hook only inspects git commit commands
		{"passthrough: ls", "ls -la", 0},
		{"passthrough: git status", "git status", 0},
		{"passthrough: git push", "git push origin main", 0},
		{"passthrough: a path starting with commit isn't a commit", "cat > docs/rules.md <<EOF\nNever add Co-Authored-By: Claude\nEOF\ngit add commitlint.config.js", 0},
		{"passthrough: commit as a word in a later command isn't a commit", "git add x\ncat > notes.md <<EOF\nWe commit to Co-Authored-By: Claude docs\nEOF", 0},
		// allow: normal commits
		{"allow: feat conventional commit", `git commit -m "feat: add foo"`, 0},
		{"allow: fix conventional commit", `git commit -m "fix: bar bug"`, 0},
		{"allow: chore with scope", `git commit -m "chore(claude): clean up settings"`, 0},
		{"allow: plain subject without prefix", `git commit -m "Update README with install steps"`, 0},
		{"allow: commit with single-quoted message", `git commit -m 'feat: add foo'`, 0},
		// block: AI-tell phrasing in subject
		{"block: certainly in subject after conventional prefix", `git commit -m "feat: certainly add foo"`, 2},
		{"block: here is in subject", `git commit -m "fix: here is the patch"`, 2},
		{"block: i have at start of subject", `git commit -m "I have refactored the loop"`, 2},
		{"block: let me at start of subject", `git commit -m "let me clean this up"`, 2},
		{"block: in this commit phrasing", `git commit -m "in this commit we add the API"`, 2},
		{"block: AI-tell phrasing in a clustered -am subject", `git commit -am "feat: certainly add foo"`, 2},
		{"block: AI-tell phrasing in a --message= subject", `git commit --message="let me clean this up"`, 2},
		{"block: AI-tell phrasing in a subject with an expansion", `git commit -m "Let me fix $X"`, 2},
		{"block: AI-tell phrasing when a later paragraph has a $", `git commit -m "let me fix it" -m "costs \$5"`, 2},
		// wall of text: only the heredoc opened on the git commit line is the message
		{"block: wall of text in a -m heredoc", "git commit -m \"$(cat <<'EOF'\nfeat: x\n\n" + prose(5) + "EOF\n)\"", 2},
		{"block: wall of text in a -F - heredoc", "git commit -F - <<'EOF'\nfeat: x\n\n" + prose(5) + "EOF", 2},
		{"block: wall of text in a heredoc with another delimiter", "git commit -m \"$(cat <<'MSG'\nfeat: x\n\n" + prose(5) + "MSG\n)\"", 2},
		{"block: wall of text in a <<- heredoc closed by a tab-indented delimiter", "git commit -F - <<-MSG\n\tfeat: x\n\n" + prose(5) + "\tMSG", 2},
		{"allow: wall of text in a heredoc writing a file", "cat > notes.md <<'EOF'\n" + prose(20) + "EOF\ngit add -A && git commit -qm init", 0},
		// subjects: the commit's heredoc is its message, another heredoc writes a file
		{"block: AI-tell phrasing in a heredoc subject", "git -C . commit -m \"$(cat <<'EOF'\nlet me fix this\nEOF\n)\"", 2},
		{"block: AI-tell phrasing in a <<- heredoc subject with a spaced opener", "git commit -F - <<- \"MSG\"\n\there is the patch\n\tMSG", 2},
		{"allow: a quoted -m subject before a heredoc body", "git commit -m \"feat: x\" -m \"$(cat <<'EOF'\nThe parser dropped the last line.\nEOF\n)\"", 0},
		{"block: AI-tell phrasing opening a heredoc body line", "git commit -m \"feat: x\" -m \"$(cat <<'EOF'\nlet me explain\nEOF\n)\"", 2},
		// slop: a tier in slop.block_tiers blocks anywhere in the message, the rest is logged
		{"block: a tier 1 tell in a -m body", `git commit -m "feat: x" -m "I hope this helps."`, 2},
		{"block: a tier 1 tell in a -F - heredoc body", "git commit -F - <<'EOF'\nfeat: x\n\nI hope this helps.\nEOF", 2},
		{"block: a tier 2 tell", `git commit -m "fix: x" -m "Addressed review feedback."`, 2},
		{"allow: a dense tier 3 passage is only logged", `git commit -m "fix: x" -m "A robust, meticulous, comprehensive tool."`, 0},
		{"allow: a tell quoted in the body", "git commit -m \"docs: x\" -m 'Ban the phrase \"I hope this helps\".'", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			shared.commit("", c.cmd).Want(t, c.status)
		})
	}
}

// The -F file is scanned for AI tells too; a blocked one names the tell and
// its slug, a logged one lands in the guards log with its rule.
func TestGuardCommitScansTheMessageForAITells(t *testing.T) {
	k := newSandbox(t)
	testutil.Write(t, filepath.Join(k.home, "msg.txt"), "fix: x\n\nI hope this helps.\n")
	r := k.commit("", "git commit -F msg.txt")
	r.Want(t, 2)
	r.Has(t, `L3    "I hope this helps" -> cut it`, "slop-patterns")

	k.commit("", `git commit -m "fix: x" -m "A robust, meticulous, comprehensive tool."`).Want(t, 0)
	log, _ := os.ReadFile(filepath.Join(k.claude, "logs", "guards.jsonl"))
	for _, want := range []string{"slop-hope-helps", "slop-vocabulary"} {
		if !strings.Contains(string(log), want) {
			t.Errorf("guards log lacks %s:\n%s", want, log)
		}
	}
}

// gitleaks scans the staged diff: exit 0 is clean, 1 a leak, anything else
// an operational error that fails open. The scan runs before the subject
// parsing, so a commit with an unquoted -m, or none, is still scanned, and
// git -C <dir> commit still counts as one.
func TestGuardCommitScansTheStagedDiff(t *testing.T) {
	for _, c := range []struct {
		name, cmd    string
		code, status int
		has          string
	}{
		{"allow: gitleaks exits 0 (clean scan)", `git commit -m "feat: add foo"`, 0, 0, ""},
		{"block: gitleaks exits 1 (leak found)", `git commit -m "feat: add foo"`, 1, 2, "gitleaks flagged a secret"},
		{"allow (fail open): gitleaks exits a non-0/1 operational error code", `git commit -m "feat: add foo"`, 2, 0, "gitleaks exited 2"},
		{"block: gitleaks leak is still caught with an unquoted -m message", "git commit -m foo", 1, 2, "gitleaks flagged a secret"},
		{"allow: unquoted -m message with a clean gitleaks scan still passes through", "git commit -m foo", 0, 0, ""},
		{"block: gitleaks leak is still caught with no -m at all (--allow-empty)", "git commit --allow-empty", 1, 2, ""},
		{"block: git -C <dir> commit is still caught by the entry guard (gitleaks leak)", "git -C " + t.TempDir() + " commit -m test", 1, 2, "gitleaks flagged a secret"},
		// gitleaks exits 1 on a path it can't stat, the same code as a leak.
		{"allow (fail open): git -C names a missing directory", "git -C " + filepath.Join(t.TempDir(), "missing") + " commit -m test", 1, 0, "skipping scan"},
		{"block: git -C ~ resolves against home and is scanned", "git -C ~ commit -m test", 1, 2, "gitleaks flagged a secret"},
		// Most expansions name the cwd's repo: "$PWD", "$(git rev-parse --show-toplevel)".
		{"block: git -C names an expansion, so the cwd's repo is scanned", `git -C "$PWD" commit -m test`, 1, 2, "gitleaks flagged a secret"},
	} {
		t.Run(c.name, func(t *testing.T) {
			gitleaksStub(t, c.code)
			r := newSandbox(t).commit("", c.cmd)
			r.Want(t, c.status)
			if c.has != "" {
				r.Has(t, c.has)
			}
		})
	}

	t.Run("allow: gitleaks not installed, scan is skipped silently", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		r := newSandbox(t).commit("", `git commit -m "feat: add foo"`)
		r.Want(t, 0)
		// No transcript shows the message, so it asks; nothing about gitleaks.
		if strings.Contains(r.Output, "gitleaks") {
			t.Errorf("gitleaks notice in output:\n%s", r.Output)
		}
	})

	t.Run("gitleaks is invoked against payload .cwd, not the hook's own cwd", func(t *testing.T) {
		calls := gitleaksStub(t, 0)
		fixture := filepath.Join(testutil.Physical(t, t.TempDir()), "fixture-repo")
		testutil.Mkdir(t, fixture)
		newSandbox(t).commit(fixture, `git commit -m "feat: add foo"`).Want(t, 0)
		if runs := strings.Join(testutil.Calls(t, calls), "\n"); !strings.Contains(runs, fixture) {
			t.Errorf("gitleaks runs %q lack %q", runs, fixture)
		}
	})

	// No $ in the name: the stub's calls path is under t.TempDir, in a shell string.
	t.Run("a commit that git -C or cd points at home is scanned at home, not the cwd", func(t *testing.T) {
		for _, cmd := range []string{"git -C $HOME commit -m test", "git -C ${HOME}/ commit -m test", "cd && git commit -m test", "cd $HOME; git commit -m test"} {
			calls := gitleaksStub(t, 0)
			k := newSandbox(t)
			k.commit(t.TempDir(), cmd).Want(t, 0)
			if runs := strings.Join(testutil.Calls(t, calls), "\n"); !strings.Contains(runs, k.home) {
				t.Errorf("%s: gitleaks runs %q lack %q", cmd, runs, k.home)
			}
		}
	})
}

func TestShown(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		text, subject string
		want          bool
	}{
		{"Commit as `fix: x`?", "fix: x", true},
		{"1. fix: x", "fix: x", true},
		{"- 1.2 release notes", "1.2 release notes", true},
		{"1.2 release notes", "1.2 release notes", true},
		{"> **fix: x**", "fix: x", true},
		{"1. fix: x, then the docs", "fix: x", false},
		{"wipe the cache", "wip", false},
	} {
		if got := shown(tc.text, tc.subject); got != tc.want {
			t.Errorf("shown(%q, %q) = %v, want %v", tc.text, tc.subject, got, tc.want)
		}
	}
}
