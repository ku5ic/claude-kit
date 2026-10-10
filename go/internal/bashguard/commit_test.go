package bashguard

import (
	"os"
	"path/filepath"
	"testing"
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
	write(t, filepath.Join(dir, "sub", "msg.txt"), "fix: from a file\n")
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
	write(t, filepath.Join(repo, "package.json"), `{"commitlint":{"extends":["@commitlint/config-conventional"]}}`)
	write(t, filepath.Join(repo, ".gitignore"), "node_modules\n")
	bin := filepath.Join(repo, "node_modules", ".bin", "commitlint")
	write(t, bin, "#!/bin/sh\nhead -1 \"$2\" | grep -Eq '^(feat|fix)(\\(.+\\))?: ' || { echo 'type must be one of [feat, fix] [type-enum]'; exit 1; }\n")
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
