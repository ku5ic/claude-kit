// Package claims checks the factual claims in a text against the session's
// evidence before the text reaches anyone: a claude -p verifier reads the
// session's tool results, checks what they don't cover with read-only
// tools, and names every claim nothing confirms.
package claims

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claudecli"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/connector"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// Guard is set in the verifier's environment, with a marker under the cache
// (claudecli.Guarded), so its own session runs no kit hook and can't
// recurse into another verification.
const Guard = "KIT_VERIFIER"

// AgentFile is the claim-checker agent under the kit root. Its body is the
// verifier's system prompt, so the hook's own run and the agent Claude
// starts follow one text.
func AgentFile(root string) string { return filepath.Join(root, "agents", "claim-checker.md") }

// systemPrompt is the agent file's body, its frontmatter dropped.
func systemPrompt(root string) (string, error) {
	data, err := os.ReadFile(AgentFile(root))
	if err != nil {
		return "", fmt.Errorf("claims: %w", err)
	}
	text := string(data)
	if rest, ok := strings.CutPrefix(text, "---\n"); ok {
		if _, body, ok := strings.Cut(rest, "\n---\n"); ok {
			text = body
		}
	}
	return strings.TrimSpace(text), nil
}

// builtins are the read-only tools the verifier always has. ToolSearch
// loads the connector tools, which arrive deferred.
var builtins = []string{"Read", "Grep", "Glob", "WebSearch", "WebFetch", "ToolSearch"}

// Size caps on the evidence, in bytes: per tool input, per tool result, per
// earlier assistant text, and in all, the oldest dropped first; and on the
// text to verify, so a page-sized post can't hold its tool call for long.
const (
	inputCap    = 4000
	resultCap   = 20000
	assistCap   = 1500
	evidenceCap = 300000
	textCap     = 40000
)

// threeWords is three or more words on one line, with no question mark. A
// number is a word: "42 tests pass" and "foo.go:12" state counts and places.
const threeWords = `[\pL\pN][\pL\pN'-]*[^\pL\pN?\n]+[\pL\pN][\pL\pN'-]*[^\pL\pN?\n]+[\pL\pN][^?\n]*?`

var (
	fence = regexp.MustCompile("(?ms)^\\s*(```|~~~).*?^\\s*(```|~~~)")
	// statement is three or more words ending a sentence with . or !, or a
	// line with ) or :.
	statement = regexp.MustCompile(`(?m)` + threeWords + `([.!](\s|$)|[):]$)`)
	// item is a list item, table row, or heading of three or more words,
	// which often ends without punctuation.
	item = regexp.MustCompile(`(?m)^\s*([-*+]|\d+[.)]|\||#{1,6})\s*[^?\n]*?` + threeWords + `$`)
	// blockStart is a list item, table row, or heading, which starts a line
	// of its own.
	blockStart = regexp.MustCompile(`^\s*([-*+]\s|\d+[.)]\s|\||#{1,6}\s)`)
	// unjoined is a table row or heading, which no next line continues.
	unjoined = regexp.MustCompile(`^\s*(\||#{1,6}\s)`)
)

