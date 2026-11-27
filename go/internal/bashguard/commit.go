package bashguard

import (
	"bytes"
	"cmp"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	"github.com/ku5ic/claude-kit/go/internal/transcript"
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
	commits := commitCalls(cmd, h.Home, h.Payload.Cwd())
	if !commits[0].commits(cmd) {
		return nil
	}
	scanned, d := map[string]bool{}, detector(h)
	// An expansion other than $HOME or a pinned variable can't be resolved
	// here; most name the cwd's repo ("$PWD", "$(git rev-parse
	// --show-toplevel)").
	segs, _ := parseAll(cmd)
	pinned, homeKnown := pinnedCalls(h, cmd, segs)
	repos, msgs, unreads, subjects := make([]string, len(commits)), make([]string, len(commits)), make([]string, len(commits)), make([]string, len(commits))
	for i, c := range commits {
		repos[i] = cmp.Or(c.repo, h.Payload.Cwd())
		if p := slices.IndexFunc(pinned, func(p commit) bool { return p.start == c.start }); homeKnown && p >= 0 && pinned[p].repo != "" {
			repos[i] = pinned[p].repo
		}
		msgs[i], unreads[i] = message(cmd, c, repos[i], h.Home)
		subjects[i] = subject(msgs[i])
	}
	// The approval reads the transcript while gitleaks and the commit-msg
	// checks run, and it touches nothing of h. Throwaway repos need none.
	ok := make(chan bool, 1)
	if throwaway(h, cmd, commits) {
		ok <- true
	} else {
		go func() {
			// A panic here would exit 2, which Claude Code reads as a
			// block; the hook's recover can't reach this goroutine. A
			// failed read lets the commit through, as the hook would.
			defer func() {
				if recover() != nil {
					ok <- true
				}
			}()
			ok <- approved(h.Payload.String(hook.KeyTranscriptPath), subjects)
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
		h.Decide("ask", "skills/kit-rules/reference/workflow.md section 1: the current prompt doesn't ask for a commit, and no earlier reply showed this message; confirm only if the user asked for it")
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
	// --work-tree, --namespace, -c). seg is the segment it's in, and cwd
	// where the cds before it moved.
	dir       *Word
	elsewhere bool
	seg       int
	cwd       string
}

// commits reports whether cmd, parsed as c, runs git commit: a parsed call
// whose real command is git with commit as its subcommand. Where part of
// cmd didn't parse, the raw text is matched instead, so a commit whose
// heredoc never closes still counts.
func (c commit) commits(cmd string) bool {
	return c.found || c.unparsed && commitLine.MatchString(cmd)
}

// commitCalls parses cmd, run from cwd, for every git commit call in it;
// with none, it's one commit that isn't found.
func commitCalls(cmd, home, cwd string) []commit {
	segs, unparsed := parseAll(cmd)
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
			c := commit{sub: sub, args: args, repo: repo, found: true, unparsed: unparsed, start: w.call.start, cwd: w.cwd}
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

// approved reports whether the user approved these commits: the current
// prompt asks for a commit, or an assistant reply before it showed every
// subject, so the user answered the messages.
func approved(path string, subjects []string) bool {
	if path == "" || slices.Contains(subjects, "") {
		return false
	}
	// The current prompt sits in the transcript's tail; parsing the whole
	// session is needed only to look for an earlier reply showing the
	// subjects, and only when the file holds every one of them.
	// An AskUserQuestion pick approves one commit; a command running
	// several asks.
	one := len(subjects) == 1
	turnInTail := false
	if tail, err := transcript.Tail(path, math.MaxInt); err == nil {
		for i, e := range slices.Backward(tail) {
			if !e.StartsTurn() {
				continue
			}
			turnInTail = true
			reply := lastReply(tail[:i])
			if e.FromUser() && requests(e.PromptText(), reply) || one && pickedCommit(tail[i+1:]) {
				return true
			}
			// A reply the tail cut off is left to the whole parse below.
			if reply != "" {
				if data, err := os.ReadFile(path); err == nil && slices.ContainsFunc(subjects, func(s string) bool { return !mayHold(data, s) }) {
					return false
				}
			}
			break
		}
	}
	var earlier, current strings.Builder
	prompt, reply, before := "", "", ""
	// The entries of a turn that started before the tail, for its picks.
	var turn []transcript.Entry
	_ = transcript.Each(path, func(e transcript.Entry) {
		if e.StartsTurn() {
			turn = nil
		} else if one && !turnInTail {
			turn = append(turn, e)
		}
		switch {
		case e.StartsTurn():
			earlier.WriteString(current.String())
			current.Reset()
			prompt, before = "", reply
			if e.FromUser() {
				prompt = e.PromptText()
			}
		case e.Type == "assistant":
			current.WriteString(e.PromptText() + "\n")
			if text := strings.TrimSpace(e.PromptText()); text != "" {
				reply = text
			}
		}
	})
	if requests(prompt, before) || pickedCommit(turn) {
		return true
	}
	text := earlier.String()
	return !slices.ContainsFunc(subjects, func(s string) bool { return !shown(text, s) })
}

// bareYes is a prompt that only says yes.
var bareYes = regexp.MustCompile(`(?i)^\s*(yes|y|yep|yeah|yup|sure|ok|okay|go|go ahead|do it|please do)\s*[.!]*\s*$`)

// commitQuestion is a reply's last line when it asks whether to commit
// and shows no message: "Commit step 16?", "Should I commit both?". One
// that shows a message approves only that message, as shown() reads it.
var commitQuestion = regexp.MustCompile("(?i)^\\W*(?:(?:should|shall|can) i\\s+|want me to\\s+)?commit\\b[^?`]*\\?\\s*$")

// orWord is a question's "or": "Commit step 4, or run the gate first?"
// offers two things, and a yes picks neither.
var orWord = regexp.MustCompile(`(?i)\bor\b`)

// requests is whether prompt asks for a commit: in its own words, or as a
// bare yes to reply, the text before it, when reply's last line asks to
// commit and nothing else.
func requests(prompt, reply string) bool {
	if asksForCommit(prompt) {
		return true
	}
	lines := strings.Split(strings.TrimSpace(reply), "\n")
	last := lines[len(lines)-1]
	return bareYes.MatchString(prompt) && commitQuestion.MatchString(last) && !orWord.MatchString(last)
}

// recommended is the marker Claude puts on the option it suggests.
var recommended = regexp.MustCompile(`\s*\(Recommended\)\s*$`)

// pickedCommit is whether the user, in entries, the turn so far, answered
// an AskUserQuestion with a commit, and no commit has run since: one
// answer approves one commit. A question the user rejected withdraws it,
// and a commit that failed doesn't spend it. The current call's tool_use is
// in the transcript when its hook runs, so a second commit with no result
// yet is a sibling sent beside it, and the pick covers neither.
func pickedCommit(entries []transcript.Entry) bool {
	asks, pending := map[string]map[string]question{}, map[string]bool{}
	picked := false
	for _, e := range entries {
		for _, b := range e.ToolUses() {
			switch {
			case b.Name == "AskUserQuestion":
				asks[b.ID] = offered(b.RawInput)
			case b.Name == "Bash" && isCommit(b.Input.Command):
				pending[b.ID] = true
			}
		}
		for _, b := range e.Blocks {
			switch {
			case b.Type != "tool_result":
			case asks[b.ToolUseID] != nil:
				picked = !b.IsError && commitAnswer(asks[b.ToolUseID], e.ToolUseResult.Answers)
			case pending[b.ToolUseID]:
				delete(pending, b.ToolUseID)
				if !b.IsError {
					picked = false
				}
			}
		}
	}
	return picked && len(pending) <= 1
}

// isCommit is whether cmd runs git commit, as CheckCommit reads it.
func isCommit(cmd string) bool {
	return commitLine.MatchString(cmd) && commitCalls(cmd, "", "")[0].commits(cmd)
}

// question is an AskUserQuestion's options and whether the user may pick
// several.
type question struct {
	labels []string
	multi  bool
}

// offered is an AskUserQuestion input's questions, by their text.
func offered(input json.RawMessage) map[string]question {
	var in struct {
		Questions []struct {
			Question    string `json:"question"`
			MultiSelect bool   `json:"multiSelect"`
			Options     []struct {
				Label string `json:"label"`
			} `json:"options"`
		} `json:"questions"`
	}
	_ = json.Unmarshal(input, &in)
	out := map[string]question{}
	for _, q := range in.Questions {
		asked := question{multi: q.MultiSelect}
		for _, o := range q.Options {
			asked.labels = append(asked.labels, o.Label)
		}
		out[q.Question] = asked
	}
	return out
}

// commitAnswer is whether answers, the structured record the harness keeps,
// answer one of the questions asked with a commit: a picked label or typed
// text that asks for one now, as a prompt would. The question text, which
// Claude wrote, is never matched.
func commitAnswer(asked map[string]question, answers map[string]string) bool {
	for text, answer := range answers {
		q, ok := asked[text]
		if !ok {
			continue
		}
		for _, pick := range picks(q, answer) {
			if asksForCommit(recommended.ReplaceAllString(pick, "")) {
				return true
			}
		}
	}
	return false
}

// picks is answer split into what the user chose: a multi-select's labels,
// which the harness joins with ", " and quotes when one holds a comma, or
// the single label or typed text. An answer whose parts aren't all labels
// is typed text, read whole: a comma inside it doesn't end a negation.
func picks(q question, answer string) []string {
	if !q.multi || slices.Contains(q.labels, answer) {
		return []string{answer}
	}
	r := csv.NewReader(strings.NewReader(answer))
	r.TrimLeadingSpace, r.LazyQuotes = true, true
	fields, err := r.Read()
	if err != nil || slices.ContainsFunc(fields, func(f string) bool { return !slices.Contains(q.labels, f) }) {
		return []string{answer}
	}
	return fields
}

// lastReply is the text of the last assistant entry in entries that has
// any, "" for none.
func lastReply(entries []transcript.Entry) string {
	for _, e := range slices.Backward(entries) {
		if e.Type == "assistant" {
			if text := strings.TrimSpace(e.PromptText()); text != "" {
				return text
			}
		}
	}
	return ""
}

// mayHold is whether data, a JSONL transcript, could hold subject in some
// reply: as written, or JSON-escaped ("fix: \"x\"").
func mayHold(data []byte, subject string) bool {
	if bytes.Contains(data, []byte(subject)) {
		return true
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if enc.Encode(subject) != nil {
		return true
	}
	escaped := bytes.TrimSuffix(b.Bytes(), []byte("\n"))
	return bytes.Contains(data, escaped[1:len(escaped)-1])
}

// lineMarker is a Markdown marker opening a line: a bullet, a quote, a
// heading, or a numbered item.
var lineMarker = regexp.MustCompile(`^(?:[-*+>#]+|\d+[.)])\s*`)

// shown is whether text showed subject as a message: a backticked span, or
// a line holding only it (in a fence, a list item, a quote, or bold). A
// subject inside a sentence ("wip" in "wipe") isn't a shown message.
func shown(text, subject string) bool {
	if strings.Contains(text, "`"+subject+"`") {
		return true
	}
	for line := range strings.Lines(text) {
		// Each marker off in turn: a subject may itself open with one ("1.2 notes").
		for line = strings.TrimSpace(line); ; {
			if line == subject || strings.Trim(line, "*_\"'") == subject {
				return true
			}
			m := lineMarker.FindString(line)
			if m == "" {
				break
			}
			line = line[len(m):]
		}
	}
	return false
}

var (
	commitWord = regexp.MustCompile(`(?i)\bcommit\b`)
	// negated is a clause that turns down what follows: "don't", "not",
	// "aren't", "never", "no", and git status's "nothing to".
	negated = regexp.MustCompile(`(?i)\b(don['\x{2019}]?t|not|never|no|nothing\s+to)\b|n['\x{2019}]t\b`)
	// directAsk opens a question that is a request: "can you commit this?".
	// Any other question ("is it ok to commit?") asks about one.
	directAsk = regexp.MustCompile(`(?i)^\s*(can|could|would|will)\s+(you|we)\b`)
	// ownPlan opens a sentence where the user says what they'll do: "let
	// me review it, then commit".
	ownPlan = regexp.MustCompile(`(?i)^\s*(let me|i['\x{2019}]ll|i will|i['\x{2019}]m going to|i am going to)\b`)
	// The words after a requested "commit", up to "and", "then", or the
	// clause's end, name what to commit and nothing else: an object, a
	// determiner and up to three words ("the docs fix"), a step number, or
	// a word that changes nothing about when. Any other word ("commit
	// hooks", "commit 35c7066", "commit it later") isn't a go-ahead now.
	commitObjects  = []string{"it", "them", "everything", "changes", "work", "step"}
	determiners    = []string{"the", "my", "your", "our", "this", "that", "these", "those", "all", "both", "any", "every", "each"}
	commitModifier = []string{"now", "please", "first", "too", "again", "also"}
	stepNumber     = regexp.MustCompile(`^\d{1,3}(-\d{1,3})?$`)
	// timeWords put a commit off even inside its object: "commit the docs
	// fix later".
	timeWords = []string{"later", "tomorrow", "tonight", "today", "afterwards", "afterward", "eventually", "soon", "sometime"}
	// requestLead is a word that can come right before "commit" when it's
	// asked for: "fix it and commit", "can you commit", "please commit".
	// Any other ("the last commit", "in commit abc") makes it a noun.
	requestLead = []string{"and", "then", "now", "please", "pls", "also", "just", "so", "go", "ahead", "can", "could", "yes", "ok", "okay", "sure", "let's", "lets"}
	// askedYou can come before "you commit" in a request: "can you
	// commit". Any other ("if you commit this") makes it a remark.
	askedYou = []string{"can", "could", "would", "will", "please"}
	// userSaid is a clause that repeats a request: "I said commit", "ugh,
	// I said commit". Anyone else's ("the reviewer said commit") isn't one,
	// nor one told as a story: inside a clause ("the bug is that I said
	// commit"), or after a pastTime word ("earlier, I said commit").
	userSaid = regexp.MustCompile(`(?i)^\s*i\s+(?:already\s+|just\s+)?said\s*$`)
	pastTime = regexp.MustCompile(`(?i)\b(earlier|yesterday|previously|ago|originally|initially|last)\b`)
	// subordinator anywhere in the commit's sentence makes it conditional,
	// across a comma too: "when the tests pass, commit it", "commit it,
	// but only after I review". Subordinating conjunctions are a closed
	// class, so this list doesn't grow with each new phrasing.
	subordinator = regexp.MustCompile(`(?i)\b(if|unless|once|when|whenever|after|before|until|till|provided|providing|assuming|while|whilst|lest|as soon as|as long as|so long as|in case)\b`)
	// toldIf is an if or when that opens what the user asks to be told,
	// "tell me if", "let me know when": it conditions the telling, not the
	// commit.
	toldIf = regexp.MustCompile(`(?i)\b(?:tell|let|show)\s+(?:me|us)(?:\s+know)?\s+(?:if|when)\b`)
)

// asksForCommit is true when prompt uses "commit" as a request: first in
// its clause, after a requestLead word, an askedYou word and "you", or
// "I said" (leadsRequest), followed by only an object (objectPhrase),
// with no negation earlier in its clause and no subordinator in its
// sentence, in a sentence that isn't the user's own plan, and, when it
// ends in "?", opens as a direct ask. Wording it can't place reads as no request, so the
// guard asks.
func asksForCommit(prompt string) bool {
	for _, m := range commitWord.FindAllStringSubmatchIndex(prompt, -1) {
		// commit-msg, commit/push, commit's: part of a compound or a noun.
		if end := m[0] + len("commit"); end < len(prompt) && strings.ContainsRune("-/:'", rune(prompt[end])) {
			continue
		}
		rest := prompt[m[1]:]
		if end := stopIndex(rest, ".!?;,\n"); end >= 0 {
			rest = rest[:end]
		}
		if !objectPhrase(strings.FieldsFunc(strings.ToLower(rest), notPathRune)) {
			continue
		}
		start := lastStopIndex(prompt[:m[0]], ".!?\n") + 1
		clause := prompt[lastStopIndex(prompt[:m[0]], ".!?;,\n")+1 : m[0]]
		// A negation reaches across commas and line breaks: "Don't, under
		// any circumstances, commit this." A ; or a sentence end stops it.
		scope := prompt[lastStopIndex(prompt[:m[0]], ".!?;")+1 : m[0]]
		if !leadsRequest(clause, prompt[start:m[0]]) || negated.MatchString(scope) {
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
		// A told clause after the commit conditions the telling; one before
		// it ("tell us when the tests pass and commit it") sets its time.
		before, after := sentence[:m[1]-start], sentence[m[1]-start:]
		if subordinator.MatchString(before+toldIf.ReplaceAllString(after, "")) || deferred(strings.FieldsFunc(strings.ToLower(after), notWordRune)) {
			continue
		}
		if !ownPlan.MatchString(sentence) {
			return true
		}
	}
	return false
}

// leadsRequest is whether clause, the words before "commit" in its
// clause, puts it as a request: none, a requestLead word, an askedYou word
// and "you", or "I said" with no pastTime word in before, the words before
// it in its sentence.
func leadsRequest(clause, before string) bool {
	words := strings.Fields(strings.ToLower(clause))
	n := len(words)
	return n == 0 || slices.Contains(requestLead, words[n-1]) ||
		n >= 2 && words[n-1] == "you" && slices.Contains(askedYou, words[n-2]) ||
		userSaid.MatchString(clause) && !pastTime.MatchString(before)
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

// subjects open a clause of its own: what follows is someone else's plan
// ("commit it, I'll push tomorrow").
var subjects = []string{"i", "i'll", "i'm", "i'd", "i've", "we", "we'll", "we're", "you", "you'll", "you're", "they", "they'll"}

// deferred is whether words, those after "commit" to the end of its
// sentence, put the commit off: a time word, or "yet", before a "now" or
// a clause with a subject of its own, or "not now" or "not right now".
// "commit and push tomorrow", "commit it, but not yet", and "commit it,
// but not right now" wait; "commit it now, push tomorrow" and "commit it,
// do not push now" don't.
func deferred(words []string) bool {
	for i, w := range words {
		switch {
		case w == "now":
			return i > 0 && words[i-1] == "not" || i > 1 && words[i-1] == "right" && words[i-2] == "not"
		case slices.Contains(subjects, w):
			return false
		case w == "yet" || slices.Contains(timeWords, w):
			return true
		}
	}
	return false
}

// objectPhrase is whether words, those after "commit" in its clause, name
// only what to commit, up to an "and" or "then" that starts the next
// instruction.
func objectPhrase(words []string) bool {
	for i := 0; i < len(words); i++ {
		switch w := words[i]; {
		case w == "and" || w == "then":
			return true
		case slices.Contains(determiners, w):
			// "every time", "any time": when, not what.
			if next := i + 1; next < len(words) && (w == "every" || w == "each" || w == "any") && (words[next] == "time" || words[next] == "times") {
				return false
			}
			// The noun the determiner opens: up to three words, none a
			// time word.
			for n := 1; i+1 < len(words) && words[i+1] != "and" && words[i+1] != "then"; n++ {
				i++
				if n > 3 || slices.Contains(timeWords, words[i]) {
					return false
				}
			}
		// A dash used as punctuation, kept whole by notPathRune.
		case strings.Trim(w, "./_-") == "":
		case slices.Contains(commitObjects, w) || slices.Contains(commitModifier, w) || stepNumber.MatchString(w) || fileName(w):
		default:
			return false
		}
	}
	return true
}

// notWordRune splits a clause into words, dropping punctuation.
func notWordRune(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\'' }

// notPathRune splits a clause into words as notWordRune does, but keeps a
// file name or path whole: "notes.txt", "go/internal/git.go", "pre-commit".
func notPathRune(r rune) bool { return notWordRune(r) && !strings.ContainsRune("./_-", r) }

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

// fileName is whether word, lowercased, is a file name or path: a "/" and
// a letter or digit, or a nameDot.
func fileName(word string) bool {
	if strings.Contains(word, "/") && strings.ContainsFunc(word, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return true
	}
	for i := range len(word) {
		if word[i] == '.' && nameDot(word, i) {
			return true
		}
	}
	return false
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
