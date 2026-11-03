package bashguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// Each case runs guard-pr from a cwd holding tell.md (a tier 1 tell),
// clean.md, and tell.json and clean.json request bodies, against the real
// kit.yml.
func TestGuardPR(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	cwd := testutil.Physical(t, t.TempDir())
	testutil.Write(t, filepath.Join(cwd, "tell.md"), "## Summary\n\nI hope this helps.\n")
	testutil.Write(t, filepath.Join(cwd, "clean.md"), "## Summary\n\nReads the body from disk.\n")
	testutil.Write(t, filepath.Join(cwd, "tell.json"), `{"title": "fix: x", "body": "I hope this helps."}`)
	testutil.Write(t, filepath.Join(cwd, "clean.json"), `{"body": "Reads the body from disk."}`)
	for _, c := range []struct {
		name, cmd string
		status    int
	}{
		{"passthrough: not gh", `echo "I hope this helps."`, 0},
		{"passthrough: gh pr view", `gh pr view 3 --json body`, 0},
		{"block: gh issue create -b", `gh issue create -t x -b "I hope this helps."`, 2},
		{"block: gh issue comment --body-file", `gh issue comment 3 --body-file tell.md`, 2},
		{"passthrough: gh issue view", `gh issue view 3 --json body`, 0},
		{"block: gh issue close --comment", `gh issue close 3 -r "not planned" --comment "I hope this helps."`, 2},
		{"block: gh pr reopen -c", `gh pr reopen 3 -c "I hope this helps."`, 2},
		{"block: gh pr merge --body", `gh pr merge 3 --squash --body "I hope this helps."`, 2},
		{"block: gh pr merge -t subject", `gh pr merge 3 -s -t "I hope this helps"`, 2},
		{"block: pr review's bool -c leaves -b read", `gh pr review 3 -c -b "I hope this helps."`, 2},
		{"passthrough: gh issue review isn't a subcommand", `gh issue review 3 -b "I hope this helps."`, 0},
		{"allow: a clean body", `gh pr create -t "fix: x" -b "Reads the body from disk."`, 0},
		{"block: --body", `gh pr create --title "fix: x" --body "I hope this helps."`, 2},
		{"block: -b", `gh pr comment 3 -b "I hope this helps."`, 2},
		{"block: --body=", `gh pr review 3 --approve --body="I hope this helps."`, 2},
		{"block: a tell in the title", `gh pr edit 3 --title "I hope this helps"`, 2},
		{"block: a $(cat <<EOF) body", "gh pr create -t x -b \"$(cat <<'EOF'\n## Summary\n\nI hope this helps.\nEOF\n)\"", 2},
		{"block: --body-file", `gh pr edit 3 --body-file tell.md`, 2},
		{"allow: --body-file clean", `gh pr edit 3 --body-file clean.md`, 0},
		{"block: -F with a cd first", "cd .. && cd " + filepath.Base(cwd) + " && gh pr edit 3 -F tell.md", 2},
		{"block: -F - from a heredoc", "gh pr comment 3 -F - <<'EOF'\nI hope this helps.\nEOF", 2},
		{"block: -F - from a < file", `gh pr comment 3 -F - < tell.md`, 2},
		{"allow (logged): -F - from a pipe", `cat tell.md | gh pr comment 3 -F -`, 0},
		{"allow (logged): a file the command writes first", "printf x > tell.md && gh pr edit 3 -F tell.md", 0},
		{"allow (logged): a missing file", `gh pr edit 3 -F missing.md`, 0},
		{"allow (logged): a file a shell heredoc writes first", "bash <<'EOF'\nprintf x > tell.md\ngh pr edit 3 -F tell.md\nEOF", 0},
		{"allow (logged): a body built by $(cat file)", `gh pr create -t x -b "$(cat tell.md)"`, 0},
		{"allow (logged): a body in a variable", `gh pr comment 3 -b "$BODY"`, 0},
		{"block: a tell next to a literal dollar amount", `gh pr comment 3 -b "Costs $5. I hope this helps."`, 2},
		{"allow: the tell quoted in the body", `gh pr comment 3 -b 'Bans "I hope this helps".'`, 0},
		{"allow: a dense tier 3 passage is only logged", `gh pr comment 3 -b "A robust, seamless, comprehensive tool."`, 0},
		{"block: gh api -f body=", `gh api repos/o/r/pulls/3 -X PATCH -f body="I hope this helps."`, 2},
		{"block: gh api --raw-field body=", `gh api repos/o/r/issues/3/comments --raw-field "body=I hope this helps."`, 2},
		{"block: gh api -F body=text", `gh api repos/o/r/issues/3/comments -F "body=I hope this helps."`, 2},
		{"block: gh api -F body=@file", `gh api repos/o/r/pulls/3 -X PATCH -F body=@tell.md`, 2},
		{"allow: gh api -f body=@file is the literal text", `gh api repos/o/r/pulls/3 -X PATCH -f body=@tell.md`, 0},
		{"allow: gh api -F body=@clean file", `gh api repos/o/r/pulls/3 -X PATCH -F body=@clean.md`, 0},
		{"allow (logged): gh api -F body=@-", `cat tell.md | gh api repos/o/r/pulls/3 -X PATCH -F body=@-`, 0},
		{"allow: gh api another field", `gh api repos/o/r/pulls/3 -X PATCH -f state="I hope this helps."`, 0},
		{"block: gh api --input JSON body", `gh api repos/o/r/pulls/3 -X PATCH --input tell.json`, 2},
		{"allow: gh api --input clean JSON", `gh api repos/o/r/pulls/3 -X PATCH --input clean.json`, 0},
		{"allow (logged): gh api --input that isn't JSON", `gh api repos/o/r/pulls/3 -X PATCH --input tell.md`, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := k.pr(cwd, c.cmd)
			r.Want(t, c.status)
			if c.status == 2 {
				r.Has(t, "I hope this helps", "slop-patterns")
			}
		})
	}
}

// What can't be read is logged with its source, so the shadow review sees
// it; a block names the source.
func TestGuardPRLogsWhatItCantRead(t *testing.T) {
	k := newSandbox(t)
	cwd := t.TempDir()
	k.pr(cwd, `cat body.md | gh api repos/o/r/pulls/3 -F body=@-`).Want(t, 0)
	log, _ := os.ReadFile(filepath.Join(k.claude, "logs", "guards.jsonl"))
	for _, want := range []string{`"slop-unread"`, `"gh api body field"`, "stdin"} {
		if !strings.Contains(string(log), want) {
			t.Errorf("guards log lacks %s:\n%s", want, log)
		}
	}
	k.pr(cwd, `gh pr create -t x -b "I hope this helps."`).Has(t, "gh pr create --body")
	k.pr(cwd, `gh pr create -t x -b "$(cat pr.md)"`).Want(t, 0)
	if log, _ := os.ReadFile(filepath.Join(k.claude, "logs", "guards.jsonl")); !strings.Contains(string(log), "an expansion builds the text") {
		t.Errorf("guards log lacks the unread body:\n%s", log)
	}
}