// prose is text outside its code fences, each line inside a paragraph
// joined to the one before it, so a wrapped sentence is one statement.
func prose(text string) string {
	var lines []string
	for line := range strings.SplitSeq(fence.ReplaceAllString(text, ""), "\n") {
		if n := len(lines); n > 0 && strings.TrimSpace(line) != "" && strings.TrimSpace(lines[n-1]) != "" &&
			!blockStart.MatchString(line) && !unjoined.MatchString(lines[n-1]) {
			lines[n-1] += " " + strings.TrimSpace(line)
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// Declarative is true when text, outside its code fences, holds a sentence,
// list item, table row, or heading that states something: questions, code,
// and a word or two ("Done.") have nothing to verify.
func Declarative(text string) bool {
	p := prose(text)
	return statement.MatchString(p) || item.MatchString(p)
}

// Statements is each sentence, list item, and table row of text, outside
// its code fences, that states something, trimmed, each once.
func Statements(text string) []string {
	p := prose(text)
	var out []string
	for _, re := range []*regexp.Regexp{item, statement} {
		for _, m := range re.FindAllString(p, -1) {
			if m = strings.TrimSpace(m); !slices.Contains(out, m) {
				out = append(out, m)
			}
		}
	}
	return out
}

// Verdict is the verifier's answer: OK when no claim is unsupported. Usage
// is what the run cost, set on a failed run too.
type Verdict struct {
	OK          bool            `json:"ok"`
	Unsupported []Unsupported   `json:"unsupported"`
	Usage       claudecli.Usage `json:"-"`
}

// ErrOff is Verify's error when kit.yml turns the verifier off.
var ErrOff = errors.New("claims: verifier is off")

// Unsupported is a claim nothing confirmed, and the check that would.
type Unsupported struct {
	Claim string `json:"claim"`
	Check string `json:"check"`
}

// Paused is Verify's error while the verifier waits out a failed run, so
// an outage costs one run's timeout, not one per text.
type Paused struct{ claudecli.Failure }

func (p *Paused) Error() string { return "claims: the verifier is paused after a failed run: " + p.Why }

// verifierKey is the verifier's one key in its cooldown: cfg's command and
// model, so a changed one runs at once.
func verifierKey(cfg *config.Config) string {
	sum := sha256.Sum256([]byte(strings.Join(append(slices.Clone(cfg.Claims.Command), cfg.Claims.Model), "\x00")))
	return "verifier-" + hex.EncodeToString(sum[:])[:16]
}

// Verify checks text from dir: first its bare references, with no model,
// then the verifier on the rest, with the session transcript at
// transcriptPath (none when "") as its evidence and the agent file under
// root as its instructions. An error means no verdict; callers fail open
// on it. A failed run pauses the verifier for claudecli.CooldownPeriod,
// when Verify returns *Paused without running.
func Verify(cfg *config.Config, root, cacheDir, dir, text, transcriptPath string) (Verdict, error) {
	if cfg.Claims.Off() {
		return Verdict{}, ErrOff
	}
	// The transcript is parsed only for a text one of the passes needs it for.
	load := sync.OnceValues(func() ([]transcript.Entry, error) {
		if transcriptPath == "" {
			return nil, nil
		}
		return transcript.Load(transcriptPath)
	})
	// A finding whose evidence the session never touched needs no model.
	if evidenceField.MatchString(text) {
		entries, err := load()
		if err != nil {
			return Verdict{}, err
		}
		if u := ungrounded(dir, entries, text); len(u) > 0 {
			return Verdict{Unsupported: u}, nil
		}
	}
	flagged, rest := settleReferences(dir, text, time.Now().Add(settleBudget))
	if !Declarative(rest) {
		return Verdict{OK: len(flagged) == 0, Unsupported: flagged}, nil
	}
	v, err := verifyWithModel(cfg, root, cacheDir, dir, rest, load, transcriptPath)
	if err == nil {
		v.Unsupported = append(flagged, v.Unsupported...)
		v.OK = v.OK && len(flagged) == 0
	}
	return v, err
}

// verifyWithModel is Verify's model run on text, what the no-model pass
// left, with load's entries, the transcript at transcriptPath: paused after
// a failed run, else one claude -p run.
func verifyWithModel(cfg *config.Config, root, cacheDir, dir, text string, load func() ([]transcript.Entry, error), transcriptPath string) (Verdict, error) {
	cool, key := claudecli.NewCooldown(cacheDir, "claims"), verifierKey(cfg)
	sum := sha256.Sum256([]byte(text))
	textKey := key + "-" + hex.EncodeToString(sum[:])[:16]
	for _, k := range []string{key, textKey} {
		if f, ok := cool.Cooling(k, time.Now()); ok {
			return Verdict{}, &Paused{f}
		}
	}
	entries, err := load()
	if err != nil {
		return Verdict{}, err
	}
	argv := cfg.Claims.Command
	if len(argv) == 0 {
		prompt, err := systemPrompt(root)
		if err != nil {
			return Verdict{}, err
		}
		argv = claude(cfg, root, entries, prompt)
	}
	forked := transcriptPath != "" && fsx.IsFile(transcript.ForkSidecar(transcriptPath))
	v, err := run(cfg, argv, filepath.Join(cacheDir, cache.Guard), dir, evidence(entries, text, forked))
	// Best effort: unrecorded, the next text runs the verifier again. A
	// second text timing out within the cooldown is the verifier hanging,
	// so it pauses all.
	timeoutKey := key + "-timeout"
	if err != nil {
		msg := strings.TrimPrefix(err.Error(), "claims: ")
		failed := map[string]string{key: msg}
		if textFailure(err) {
			failed = map[string]string{textKey: msg}
		}
		if strings.Contains(msg, "timed out") {
			if _, ok := cool.Cooling(timeoutKey, time.Now()); ok {
				failed[key] = msg
			}
			failed[timeoutKey] = msg
		}
		_ = cool.Record(time.Now(), failed, nil)
	} else {
		_ = cool.Record(time.Now(), nil, []string{key, textKey, timeoutKey})
	}
	return v, err
}

// textFailure is whether err is a run failing on the text it was given, its
// budget, its turns, or its time, so only that text waits: the next text
// may well pass. Anything else is the verifier failing, and pauses all.
func textFailure(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "timed out") || strings.Contains(msg, "error_max_budget_usd") || strings.Contains(msg, "error_max_turns")
}

// run runs the verifier command argv from dir on input, the evidence and
// the text to verify, guarded by a marker under markerDir.
func run(cfg *config.Config, argv []string, markerDir, dir, input string) (Verdict, error) {
	answer, usage, err := claudecli.Run(time.Duration(cfg.Claims.Timeout)*time.Second, argv, dir, strings.NewReader(input), markerDir, Guard)
	if err != nil {
		return Verdict{Usage: usage}, fmt.Errorf("claims: %w", err)
	}
	var v Verdict
	if err := json.Unmarshal(answer, &v); err != nil {
		return Verdict{Usage: usage}, fmt.Errorf("claims: malformed answer: %w", err)
	}
	v.OK = v.OK && len(v.Unsupported) == 0
	v.Usage = usage
	return v, nil
}

// claude is the verifier command. --restricted drops every settings file,
// so no allow rule of the user's or the project's applies, and dontAsk
// denies whatever --allowedTools doesn't name: the read-only builtins,
// each read tool the session called, and kit.yml's claims.allow, less any
// write tool.
func claude(cfg *config.Config, root string, entries []transcript.Entry, systemPrompt string) []string {
	allowed := slices.Clone(builtins)
	for _, e := range entries {
		for _, b := range e.ToolUses() {
			if connector.IsRead(cfg.Connectors, b.Name) && !slices.Contains(allowed, b.Name) {
				allowed = append(allowed, b.Name)
			}
		}
	}
	for _, name := range cfg.Claims.Allow {
		// An allow rule of mcp__<server> or mcp__<server>__* grants every
		// tool on the server, so an MCP name needs a read verb of its own.
		if strings.HasPrefix(name, "mcp__") && connector.IsRead(cfg.Connectors, name) && !slices.Contains(allowed, name) {
			allowed = append(allowed, name)
		}
	}
	// --tools names builtins; an MCP tool is reachable through --allowedTools alone.
	tools := slices.DeleteFunc(slices.Clone(allowed), func(n string) bool { return strings.HasPrefix(n, "mcp__") })
	// Bash is there for kit read alone; denyReads keeps Claude Code's
	// unprompted read-only commands off the credential files.
	kitRead := filepath.Join(root, "bin", "kit") + " read"
	allowed = append(allowed, "Bash("+kitRead+" *)")
	systemPrompt += "\n\nRun git and the code host only as `" + kitRead + " git <command>`, `" + kitRead + " gh <command>`, or `" + kitRead + " glab <command>`. It refuses any call that could write."
	argv := []string{"claude", "-p", "--restricted", "--permission-mode", "dontAsk",
		"--tools", strings.Join(append(tools, "Bash"), ","), "--allowedTools", strings.Join(allowed, ","),
		"--disable-slash-commands", "--no-session-persistence", "--system-prompt", systemPrompt,
		"--max-budget-usd", strconv.FormatFloat(cfg.Claims.BudgetUSD, 'f', -1, 64),
		"--output-format", "json", "--json-schema", schema}
	if denied := denyReads(cfg.SensitivePaths); len(denied) > 0 {
		argv = append(argv, "--disallowedTools", strings.Join(denied, ","))
	}
	if cfg.Claims.Model != "" {
		argv = append(argv, "--model", cfg.Claims.Model)
	}
	return argv
}

// denyReads is a Read deny rule per sensitive_paths entry, so neither a
// claim nor an instruction injected into the evidence gets the verifier to
// read a credential file; a Read deny reaches Grep and Glob too
// (code.claude.com/docs/en/permissions). A "~/" entry is under home, a
// directory when it ends in "/"; any other is a basename at any depth from
// the filesystem root, "//", since a bare "**/" covers only the cwd. A
// deny can't be narrowed, so a "!" exception is dropped. Each entry also
// gets a Bash deny on any command naming it, unless it's a glob too short
// to name a file: Claude Code runs cat and git's read forms with no prompt
// in every mode.
func denyReads(entries []string) []string {
	var rules []string
	for _, e := range entries {
		if e == "" || strings.HasPrefix(e, "!") {
			continue
		}
		switch {
		case strings.HasPrefix(e, "~/") && strings.HasSuffix(e, "/"):
			rules = append(rules, "Read("+e+"**)")
		case strings.HasPrefix(e, "~/"):
			rules = append(rules, "Read("+e+")")
		default:
			rules = append(rules, "Read(//**/"+e+")")
		}
		// shortcut: it matches the command's text, so a glob like .en? gets past it.
		literal := strings.Trim(strings.TrimPrefix(e, "~/"), "*")
		if !strings.Contains(e, "*") || len(literal) >= minDenyLiteral {
			rules = append(rules, "Bash(*"+literal+"*)")
		}
	}
	return rules
}

// minDenyLiteral is the shortest literal part of a glob entry that gets a
// Bash deny: "*.pem" keeps ".pem", while "id_*"'s "id_" would match any
// command naming valid_ids.go.
const minDenyLiteral = 4

const schema = `{"type":"object","additionalProperties":false,"required":["ok","unsupported"],"properties":{` +
	`"ok":{"type":"boolean"},"unsupported":{"type":"array","items":{"type":"object","additionalProperties":false,` +
	`"required":["claim","check"],"properties":{"claim":{"type":"string"},"check":{"type":"string"}}}}}}`

// evidence is the verifier's input: the session's prompts, tool calls,
// and results, oldest first, then text. Prompts are always whole. Within
// evidenceCap, each call and result naming what text names (namedIn) is
// kept first, newest first, then the rest newest first until one doesn't
// fit, so every item cut is older than every one kept for its age.
// A forked skill's task is its first meta prompt, kept when forked.
func evidence(entries []transcript.Entry, text string, forked bool) string {
	type item struct {
		text              string
		id                string // a tool call's, or the call's a result answers
		prompt, named, in bool
	}
	names := map[string]string{}  // tool_use id -> tool name
	inputs := map[string]string{} // tool_use id -> its raw input
	tokens := namedIn(text)
	namedIDs := map[string]bool{}
	var items []item
	for _, e := range entries {
		if e.IsMeta {
			if forked && e.Type == "user" {
				items = append(items, item{text: "### Task (forked skill prompt)\n\n" + e.PromptText(), prompt: true})
				forked = false
			}
			continue
		}
		switch {
		case e.StartsTurn():
			items = append(items, item{text: "### User prompt\n\n" + e.PromptText(), prompt: true})
		case e.Type == "user":
			for _, b := range e.Blocks {
				if b.Type == "tool_result" {
					name, out := names[b.ToolUseID], b.ResultText()
					// A subagent's report is claims of its own, not a read.
					report := name == "Agent"
					if report {
						name += " (not evidence)"
					}
					named := !report && slices.ContainsFunc(tokens, func(t string) bool {
						return strings.Contains(inputs[b.ToolUseID], t) || strings.Contains(out, t)
					})
					namedIDs[b.ToolUseID] = namedIDs[b.ToolUseID] || named
					items = append(items, item{text: fmt.Sprintf("#### Result of %s\n\n%s", name, clip(out, resultCap)), id: b.ToolUseID, named: named})
				}
			}
		case e.Type == "assistant":
			for _, b := range e.Blocks {
				switch b.Type {
				case "text":
					items = append(items, item{text: "### Assistant text (not evidence)\n\n" + clip(b.Text, assistCap)})
				case "tool_use":
					names[b.ID], inputs[b.ID] = b.Name, string(b.RawInput)
					items = append(items, item{text: fmt.Sprintf("#### Tool %s\n\nInput: %s", b.Name, clip(string(b.RawInput), inputCap)), id: b.ID})
				}
			}
		}
	}
	budget := evidenceCap
	for i, it := range items {
		items[i].named = it.named || it.id != "" && namedIDs[it.id]
		items[i].in = it.prompt
		if it.prompt {
			budget -= len(it.text) + 2
		}
	}
	fits := func(it item) bool {
		if size := len(it.text) + 2; size <= budget {
			budget -= size
			return true
		}
		return false
	}
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].named && !items[i].prompt {
			items[i].in = fits(items[i])
		}
	}
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].prompt || items[i].named {
			continue
		}
		if !fits(items[i]) {
			break
		}
		items[i].in = true
	}
	cut := 0
	for _, it := range items {
		if !it.in {
			cut++
		}
	}
	var b bytes.Buffer
	b.WriteString("## Session evidence, oldest first\n\n")
	if cut > 0 {
		fmt.Fprintf(&b, "[%d older items cut]\n\n", cut)
	}
	for _, it := range items {
		if it.in {
			b.WriteString(it.text + "\n\n")
		}
	}
	b.WriteString("## Text to verify\n\n" + clip(text, textCap) + "\n")
	return b.String()
}

