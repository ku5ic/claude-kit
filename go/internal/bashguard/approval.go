package bashguard

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"math"
	"os"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// approved reports whether the user approved these commits: the current
// prompt asks for a commit, or an assistant reply before it showed every
// subject, so the user answered the messages.
func approved(path, shell string, subjects []string) bool {
	if path == "" || slices.Contains(subjects, "") {
		return false
	}
	// The current prompt sits in the transcript's tail; parsing the whole
	// session is needed only to look for an earlier reply showing the
	// subjects, and only when the file holds every one of them.
	// An AskUserQuestion pick approves one commit; a command running
	// several asks.
	one := len(subjects) == 1
	ok, turnInTail, replied := approvedInTail(path, shell, one)
	if ok {
		return true
	}
	// One read of the whole file serves the subject test and the parse.
	data, err := os.ReadFile(path)
	if err != nil || replied && slices.ContainsFunc(subjects, func(s string) bool { return !mayHold(data, s) }) {
		return false
	}
	return approvedInFile(data, subjects, shell, one && !turnInTail)
}

// approvedInTail is whether the user's latest turn, found in the
// transcript's tail, asks for the commit, or with one picks it; whether
// that turn was in the tail; and whether a reply before it was. A
// notification or a compaction summary after the user's prompt doesn't
// replace it, and a reply the tail cut off is left to the whole parse.
func approvedInTail(path, shell string, one bool) (ok, turnInTail, replied bool) {
	tail, err := transcript.Tail(path, math.MaxInt)
	if err != nil {
		return false, false, false
	}
	for i, e := range slices.Backward(tail) {
		if !e.UserTurn() {
			continue
		}
		reply := lastReply(tail[:i])
		if requests(e.PromptText(), reply) || one && pickedCommit(tail[i+1:], shell) {
			return true, true, false
		}
		return false, true, reply != ""
	}
	return false, false, false
}

// approvedInFile is approved read from the whole transcript: the last
// turn asks for the commit or, with picks, picks it, or a reply before
// that turn showed every subject.
func approvedInFile(data []byte, subjects []string, shell string, picks bool) bool {
	var earlier, current strings.Builder
	prompt, reply, before := "", "", ""
	// The entries of a turn that started before the tail, for its picks.
	var turn []transcript.Entry
	_ = transcript.EachIn(data, func(e transcript.Entry) {
		if e.UserTurn() {
			turn = nil
		} else if picks {
			turn = append(turn, e)
		}
		switch {
		case e.UserTurn():
			earlier.WriteString(current.String())
			current.Reset()
			prompt, before = e.PromptText(), reply
		case e.Type == "assistant":
			current.WriteString(e.PromptText() + "\n")
			if text := strings.TrimSpace(e.PromptText()); text != "" {
				reply = text
			}
		}
	})
	if requests(prompt, before) || pickedCommit(turn, shell) {
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
	last := lastQuestion(lines[len(lines)-1])
	return bareYes.MatchString(prompt) && commitQuestion.MatchString(last) && !orWord.MatchString(last)
}

// lastQuestion is line's last sentence when a statement ends before it:
// "Tests pass. Commit it?" is "Commit it?". After another question the
// line stays whole, since a yes may answer that one.
func lastQuestion(line string) string {
	line = strings.TrimSpace(line)
	body := strings.TrimSuffix(line, "?")
	if i := lastStopIndex(body, ".!?"); i >= 0 && body[i] != '?' {
		return line[i+1:]
	}
	return line
}

// recommended is the marker Claude puts on the option it suggests.
var recommended = regexp.MustCompile(`\s*\(Recommended\)\s*$`)

// pickedCommit is whether the user, in entries, the turn so far, answered
// an AskUserQuestion with a commit, and no commit has run since: one
// answer approves one commit. A question the user rejected withdraws it,
// and a commit that failed doesn't spend it. The current call's tool_use is
// in the transcript when its hook runs, so a second commit with no result
// yet is a sibling sent beside it, and the pick covers neither.
func pickedCommit(entries []transcript.Entry, shell string) bool {
	asks, pending := map[string]map[string]question{}, map[string]bool{}
	picked := false
	for _, e := range entries {
		for _, b := range e.ToolUses() {
			switch {
			case b.Name == "AskUserQuestion":
				asks[b.ID] = offered(b.RawInput)
			case b.Name == "Bash" && isCommit(b.Input.Command, shell):
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
func isCommit(cmd, shell string) bool {
	return commitLine.MatchString(cmd) && commitCalls(cmd, "", "", shell)[0].commits(cmd)
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
	// A malformed input leaves no questions, so no pick approves the commit.
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

// notWordRune splits a clause into words, dropping punctuation.
func notWordRune(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\'' }

// letterAfter is whether s[i+1:] opens with a digit or a rune letter
// accepts.
func letterAfter(s string, i int, letter func(rune) bool) bool {
	r, _ := utf8.DecodeRuneInString(s[i+1:])
	return letter(r) || unicode.IsDigit(r)
}

// stopIndex is the index of the first byte of stops in s, or -1. A "."
// before a lowercase letter or digit ("notes.txt", "v1.2") is part of a
// name, not a stop; before a capital it ends a sentence.
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
