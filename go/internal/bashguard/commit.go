package bashguard

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/md"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/resolve/kittool"
	"github.com/ku5ic/claude-kit/go/internal/run"
	"github.com/ku5ic/claude-kit/go/internal/slop"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

var (
	// git, any global options, then commit as a word, all in one simple
	// command: the gap crosses no separator or newline but a line continuation.
	commitLine   = regexp.MustCompile(`\bgit(?:[ \t]|\\\n)+(?:[^[:space:];&|]+(?:[ \t]|\\\n)+)*commit(?:[^-[:alnum:]_.]|$)`)
	aiSignature  = regexp.MustCompile(`(?i)Co-Authored-By:[[:space:]]*Claude|Generated[[:space:]]+(by|with)[[:space:]]+Claude|🤖[[:space:]]*Generated`)
	heredocOpen  = regexp.MustCompile(`<<(-?)[[:space:]]*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)
	nonProseLine = regexp.MustCompile(`^[[:space:]]*([0-9]+[.)]|[-*+][[:space:]]|#{1,6}[[:space:]]|>|\|)`)
)

// CheckCommit is guard-commit: it inspects git commit commands for AI signatures, a staged
// secret (gitleaks), an unchunked wall of text in the body (skills/kit-rules/reference/output.md
// section 1), and AI tells in the message (kit.yml slop).
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
			reason := fmt.Sprintf("a commit message paragraph runs %d prose lines\n", run) +
				layout.KV("do", "split it into paragraphs of 4 lines or fewer") + "\n" +
				layout.KV("how", "skills/kit-rules/reference/output.md section 1")
			if err := h.Block(reason, "commit-wall-of-text"); err != nil {
				return err
			}
		}
	}

	subject := subject(msg)
	if err := scanSlop(h, detector(h), msg, "commit", "the commit message"); err != nil {
		return err
	}
	if failed := commitMsg(h, msg, ok, repo); failed != "" {
		return h.Block("the project's commit-msg checks reject this message\n"+failed+"\n"+layout.KV("do", "fix it to their convention"), "commit-msg")
	}
	if !approved(h.Payload.String("transcript_path"), subject) {
		h.Decide("ask", "skills/kit-rules/reference/workflow.md section 1: the current prompt doesn't ask for a commit, and no earlier reply showed this message; confirm only if the user asked for it")
	}
	return nil
}

// scanSlop blocks text, read as surface, when it holds AI tells in a
// slop.block_tiers tier, and logs every hit; where names the text in both.
// A nil d (no kit.yml) fails open.
func scanSlop(h *hook.Hook, d *slop.Detector, text, surface, where string) error {
	if d == nil || strings.TrimSpace(text) == "" {
		return nil
	}
	_, hits := d.Check(slop.Prose(text), surface, func(hit slop.Hit) {
		h.Log(kitlog.Guards, "slop", slop.LogPairs(hit, where)...)
	})
	if len(hits) > 0 {
		return h.Block(slop.Message(where, hits, ""), "slop")
	}
	return nil
}

// detector is h's slop detector; nil without kit.yml.
func detector(h *hook.Hook) *slop.Detector {
	if cfg := h.Config(); cfg != nil {
		return slop.New(cfg)
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
	for w := range walk(segs, home, cwd) {
		if w.name != "git" {
			continue
		}
		var words []string
		for _, word := range w.call.Words[w.i+1:] {
			words = append(words, word.Value)
		}
		if sub, args, dir := classify.GitSubcommand(words); sub == "commit" {
			repo := resolveDir(home, w.cwd, dir)
			if strings.ContainsAny(repo, "$`") {
				repo = ""
			}
			return commit{args: args, repo: repo, found: true, unparsed: unparsed, start: w.call.start}
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

// approved reports whether the user approved this commit: the current
// prompt asks for a commit, or an assistant reply before it held subject,
// so the message was shown and the user answered.
func approved(path, subject string) bool {
	if path == "" || subject == "" {
		return false
	}
	var earlier, current strings.Builder
	prompt := ""
	_ = transcript.Each(path, func(e transcript.Entry) {
		switch {
		case e.StartsTurn():
			earlier.WriteString(current.String())
			current.Reset()
			prompt = ""
			if e.FromUser() {
				prompt = e.PromptText()
			}
		case e.Type == "assistant":
			current.WriteString(e.PromptText() + "\n")
		}
	})
	return asksForCommit(prompt) || strings.Contains(earlier.String(), subject)
}

var (
	commitWord = regexp.MustCompile(`(?i)\bcommit\b(\s+\w+)?`)
	// negated is a clause that turns down what follows: "don't", "do not",
	// "not yet", "never", "no".
	negated = regexp.MustCompile(`(?i)\b(don'?t|do not|not yet|never|no)\b`)
	// commitObjects can follow "commit" when it's asked for: what to
	// commit, or what comes next. Any other word ("commit hooks", "commit
	// 35c7066", "commit message") makes it a noun.
	commitObjects = []string{"it", "this", "that", "these", "those", "them", "all", "both", "everything", "the", "my", "your", "our", "and", "then", "now", "please", "first", "too", "again", "step", "changes", "work"}
	stepNumber    = regexp.MustCompile(`^\d{1,3}$`)
	// requestLead is a word that can come right before "commit" when it's
	// asked for: "fix it and commit", "can you commit", "please commit".
	// Any other ("the last commit", "in commit abc") makes it a noun.
	requestLead = []string{"and", "then", "now", "please", "pls", "also", "just", "so", "go", "ahead", "you", "can", "could", "to", "yes", "ok", "okay", "sure", "let's", "lets"}
)

// asksForCommit is true when prompt uses "commit" as a request: first in
// its clause or after a requestLead word, followed by nothing, a
// commitObjects word, or a step number, and with no negation earlier in
// its clause.
func asksForCommit(prompt string) bool {
	for _, m := range commitWord.FindAllStringSubmatchIndex(prompt, -1) {
		if m[2] >= 0 {
			next := strings.ToLower(strings.TrimSpace(prompt[m[2]:m[3]]))
			if !slices.Contains(commitObjects, next) && !stepNumber.MatchString(next) {
				continue
			}
		}
		clause := prompt[strings.LastIndexAny(prompt[:m[0]], ".!?;,\n")+1 : m[0]]
		words := strings.Fields(strings.ToLower(clause))
		if len(words) > 0 && !slices.Contains(requestLead, words[len(words)-1]) {
			continue
		}
		if !negated.MatchString(clause) {
			return true
		}
	}
	return false
}

// scanStaged runs gitleaks on repo's staged diff. Its exit codes: 0 clean, 1 a
// leak, anything else an operational error that fails open with a notice.
func scanStaged(h *hook.Hook, repo string) error {
	gitleaks, err := kittool.Path("gitleaks")
	if err != nil {
		return nil
	}
	// gitleaks exits 1, a leak's code, on a path it can't stat: a missing
	// git -C directory.
	if !fsx.IsDir(repo) {
		fmt.Fprintf(h.Stderr, "%s: %s is not a directory; skipping scan\n", h.Name, repo)
		return nil
	}
	err = proc.Command(proc.Quick, gitleaks, "git", "--staged", "--no-banner", "--redact", "--log-level", "error", repo).Run()
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
// code and YAML frontmatter: a deterministic stand-in for skills/kit-rules/reference/output.md
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
