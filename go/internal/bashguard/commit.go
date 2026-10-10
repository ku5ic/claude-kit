package bashguard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
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
	messageDQ    = regexp.MustCompile(`(-m|--message=?)[[:space:]]*"[^"]*"`)
	messageSQ    = regexp.MustCompile(`(-m|--message=?)[[:space:]]*'[^']*'`)
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
	if !committing(cmd) {
		return nil
	}

	// Line by line, as grep: the trailer sits on a line of its own. The
	// whole command, since a message can come from anywhere in it (a file a
	// heredoc writes, then -F); a signature in an unrelated heredoc blocks too.
	for line := range strings.SplitSeq(cmd, "\n") {
		if aiSignature.MatchString(line) {
			if err := h.Block("AI signature in commit message", "ai-commit-sig"); err != nil {
				return err
			}
			break
		}
	}

	// Before the subject parsing: secret scanning must not depend on
	// whether -m used a quoted message.
	if err := scanStaged(h); err != nil {
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

	subject := subject(cmd, body)
	if subject != "" && aiTell.MatchString(subject) {
		return h.Block("AI-tell phrasing in commit subject", "ai-commit-tell")
	}
	if failed := commitMsg(h, cmd); failed != "" {
		return h.Block("the project's commit-msg checks reject this message; fix it to their convention:\n"+failed, "commit-msg")
	}
	if !shownBefore(h.Payload.String("transcript_path"), subject) {
		h.Decide("ask", "rules/workflow.md section 1: show the commit message and staged diff summary, then wait for the user's go; confirm only if they've seen this message")
	}
	return nil
}

// committing reports whether cmd runs git commit: a parsed call whose real
// command is git with commit as its subcommand. Where part of cmd didn't
// parse, the raw text is matched instead, so a commit whose heredoc never
// closes still counts.
func committing(cmd string) bool {
	_, _, found, unparsed := commitCall(cmd)
	return found || unparsed && commitLine.MatchString(cmd)
}

// commitCall is the args and -C directory of cmd's first parsed git commit
// call; found is false when none commits, unparsed true when part of cmd
// didn't parse.
func commitCall(cmd string) (args []string, dir string, found, unparsed bool) {
	segs, unparsed := parseAll(cmd)
	for _, seg := range segs {
		for _, call := range seg.Calls {
			i := lead(call.Words)
			if i >= len(call.Words) || baseName(call.Words[i].Value) != "git" {
				continue
			}
			var words []string
			for _, w := range call.Words[i+1:] {
				words = append(words, w.Value)
			}
			if sub, args, dir := gitSubcommand(words); sub == "commit" {
				return args, dir, true, unparsed
			}
		}
	}
	return nil, "", false, unparsed
}

// message is the message cmd's git commit gives, as the commit-msg hook
// would read it: the commit line's heredoc, else its -m messages as
// paragraphs, else the file -F names, from dir. ok is false when it can't
// be told: no message, or one an expansion builds.
func message(cmd, dir string) (msg string, ok bool) {
	if body := heredocBody(cmd); body != "" {
		return body + "\n", true
	}
	args, cdir, found, _ := commitCall(cmd)
	if !found {
		return "", false
	}
	var paragraphs []string
	file := ""
	commitOptions(args, func(opt, value string) {
		switch opt {
		case "-m", "--message":
			paragraphs = append(paragraphs, value)
		case "-F", "--file":
			file = value
		}
	})
	switch {
	case len(paragraphs) > 0 && !strings.Contains(strings.Join(paragraphs, ""), "$"):
		return strings.Join(paragraphs, "\n\n") + "\n", true
	case len(paragraphs) == 0 && file != "" && file != "-":
		data, err := os.ReadFile(fsx.Abs(fsx.Abs(dir, cdir), file))
		return string(data), err == nil
	}
	return "", false
}

// commitMsgBudget bounds the project's commit-msg checks, inside
// bash-dispatch's timeout in hooks.json, gitleaks' share apart.
const commitMsgBudget = 12 * time.Second