// manyDigits is a number of two digits or more; a single digit is in
// nearly every result.
var manyDigits = regexp.MustCompile(`\b\d{2,}\b`)

// namedIn is what text names that a tool call or result can show: each
// backticked span, each path, file:line's path, and SHA outside them, and
// each number of two digits or more.
func namedIn(text string) []string {
	var out []string
	for _, m := range backticked.FindAllStringSubmatch(text, -1) {
		out = append(out, lineSuffix.ReplaceAllString(m[1], ""))
	}
	for word := range strings.FieldsSeq(backticked.ReplaceAllString(text, " ")) {
		word = strings.Trim(word, `.,;:!()[]"'|-*+#>`)
		if strings.Contains(word, "/") || citation.MatchString(word) || isSHA(word) {
			out = append(out, lineSuffix.ReplaceAllString(word, ""))
		}
	}
	return slices.DeleteFunc(append(out, manyDigits.FindAllString(text, -1)...), func(t string) bool { return t == "" })
}

// CitedFiles is each regular file under root that text names, by its path
// from root, in order; a path with a comma, "=", or whitespace is left out,
// so it can sit in a ledger line.
func CitedFiles(root, text string) []string {
	var out []string
	for _, name := range namedIn(text) {
		path := fsx.Abs(root, name)
		rel, err := filepath.Rel(root, path)
		if err != nil || !fsx.Within(root, path) || strings.ContainsAny(rel, ",= \t\n") || slices.Contains(out, rel) {
			continue
		}
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			out = append(out, rel)
		}
	}
	return out
}

// clip is s cut to limit bytes, with a note of what was cut.
func clip(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + fmt.Sprintf("\n[... %d bytes cut]", len(s)-limit)
}
