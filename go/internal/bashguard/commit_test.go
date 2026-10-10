package bashguard

import "testing"

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
