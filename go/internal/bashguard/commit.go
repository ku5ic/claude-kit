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
	"unicode"
	"unicode/utf8"

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
)

var (
	// git, any global options, then commit as a word, all in one simple
	// command: the gap crosses no separator or newline but a line continuation.
	commitLine   = regexp.MustCompile(`\bgit(?:[ \t]|\\\n)+(?:[^[:space:];&|]+(?:[ \t]|\\\n)+)*commit(?:[^-[:alnum:]_.]|$)`)
	heredocOpen  = regexp.MustCompile(`<<(-?)[[:space:]]*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)
	nonProseLine = regexp.MustCompile(`^[[:space:]]*([0-9]+[.)]|[-*+][[:space:]]|#{1,6}[[:space:]]|>|\|)`)
)

// CheckCommit is guard-commit: it inspects git commit commands for a staged
// secret (gitleaks), an unchunked wall of text in the body (skills/kit-rules/reference/output.md
// section 1), and AI tells in the message (kit.yml slop). AI attribution is
// guard-attribution's.
func CheckCommit(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	cmd := h.Payload.String(hook.KeyInputCommand)
	segs, unparsed := parseAll(cmd, h.Shell)
	commits := commitsIn(segs, unparsed, h.Home, h.Payload.Cwd())
	if !commits[0].commits(cmd) {
		return nil
	}
	scanned, d := map[string]bool{}, detector(h)
	// An expansion other than $HOME or a pinned variable can't be resolved
	// here; most name the cwd's repo ("$PWD", "$(git rev-parse
	// --show-toplevel)").
	pin := pinnedCalls(h, cmd, segs)
	repos, msgs, unreads, subjects := make([]string, len(commits)), make([]string, len(commits)), make([]string, len(commits)), make([]string, len(commits))
	for i, c := range commits {
		repos[i] = cmp.Or(c.repo, h.Payload.Cwd())
		if p := slices.IndexFunc(pin.calls, func(p commit) bool { return p.start == c.start }); pin.homeKnown && p >= 0 && pin.calls[p].repo != "" {
			repos[i] = pin.calls[p].repo
		}
		msgs[i], unreads[i] = message(cmd, c, repos[i], h.Home)
		subjects[i] = subject(msgs[i])
	}
	// The approval reads the transcript while gitleaks and the commit-msg
	// checks run, and it touches nothing of h. Throwaway repos need none.
	ok := make(chan bool, 1)
	if throwaway(h, cmd, segs, unparsed, pin, commits) {
		ok <- true
	} else {
		go func() {
			// A failed read lets the commit through, as the hook would.
			defer proc.Recover(func(any) { ok <- true })
			ok <- approved(h.Payload.String(hook.KeyTranscriptPath), h.Shell, subjects)
		}()
	}
	for i := range commits {
		repo, msg, unread := repos[i], msgs[i], unreads[i]

		// Before the subject parsing: secret scanning must not depend on
		// whether -m used a quoted message.
		if !scanned[repo] {
			scanned[repo] = true
			if err := scanStaged(h, repo); err != nil {
				return err
			}
		}
		if i == 0 {
			if err := wallOfText(h, cmd); err != nil {
				return err
			}
		}
		if err := scanSlop(h, d, msg, "commit", "the commit message"); err != nil {
			return err
		}
		if unread != "" {
			return h.Block("can't read the commit message: "+unread+"\n"+
				layout.KV("do", "give it in -m, a -m \"$(cat <<'EOF' ...)\" heredoc, -F - with a heredoc, or a file written in an earlier call"), "unread-text")
		}
		if failed := commitMsg(h, msg, repo); failed != "" {
			return h.Block("the project's commit-msg checks reject this message\n"+failed+"\n"+layout.KV("do", "fix it to their convention"), "commit-msg")
		}
	}
	if !<-ok {
		h.Ask("skills/kit-rules/reference/workflow.md section 1: the current prompt doesn't ask for a commit, and no earlier reply showed this message; confirm only if the user asked for it", "commit-approval")
	}
	return nil
}

// wallOfText blocks a commit message paragraph over 4 prose lines. This
// repo passes multi-line messages via a heredoc (-m "$(cat <<'EOF' ...
// EOF)"). A single-line -m has no heredoc and is skipped: short enough that
// a miss is harmless.
func wallOfText(h *hook.Hook, cmd string) error {
	if run := longestProseRun(heredocBody(cmd)); run > 4 {
		reason := fmt.Sprintf("a commit message paragraph runs %d prose lines\n", run) +
			layout.KV("do", "split it into paragraphs of 4 lines or fewer") + "\n" +
			layout.KV("how", "skills/kit-rules/reference/output.md section 1")
		return h.Block(reason, "commit-wall-of-text")
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

// commit is the args and repo of one parsed git commit call in a command;
// found is false when none commits, unparsed true when part of the command
// didn't parse.
type commit struct {
	sub  string // the git subcommand: commit, or another for gitCalls
	args []string
	// repo is the directory it commits in: its git -C directory, against
	// where the cds before it moved, "" when an expansion names it.
	repo            string
	found, unparsed bool
	start           int // source offset of the git call
	// dir is its one git -C word, nil for none; elsewhere is true when
	// something else may pick the repo (several or joined -C, --git-dir,
	// --work-tree, --namespace, -c). seg is the segment it's in, cwd
	// where the cds before it moved, and sure whether the shell surely
	// runs it there (dirs.lost).
	dir       *Word
	elsewhere bool
	seg       int
	cwd       string
	sure      bool
}

// commits reports whether cmd, parsed as c, runs git commit: a parsed call
// whose real command is git with commit as its subcommand. Where part of
// cmd didn't parse, the raw text is matched instead, so a commit whose
// heredoc never closes still counts.
func (c commit) commits(cmd string) bool {
	return c.found || c.unparsed && commitLine.MatchString(cmd)
}

// commitCalls parses cmd, run from cwd, for its commitsIn.
func commitCalls(cmd, home, cwd, shell string) []commit {
	segs, unparsed := parseAll(cmd, shell)
	return commitsIn(segs, unparsed, home, cwd)
}

// commitsIn is every git commit call in segs, run from cwd; with none,
// it's one commit that isn't found.
func commitsIn(segs []Segment, unparsed bool, home, cwd string) []commit {
	out := slices.DeleteFunc(gitCalls(segs, home, cwd, unparsed, nil), func(c commit) bool { return c.sub != "commit" })
	if len(out) == 0 {
		return []commit{{unparsed: unparsed}}
	}
	return out
}

// gitCalls is every git call of segs, run from cwd, read as a commit is.
func gitCalls(segs []Segment, home, cwd string, unparsed bool, pin pinFunc) []commit {
	var out []commit
	for w := range walkPinned(segs, home, cwd, pin) {
		if w.name != "git" {
			continue
		}
		if sub, args, dir := classify.GitSubcommand(values(w.call.Words[w.i+1:])); sub != "" {
			repo := resolveDir(home, w.cwd, dir)
			if strings.ContainsAny(repo, "$`") {
				repo = ""
			}
			c := commit{sub: sub, args: args, repo: repo, found: true, unparsed: unparsed, start: w.call.start, cwd: w.cwd, sure: w.sure}
			// Read before the subcommand, since git commit -C reuses a message.
			words := w.call.Words[w.i+1:]
			for i := 0; i < len(words) && words[i].Value != sub; i++ {
				switch v := words[i].Value; {
				case v == "-C" && i+1 < len(words) && c.dir == nil:
					c.dir = &words[i+1]
					i++
				case strings.HasPrefix(v, "-C"), v == "-c", strings.HasPrefix(v, "--git-dir"),
					strings.HasPrefix(v, "--work-tree"), strings.HasPrefix(v, "--namespace"):
					c.elsewhere = true
				}
			}
			c.seg = slices.IndexFunc(segs, func(s Segment) bool {
				return slices.ContainsFunc(s.Calls, func(call Call) bool { return call.start == w.call.start })
			})
			out = append(out, c)
		}
	}
	return out
}

// message is the message cmd's git commit c gives, as the commit-msg hook
// would read it: its -m messages as paragraphs, a $(cat <<EOF) one read as
// its body, else the file -F names, from repo or under home, unless the command names it
// before the commit, or the commit line's heredoc
// for -F - or a command that didn't parse. unread says why a message given
// can't be told exactly; one an expansion builds is still msg, as written,
// for the subject checks. A commit giving none (--no-edit) has neither.
func message(cmd string, c commit, repo, home string) (msg, unread string) {
	if !c.found {
		if body := heredocBody(cmd); body != "" {
			return body + "\n", ""
		}
		return "", "part of the command didn't parse"
	}
	var paragraphs []string
	file, expanded := "", false
	commitOptions(c.args, func(opt, value string) {
		switch opt {
		case "-m", "--message":
			if body, ok := catHeredoc(value); ok {
				value = body
			} else if expansion.MatchString(value) {
				expanded = true
			}
			paragraphs = append(paragraphs, value)
		case "-F", "--file":
			file = value
		}
	})
	switch {
	case len(paragraphs) > 0 && expanded:
		return strings.Join(paragraphs, "\n\n") + "\n", "an expansion builds the message"
	case len(paragraphs) > 0:
		return strings.Join(paragraphs, "\n\n") + "\n", ""
	case file == "-":
		if body := heredocBody(cmd); body != "" {
			return body + "\n", ""
		}
		return "", "stdin isn't in the command"
	// Named before the commit, the file is likely written there first:
	// what's on disk now is a stale message. Named after (an rm) it isn't.
	case expansion.MatchString(guard.ExpandHome(home, file)):
		return "", "an expansion names the file"
	case file != "" && strings.Contains(cmd[:c.start], filepath.Base(file)):
		return "", "the command writes the file first"
	case file != "":
		data, err := os.ReadFile(fsx.Abs(repo, guard.ExpandHome(home, file)))
		if err != nil {
			return "", "the file isn't readable"
		}
		return string(data), ""
	}
	return "", ""
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
// file, and returns what failed; "" when it passes or the project has
// none.
func commitMsg(h *hook.Hook, msg, repo string) string {
	root, cfg := git.Toplevel(repo), h.Config()
	if root == "" || cfg == nil {
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
	p := enforce.CommitMsg(cfg, enforce.Options{Root: root, CacheDir: h.Paths.CacheDir(), Home: h.Home}, tmp.Name())
	return run.Commit(p, commitMsgBudget)
}

var (
	commitWord = regexp.MustCompile(`(?i)\bcommit\b|\bship\s+it\b`)
	// negated is a clause that turns down what follows: "don't", "not",
	// "aren't", "never", "no", and git status's "nothing to".
	negated = regexp.MustCompile(`(?i)\b(don['\x{2019}]?t|not|never|no|nothing\s+to)\b|n['\x{2019}]t\b`)
	// directAsk opens a question that is a request: "can you commit this?".
	// Any other question ("is it ok to commit?") asks about one.
	directAsk = regexp.MustCompile(`(?i)^\s*(can|could|would|will)\s+(you|we)\b`)
	// ownPlan opens a sentence where the user says what they'll do: "let
	// me review it, then commit".
	ownPlan = regexp.MustCompile(`(?i)^\s*(let me|i['\x{2019}]ll|i will|i['\x{2019}]m going to|i am going to)\b`)
	// nounLead before "commit" makes it a noun: a determiner or a
	// preposition, as in "the last commit", "in commit abc".
	nounLead = nameSet("a", "an", "the", "my", "your", "our", "their", "his", "her", "its", "this", "that", "these", "those",
		"last", "first", "next", "previous", "initial", "each", "every", "any", "which", "whose", "in", "of", "from", "on", "at", "since", "per")
	// nounTail after "commit" makes it a noun too: "commit hooks", "commit
	// message", or a hash, "commit 35c7066".
	nounTail = nameSet("hook", "hooks", "message", "messages", "msg", "history", "log", "logs", "id", "hash", "sha", "range",
		"count", "graph", "time", "date", "author", "title", "body", "subject", "style", "format", "convention", "conventions", "rules")
	gitHash = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	// askWord opens a question about committing, not a request for one:
	// "how to commit", "when do I commit".
	askWord = nameSet("how", "when", "where", "why", "whether", "what")
	// makeVerb before a nounLead asks for the commit it names: "make a
	// commit".
	makeVerb = nameSet("make", "do", "create", "add")
	// notNow after "commit" in its sentence turns it down: "commit it, but
	// not yet".
	notNow = regexp.MustCompile(`(?i)\bnot\s+(?:yet|now|right\s+now)\b`)
)

// asksForCommit is true when prompt asks for a commit: "commit" as a verb,
// or "ship it", with no negation earlier in its clause and no "not yet"
// after it, in a sentence that isn't the user's own plan or a question,
// one ending in "?" other than a direct ask ("can you commit this?") or
// one an askWord opens ("how to commit"). A condition or a later time
// ("commit when the tests pass") is the user's to keep, so it counts.
func asksForCommit(prompt string) bool {
	for _, m := range commitWord.FindAllStringSubmatchIndex(prompt, -1) {
		// commit-msg, pre-commit, commit/push, commit's: part of a compound
		// or a noun.
		if end := m[1]; end < len(prompt) && strings.ContainsRune("-/:'", rune(prompt[end])) || m[0] > 0 && strings.ContainsRune("-/", rune(prompt[m[0]-1])) {
			continue
		}
		start := lastStopIndex(prompt[:m[0]], ".!?\n") + 1
		// A negation reaches across commas and line breaks: "Don't, under
		// any circumstances, commit this." A ; or a sentence end stops it.
		scope := prompt[lastStopIndex(prompt[:m[0]], ".!?;")+1 : m[0]]
		if negated.MatchString(scope) || commitNoun(prompt[start:m[0]], prompt[m[1]:]) {
			continue
		}
		// The sentence ends after the commit word: a "." inside a name
		// before it ("Main.Go") ends nothing.
		sentence := prompt[start:]
		if end := stopIndex(prompt[m[1]:], ".!?\n"); end >= 0 {
			end += m[1] - start
			if sentence[end] == '?' && !directAsk.MatchString(sentence) {
				continue
			}
			sentence = sentence[:end]
		}
		if !ownPlan.MatchString(sentence) && !notNow.MatchString(sentence[m[1]-start:]) {
			return true
		}
	}
	return false
}

// commitNoun is whether "commit" between before and after isn't asked for:
// a noun, with a nounLead word right before it in its clause or a nounTail
// word or a hash right after, or the subject of a question an askWord
// opens ("how to commit", "when do we commit").
func commitNoun(before, after string) bool {
	before = before[strings.LastIndexAny(before, ",;:")+1:]
	prev := strings.FieldsFunc(strings.ToLower(before), notWordRune)
	next := strings.FieldsFunc(strings.ToLower(after), notWordRune)
	n := len(prev)
	switch {
	// "make a commit", "create the commit": a noun, but asked for.
	case n > 1 && nounLead[prev[n-1]] && makeVerb[prev[n-2]]:
	case n > 0 && (nounLead[prev[n-1]] || askWord[prev[n-1]] || askWord[prev[0]]):
		return true
	case n > 1 && prev[n-1] == "to" && askWord[prev[n-2]]:
		return true
	}
	return len(next) > 0 && (nounTail[next[0]] || gitHash.MatchString(next[0]))
}

// scanStaged runs gitleaks on repo's staged diff. Its exit codes: 0 clean, 1 a
// leak, anything else an operational error that fails open with a notice.
func scanStaged(h *hook.Hook, repo string) error {
	gitleaks, err := kittool.Path("gitleaks")
	if err != nil {
		return nil
	}
	// gitleaks exits 1, a leak's code, on a path it can't stat. A missing
	// repo dir has nothing staged: the command makes it, or git fails.
	if !fsx.IsDir(repo) {
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

// notWordRune splits a clause into words, dropping punctuation.
func notWordRune(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\'' }

// nameDot is whether the "." at s[i] is inside a file name: a lowercase
// letter or digit follows it ("notes.txt", ".env", "v1.2"). Before a
// capital it ends a sentence: "Commit this.Then push".
func nameDot(s string, i int) bool { return letterAfter(s, i, unicode.IsLower) }

// letterAfter is whether s[i+1:] opens with a digit or a rune letter
// accepts.
func letterAfter(s string, i int, letter func(rune) bool) bool {
	r, _ := utf8.DecodeRuneInString(s[i+1:])
	return letter(r) || unicode.IsDigit(r)
}

// stopIndex is the index of the first byte of stops in s, or -1. A
// nameDot is part of a name, not a stop.
func stopIndex(s, stops string) int {
	for i := range len(s) {
		if isStop(s, i, stops, unicode.IsLower) {
			return i
		}
	}
	return -1
}

// lastStopIndex is stopIndex from the end, where a "." before any letter
// is part of a name: a negation or condition before "CONTRIBUTING.MD"
// keeps its reach, so a doubtful split asks rather than allows.
func lastStopIndex(s, stops string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if isStop(s, i, stops, unicode.IsLetter) {
			return i
		}
	}
	return -1
}

// isStop is whether s[i] is one of stops, a "." only when no digit or
// rune letter accepts follows it.
func isStop(s string, i int, stops string, letter func(rune) bool) bool {
	if strings.IndexByte(stops, s[i]) < 0 {
		return false
	}
	return s[i] != '.' || !letterAfter(s, i, letter)
}
