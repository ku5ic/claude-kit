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
		if got := committing(cmd); got != want {
			t.Errorf("committing(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestMessageIsWhatTheCommitMsgHookWouldRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.Write(t, filepath.Join(dir, "sub", "msg.txt"), "fix: from a file\n")
	for cmd, want := range map[string]string{
		"git commit -m \"$(cat <<'EOF'\nfeat: x\n\nbody\nEOF\n)\"": "feat: x\n\nbody\n",
		"git commit -F - <<'EOF'\nfix: y\nEOF":                     "fix: y\n",
		`git commit -m "feat: a" -m "body"`:                        "feat: a\n\nbody\n",
		`git commit -am "feat: cluster"`:                           "feat: cluster\n",
		`git commit --message="feat: long"`:                        "feat: long\n",
		`git -C sub commit -F msg.txt`:                             "fix: from a file\n",
		`git commit -m "$MSG"`:                                     "",
		`git commit --amend --no-edit`:                             "",
	} {
		if got, _ := message(cmd, dir); got != want {
			t.Errorf("message(%q) = %q, want %q", cmd, got, want)
		}
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
	k.commit(repo, `git commit -m "$MSG"`).Want(t, 0)
}

func TestTheCommitMsgCheckIsTheRepoGitDashCNames(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	repo := commitlintRepo(t, k)
	plain := gitRepo(t, filepath.Join(k.home, "plain"), "main")
	k.commit(plain, `git -C ../repo commit -m "updated stuff"`).Want(t, 2)
	k.commit(repo, `git -C ../plain commit -m "updated stuff"`).Want(t, 0)
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

// rules/workflow.md section 1: a commit's message is shown before it runs,
// so a commit whose subject no earlier turn showed gets a prompt.
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
		"no transcript":           nil,
		"never shown":             {testutil.UserPrompt("fix it and commit")},
		"shown only in this turn": {testutil.UserPrompt("fix it and commit"), testutil.AssistantText("Committing `fix: add sums`.")},
	} {
		r := commit(transcript...)
		r.Want(t, 0)
		if !strings.Contains(r.Output, `"permissionDecision":"ask"`) {
			t.Errorf("%s: no ask; output:\n%s", name, r.Output)
		}
	}
	commit(testutil.UserPrompt("fix it"), testutil.AssistantText("Commit as `fix: add sums`?"), testutil.UserPrompt("go")).Empty(t)
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
		{"block: a commit ended by a separator is still a commit", "git commit;echo \"Co-Authored-By: Claude\"", 2},
		{"block: a commit with its output redirected is still a commit", "git commit>out.log -m \"x\n\nCo-Authored-By: Claude\"", 2},
		{"passthrough: commit as a word in a later command isn't a commit", "git add x\ncat > notes.md <<EOF\nWe commit to Co-Authored-By: Claude docs\nEOF", 0},
		// allow: normal commits
		{"allow: feat conventional commit", `git commit -m "feat: add foo"`, 0},
		{"allow: fix conventional commit", `git commit -m "fix: bar bug"`, 0},
		{"allow: chore with scope", `git commit -m "chore(claude): clean up settings"`, 0},
		{"allow: plain subject without prefix", `git commit -m "Update README with install steps"`, 0},
		{"allow: commit with single-quoted message", `git commit -m 'feat: add foo'`, 0},
		// block: AI signatures in body
		{"block: Co-Authored-By Claude", `git commit -m "feat: x" -m "Co-Authored-By: Claude <claude@anthropic.com>"`, 2},
		{"block: Generated by Claude", `git commit -m "Generated by Claude"`, 2},
		{"block: Generated with Claude", `git commit -m "Generated with Claude"`, 2},
		// block: AI-tell phrasing in subject
		{"block: certainly in subject after conventional prefix", `git commit -m "feat: certainly add foo"`, 2},
		{"block: here is in subject", `git commit -m "fix: here is the patch"`, 2},
		{"block: i have at start of subject", `git commit -m "I have refactored the loop"`, 2},
		{"block: let me at start of subject", `git commit -m "let me clean this up"`, 2},
		{"block: in this commit phrasing", `git commit -m "in this commit we add the API"`, 2},
		// wall of text: only the heredoc opened on the git commit line is the message
		{"block: wall of text in a -m heredoc", "git commit -m \"$(cat <<'EOF'\nfeat: x\n\n" + prose(5) + "EOF\n)\"", 2},
		{"block: wall of text in a -F - heredoc", "git commit -F - <<'EOF'\nfeat: x\n\n" + prose(5) + "EOF", 2},
		{"block: wall of text in a heredoc with another delimiter", "git commit -m \"$(cat <<'MSG'\nfeat: x\n\n" + prose(5) + "MSG\n)\"", 2},
		{"block: wall of text in a <<- heredoc closed by a tab-indented delimiter", "git commit -F - <<-MSG\n\tfeat: x\n\n" + prose(5) + "\tMSG", 2},
		{"allow: wall of text in a heredoc writing a file", "cat > notes.md <<'EOF'\n" + prose(20) + "EOF\ngit add -A && git commit -qm init", 0},
		// signatures and subjects: the commit's heredoc is its message, another heredoc writes a file
		// the whole command is scanned: a message can come from anywhere in it
		{"block: a signature in a heredoc writing a file, even an unrelated one", "cat > notes.md <<EOF\nGenerated with Claude\nEOF\ngit commit -m \"feat: ok\"", 2},
		{"block: a signature in a message file a heredoc writes for -F", "cat > /tmp/msg <<'EOF'\nfeat: x\n\nCo-Authored-By: Claude <c@a.com>\nEOF\ngit commit -F /tmp/msg", 2},
		{"block: a signature in a heredoc opened on a continuation line", "git commit \\\n  -F - <<'EOF'\nfeat: x\n\nCo-Authored-By: Claude <c@a.com>\nEOF", 2},
		{"block: a signature after a here-string", "v=$(tr a-z A-Z <<< hello)\ngit commit -m \"feat: x\n\nCo-Authored-By: Claude <c@a.com>\"", 2},
		{"block: a signature after an arithmetic shift", "n=$((1<<b))\ngit commit -m \"feat: x\n\nCo-Authored-By: Claude <c@a.com>\"", 2},
		{"block: a signature in an unterminated commit heredoc", "git commit -m \"$(cat <<EOF\nfeat: x\nGenerated with Claude\n)\"", 2},
		{"block: a signature in a -m variable set earlier in the command", "MSG=\"Generated with Claude\"\ngit commit -m \"$MSG\"", 2},
		{"block: AI-tell phrasing in a heredoc subject", "git -C . commit -m \"$(cat <<'EOF'\nlet me fix this\nEOF\n)\"", 2},
		{"block: AI-tell phrasing in a <<- heredoc subject with a spaced opener", "git commit -F - <<- \"MSG\"\n\there is the patch\n\tMSG", 2},
		{"allow: a quoted -m subject before a heredoc body", "git commit -m \"feat: x\" -m \"$(cat <<'EOF'\nlet me explain\nEOF\n)\"", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			shared.commit("", c.cmd).Want(t, c.status)
		})
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
		{"allow: plain git commit (no -C) is unaffected by the -C entry guard fix", "git commit -m test", 0, 0, ""},
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
}
