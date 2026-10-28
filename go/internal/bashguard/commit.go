package bashguard

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/md"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/run"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

var (
	// git, any global options, then commit as a word, all in one simple
	// command: the gap crosses no separator or newline but a line continuation.
	commitLine   = regexp.MustCompile(`\bgit(?:[ \t]|\\\n)+(?:[^[:space:];&|]+(?:[ \t]|\\\n)+)*commit(?:[^-[:alnum:]_.]|$)`)
	aiSignature  = regexp.MustCompile(`(?i)Co-Authored-By:[[:space:]]*Claude|Generated[[:space:]]+(by|with)[[:space:]]+Claude|🤖[[:space:]]*Generated`)
	heredocOpen  = regexp.MustCompile(`<<(-?)[[:space:]]*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)
	aiTell       = regexp.MustCompile(`(?i)^(feat|fix|chore|refactor|docs|test|perf|build|ci|style)?:?[[:space:]]*(certainly|here is|i have|let me|in this commit|this commit)`)
	nonProseLine = regexp.MustCompile(`^[[:space:]]*([0-9]+[.)]|[-*+][[:space:]]|#{1,6}[[:space:]]|>|\|)`)
)

// CheckCommit is guard-commit: it inspects git commit commands for AI signatures, a staged
// secret (gitleaks), an unchunked wall of text in the body (rules/output.md
// section 1), and AI-tell phrasing in the subject.
func CheckCommit(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	cmd := h.Payload.String("tool_input.command")
	c := commitCall(cmd, h.Home, h.Payload.Cwd())
	if !c.commits(cmd) {
		return nil
	}
	// An expansion other than $HOME can't be resolved here; most name the
	// cwd's repo ("$PWD", "$(git rev-parse --show-toplevel)").
	repo := cmp.Or(c.repo, h.Payload.Cwd())

	// Line by line, as grep: the trailer sits on a line of its own. The
	// whole command, since a message can come from anywhere in it (a file a
	// heredoc writes, then -F); a signature in an unrelated heredoc blocks
	// too. Then the message, for a file -F reads from disk.
	msg, ok := message(cmd, c, repo, h.Home)
	for line := range strings.SplitSeq(cmd+"\n"+msg, "\n") {
		if aiSignature.MatchString(line) {
			if err := h.Block("AI signature in commit message", "ai-commit-sig"); err != nil {
				return err
			}
			break
		}
	}

	// Before the subject parsing: secret scanning must not depend on
	// whether -m used a quoted message.
	if err := scanStaged(h, repo); err != nil {
		return err
	}

	// This repo passes multi-line messages via a heredoc
	// (-m "$(cat <<'EOF' ... EOF)"). A single-line -m has no heredoc and is
	// skipped: short enough that a miss is harmless.
	body := heredocBody(cmd)
	if body != "" {
		if run := longestProseRun(body); run > 4 {
			reason := fmt.Sprintf("commit message has an unchunked wall of text (%d consecutive prose lines). rules/output.md section 1: short paragraphs, no dense blocks.", run)
			if err := h.Block(reason, "commit-wall-of-text"); err != nil {
				return err
			}
		}
	}

	subject := subject(msg)
	if subject != "" && aiTell.MatchString(subject) {
		return h.Block("AI-tell phrasing in commit subject", "ai-commit-tell")
	}
	if failed := commitMsg(h, msg, ok, repo); failed != "" {
		return h.Block("the project's commit-msg checks reject this message; fix it to their convention:\n"+failed, "commit-msg")
	}
	if !shownBefore(h.Payload.String("transcript_path"), subject) {
		h.Decide("ask", "rules/workflow.md section 1: show the commit message and staged diff summary, then wait for the user's go; confirm only if they've seen this message")
	}
	return nil
}

// commit is the args and repo of a command's first parsed git commit call;
// found is false when none commits, unparsed true when part of the command
// didn't parse.
type commit struct {
	args []string
	// repo is the directory it commits in: its git -C directory, against
	// where the cds before it moved, "" when an expansion names it.
	repo            string
	found, unparsed bool
	start           int // source offset of the git call
}

// commits reports whether cmd, parsed as c, runs git commit: a parsed call
// whose real command is git with commit as its subcommand. Where part of
// cmd didn't parse, the raw text is matched instead, so a commit whose
// heredoc never closes still counts.
func (c commit) commits(cmd string) bool {
	return c.found || c.unparsed && commitLine.MatchString(cmd)
}

// commitCall parses cmd, run from cwd, for its first git commit call.
func commitCall(cmd, home, cwd string) commit {
	segs, unparsed := parseAll(cmd)
	d := dirs{home: home, cwd: cwd}
	for _, seg := range segs {
		for _, call := range seg.Calls {
			i := lead(call.Words)
			if i >= len(call.Words) {
				continue
			}
			name := baseName(call.Words[i].Value)
			// In a pipeline or a subshell, cd moves nothing after it.
			if name == "cd" && len(seg.Calls) == 1 && !seg.subshell {
				d.cd(call.Words[i+1:])
			}
			if name != "git" {
				continue
			}
			var words []string
			for _, w := range call.Words[i+1:] {
				words = append(words, w.Value)
			}
			if sub, args, dir := classify.GitSubcommand(words); sub == "commit" {
				repo := resolveDir(home, d.cwd, dir)
				if strings.ContainsAny(repo, "$`") {
					repo = ""
				}
				return commit{args: args, repo: repo, found: true, unparsed: unparsed, start: call.start}
			}
		}
	}
	return commit{unparsed: unparsed}
}

// message is the message cmd's git commit c gives, as the commit-msg hook
// would read it: its -m messages as paragraphs, a $(cat <<EOF) one read as
// its body, else the file -F names, from repo or under home, unless the command names it
// before the commit, or the commit line's heredoc
// for -F - or a command that didn't parse. ok is false when it can't be
// told exactly: no message, or one an expansion builds, whose -m text is
// still msg, as written, for the subject checks.
func message(cmd string, c commit, repo, home string) (msg string, ok bool) {
	if !c.found {
		body := heredocBody(cmd)
		return body + "\n", body != ""
	}
	var paragraphs []string
	file, expanded := "", false
	commitOptions(c.args, func(opt, value string) {
		switch opt {
		case "-m", "--message":
			if body, ok := catHeredoc(value); ok {
				value = body
			} else if strings.Contains(value, "$") {
				expanded = true
			}
			paragraphs = append(paragraphs, value)
		case "-F", "--file":
			file = value
		}
	})
	switch {
	case len(paragraphs) > 0:
		return strings.Join(paragraphs, "\n\n") + "\n", !expanded
	case len(paragraphs) == 0 && file == "-":
		body := heredocBody(cmd)
		return body + "\n", body != ""
	// Named before the commit, the file is likely written there first:
	// what's on disk now is a stale message. Named after (an rm) it isn't.
	case len(paragraphs) == 0 && file != "" && !strings.Contains(cmd[:c.start], filepath.Base(file)):
		data, err := os.ReadFile(fsx.Abs(repo, guard.ExpandHome(home, file)))
		return string(data), err == nil
	}
	return "", false
}

// catHeredoc is the body of a -m value that is one $(cat <<EOF ... EOF)
// substitution.
func catHeredoc(value string) (string, bool) {
	head, rest, ok := strings.Cut(value, "\n")
	if !ok || !strings.HasPrefix(head, "$(cat ") || !strings.HasSuffix(value, ")") {
		return "", false
	}
	m := heredocOpen.FindStringSubmatch(head)
	if m == nil {
		return "", false
	}
	return readHeredoc(rest, m[2], m[1] == "-")
}

// commitMsgBudget bounds the project's commit-msg checks, inside
// bash-dispatch's timeout in hooks.json, gitleaks' share apart.
const commitMsgBudget = 12 * time.Second

// commitMsg runs the commit-msg enforcement of repo on msg, from a temp
// file, and returns what failed; "" when it passes, the message can't be
// told (!ok), or the project has none.
func commitMsg(h *hook.Hook, msg string, ok bool, repo string) string {
	root, cfg := git.Toplevel(repo), h.Config()
	if !ok || root == "" || cfg == nil {
		return ""
	}
	tmp, err := os.CreateTemp("", "kit-commit-msg-*")
	if err != nil {
		return ""
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, err = tmp.WriteString(msg)
	if cerr := tmp.Close(); err != nil || cerr != nil {
		return ""
	}
	p := enforce.CommitMsg(cfg, enforce.Options{Root: root, CacheDir: h.Paths.CacheDir()}, tmp.Name())
	return run.Commit(p, commitMsgBudget)
}

// shownBefore reports whether an assistant reply before the current prompt
// held subject: the message was shown, and the user answered.
func shownBefore(path, subject string) bool {
	if path == "" || subject == "" {
		return false
	}
	var earlier, current strings.Builder
	_ = transcript.Each(path, func(e transcript.Entry) {
		switch {
		case e.StartsTurn():
			earlier.WriteString(current.String())
			current.Reset()
		case e.Type == "assistant":
			current.WriteString(e.PromptText() + "\n")
		}
	})
	return strings.Contains(earlier.String(), subject)
}

// scanStaged runs gitleaks on repo's staged diff. Its exit codes: 0 clean, 1 a
// leak, anything else an operational error that fails open with a notice.
func scanStaged(h *hook.Hook, repo string) error {
	if _, err := exec.LookPath("gitleaks"); err != nil {
		return nil
	}
	// gitleaks exits 1, a leak's code, on a path it can't stat: a missing
	// git -C directory.
	if !fsx.IsDir(repo) {
		fmt.Fprintf(h.Stderr, "%s: %s is not a directory; skipping scan\n", h.Name, repo)
		return nil
	}
	err := proc.Command(proc.Quick, "gitleaks", "git", "--staged", "--no-banner", "--redact", "--log-level", "error", repo).Run()
	if err == nil {
		return nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		if exit.ExitCode() == 1 {
			return h.Block("gitleaks flagged a secret in the staged diff", "staged-secret")
		}
		fmt.Fprintf(h.Stderr, "%s: gitleaks exited %d (not a leak signal); skipping scan\n", h.Name, exit.ExitCode())
		return nil
	}
	fmt.Fprintf(h.Stderr, "%s: gitleaks failed (%v); skipping scan\n", h.Name, err)
	return nil
}

// heredocBody is the message between the opener and the closing delimiter
// of the heredoc opened on the git commit line (-m "$(cat <<'EOF'", -F -
// <<'MSG'). A message from elsewhere (a file a heredoc writes for -F, a
// heredoc opened on a continuation line) isn't read here; the signature
// scan covers it, the wall-of-text check doesn't. One never
// closed runs to the end, as the shell reads it.
func heredocBody(cmd string) string {
	for rest := cmd; rest != ""; {
		line, after, _ := strings.Cut(rest, "\n")
		if m := heredocOpen.FindStringSubmatch(line); m != nil && commitLine.MatchString(line) {
			body, _ := readHeredoc(after, m[2], m[1] == "-")
			return body
		}
		rest = after
	}
	return ""
}

// readHeredoc is text's lines up to the one closing delim; ok is false when
// none does.
func readHeredoc(text, delim string, tabs bool) (string, bool) {
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		if closes(line, delim, tabs) {
			return strings.Join(lines, "\n"), true
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), false
}

// closes is true when line ends a heredoc delimited by delim; with tabs
// (<<-), leading tabs are stripped first.
func closes(line, delim string, tabs bool) bool {
	return line == delim || tabs && strings.TrimLeft(line, "\t") == delim
}

// subject is msg's first non-blank line, as git takes it.
func subject(msg string) string {
	for line := range strings.SplitSeq(msg, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// longestProseRun is the longest run of consecutive non-blank lines that
// aren't list items, headings, blockquotes, or table rows, outside fenced
// code and YAML frontmatter: a deterministic stand-in for rules/output.md
// section 1. Frontmatter is skipped because its key: value lines would read
// as a wall, blocking every agent and skill header.
func longestProseRun(text string) int {
	best, run := 0, 0
	for l := range md.Lines(text) {
		line := strings.TrimRight(l.Text, "\r\n")
		switch {
		case l.Kind != md.Prose:
		case strings.TrimSpace(line) == "":
			run = 0
		case nonProseLine.MatchString(line):
			run = 0
		default:
			run++
			best = max(best, run)
		}
	}
	return best
}