// commitMsg runs the commit-msg enforcement of the repo cmd commits to (its
// git -C directory, else the cwd) on the message, from a temp file, and
// returns what failed; "" when it passes, the message can't be told, or the
// project has none.
func commitMsg(h *hook.Hook, cmd string) string {
	cwd := h.Payload.Cwd()
	_, cdir, _, _ := commitCall(cmd)
	root, cfg := git.Toplevel(fsx.Abs(cwd, cdir)), h.Config()
	msg, ok := message(cmd, cwd)
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

// scanStaged runs gitleaks on the staged diff. Its exit codes: 0 clean, 1 a
// leak, anything else an operational error that fails open with a notice.
func scanStaged(h *hook.Hook) error {
	if _, err := exec.LookPath("gitleaks"); err != nil {
		return nil
	}
	err := proc.Command(proc.Quick, "gitleaks", "git", "--staged", "--no-banner", "--redact", "--log-level", "error", h.Payload.Cwd()).Run()
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
// scan covers it, the wall-of-text and subject checks don't. One never
// closed runs to the end, as the shell reads it.
func heredocBody(cmd string) string {
	var lines []string
	delim, tabs := "", false
	for line := range strings.SplitSeq(cmd, "\n") {
		switch {
		case delim != "" && closes(line, delim, tabs):
			return strings.Join(lines, "\n")
		case delim != "":
			lines = append(lines, line)
		case commitLine.MatchString(line):
			if m := heredocOpen.FindStringSubmatch(line); m != nil {
				delim, tabs = m[2], m[1] == "-"
			}
		}
	}
	return strings.Join(lines, "\n")
}

// closes is true when line ends a heredoc delimited by delim; with tabs
// (<<-), leading tabs are stripped first.
func closes(line, delim string, tabs bool) bool {
	return line == delim || tabs && strings.TrimLeft(line, "\t") == delim
}

// subject is the commit message's first line: the commit heredoc's when
// it opens before any quoted -m on the commit line, else the first quoted
// -m's.
func subject(cmd, body string) string {
	for line := range strings.SplitSeq(cmd, "\n") {
		if !commitLine.MatchString(line) {
			continue
		}
		open := heredocOpen.FindStringIndex(line)
		quoted := firstQuoted(line)
		if body != "" && open != nil && (quoted < 0 || open[0] < quoted) {
			first, _, _ := strings.Cut(strings.TrimLeft(body, "\n"), "\n")
			return strings.TrimLeft(first, " \t")
		}
		break
	}
	first, _, _ := strings.Cut(quotedMessages(cmd, messageDQ, '"')+quotedMessages(cmd, messageSQ, '\''), "\n")
	return first
}

// firstQuoted is where line's first quoted -m message starts, or -1. A
// command substitution ("$(cat <<'EOF'") isn't one.
func firstQuoted(line string) int {
	first := -1
	for _, re := range []*regexp.Regexp{messageDQ, messageSQ} {
		for _, loc := range re.FindAllStringIndex(line, -1) {
			if !strings.Contains(line[loc[0]:loc[1]], "$(") && (first < 0 || loc[0] < first) {
				first = loc[0]
			}
		}
	}
	return first
}

// quotedMessages is `$(grep -oE '<re>' | sed 's/.*<q>([^<q>]*)<q>/\1/')`:
// the quoted text of every -m/--message match, one per line, matched line by
// line, with no trailing newline (command substitution strips it, so the
// double- and single-quoted results concatenate directly). A command
// substitution's opening ("$(cat << ") isn't a message.
func quotedMessages(cmd string, re *regexp.Regexp, quote byte) string {
	var out []string
	for line := range strings.SplitSeq(cmd, "\n") {
		for _, match := range re.FindAllString(line, -1) {
			inner := strings.TrimSuffix(match, string(quote))
			if msg := inner[strings.LastIndexByte(inner, quote)+1:]; !strings.HasPrefix(msg, "$(") {
				out = append(out, msg)
			}
		}
	}
	return strings.Join(out, "\n")
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
