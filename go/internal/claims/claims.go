// Package claims checks the factual claims in a text against the session's
// evidence before the text reaches anyone: a claude -p verifier reads the
// session's tool results, checks what they don't cover with read-only
// tools, and names every claim nothing confirms.
package claims

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/connector"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// Guard is set in the verifier's environment, so its own session runs no
// kit hook and can't recurse into another verification.
const Guard = "KIT_VERIFIER"

//go:embed prompt.txt
var systemPrompt string

// builtins are the read-only tools the verifier always has. ToolSearch
// loads the connector tools, which arrive deferred.
var builtins = []string{"Read", "Grep", "Glob", "WebSearch", "ToolSearch"}

// Size caps on the evidence, in bytes: per tool input, per tool result, per
// earlier assistant text, and in all, the oldest dropped first.
const (
	inputCap    = 4000
	resultCap   = 20000
	assistCap   = 1500
	evidenceCap = 300000
)

var (
	fence = regexp.MustCompile("(?ms)^\\s*(```|~~~).*?^\\s*(```|~~~)")
	// statement is three or more words ending a sentence with . or !.
	statement = regexp.MustCompile(`\pL[\pL\pN'-]*[^\pL\pN?\n]+\pL[\pL\pN'-]*[^\pL\pN?\n]+\pL[^?\n]*?[.!](\s|$)`)
)

// Declarative is true when text, outside its code fences, holds a sentence
// that states something: questions, code, and a word or two ("Done.") have
// nothing to verify.
func Declarative(text string) bool {
	return statement.MatchString(fence.ReplaceAllString(text, ""))
}

// Verdict is the verifier's answer: OK when no claim is unsupported.
type Verdict struct {
	OK          bool          `json:"ok"`
	Unsupported []Unsupported `json:"unsupported"`
}

// Unsupported is a claim nothing confirmed, and the check that would.
type Unsupported struct {
	Claim string `json:"claim"`
	Check string `json:"check"`
}

// Verify runs the verifier on text from dir, with the session transcript
// at transcriptPath (none when "") as its evidence. An error means no
// verdict; callers fail open on it.
func Verify(cfg *config.Config, dir, text, transcriptPath string) (Verdict, error) {
	if cfg.Claims.Off() {
		return Verdict{}, errors.New("claims: verifier is off")
	}
	var entries []transcript.Entry
	if transcriptPath != "" {
		var err error
		if entries, err = transcript.Load(transcriptPath); err != nil {
			return Verdict{}, err
		}
	}
	argv := cfg.Claims.Command
	if len(argv) == 0 {
		argv = claude(cfg, entries)
	}
	return run(cfg, argv, dir, evidence(entries, text))
}

// run runs the verifier command argv from dir on input, the evidence and
// the text to verify.
func run(cfg *config.Config, argv []string, dir, input string) (Verdict, error) {
	cmd := proc.Command(time.Duration(cfg.Claims.Timeout)*time.Second, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), Guard+"=1")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if cmd.TimedOut() {
		return Verdict{}, errors.New("claims: timed out")
	}
	if err != nil {
		return Verdict{}, fmt.Errorf("claims: %w", err)
	}
	var envelope struct {
		IsError    bool            `json:"is_error"`
		Subtype    string          `json:"subtype"`
		Structured json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return Verdict{}, fmt.Errorf("claims: malformed output: %w", err)
	}
	if envelope.IsError || len(envelope.Structured) == 0 {
		return Verdict{}, fmt.Errorf("claims: no answer (%s)", envelope.Subtype)
	}
	var v Verdict
	if err := json.Unmarshal(envelope.Structured, &v); err != nil {
		return Verdict{}, fmt.Errorf("claims: malformed answer: %w", err)
	}
	v.OK = v.OK && len(v.Unsupported) == 0
	return v, nil
}

// claude is the verifier command. --restricted drops every settings file,
// so no allow rule of the user's or the project's applies, and dontAsk
// denies whatever --allowedTools doesn't name: the read-only builtins,
// each read tool the session called, and kit.yml's claims.allow, less any
// write tool.
func claude(cfg *config.Config, entries []transcript.Entry) []string {
	allowed := slices.Clone(builtins)
	for _, e := range entries {
		for _, b := range e.ToolUses() {
			if connector.IsRead(cfg.Connectors, b.Name) && !slices.Contains(allowed, b.Name) {
				allowed = append(allowed, b.Name)
			}
		}
	}
	for _, name := range cfg.Claims.Allow {
		if name != "" && !connector.IsWrite(cfg.Connectors, name) && !slices.Contains(allowed, name) {
			allowed = append(allowed, name)
		}
	}
	// --tools names builtins; an MCP tool is reachable through --allowedTools alone.
	tools := slices.DeleteFunc(slices.Clone(allowed), func(n string) bool { return strings.HasPrefix(n, "mcp__") })
	argv := []string{"claude", "-p", "--restricted", "--permission-mode", "dontAsk",
		"--tools", strings.Join(tools, ","), "--allowedTools", strings.Join(allowed, ","),
		"--disable-slash-commands", "--no-session-persistence", "--system-prompt", systemPrompt,
		"--max-budget-usd", strconv.FormatFloat(cfg.Claims.BudgetUSD, 'f', -1, 64),
		"--output-format", "json", "--json-schema", schema}
	if cfg.Claims.Model != "" {
		argv = append(argv, "--model", cfg.Claims.Model)
	}
	return argv
}

const schema = `{"type":"object","additionalProperties":false,"required":["ok","unsupported"],"properties":{` +
	`"ok":{"type":"boolean"},"unsupported":{"type":"array","items":{"type":"object","additionalProperties":false,` +
	`"required":["claim","check"],"properties":{"claim":{"type":"string"},"check":{"type":"string"}}}}}}`

// evidence is the verifier's input: the session's prompts, tool calls,
// and results, oldest first and the oldest cut past evidenceCap, then text.
func evidence(entries []transcript.Entry, text string) string {
	names := map[string]string{} // tool_use id -> tool name
	var items []string
	for _, e := range entries {
		if e.IsMeta {
			continue
		}
		switch {
		case e.StartsTurn():
			items = append(items, "### User prompt\n\n"+e.PromptText())
		case e.Type == "user":
			for _, b := range e.Blocks {
				if b.Type == "tool_result" {
					items = append(items, fmt.Sprintf("#### Result of %s\n\n%s", names[b.ToolUseID], clip(b.ResultText(), resultCap)))
				}
			}
		case e.Type == "assistant":
			for _, b := range e.Blocks {
				switch b.Type {
				case "text":
					items = append(items, "### Assistant text (not evidence)\n\n"+clip(b.Text, assistCap))
				case "tool_use":
					names[b.ID] = b.Name
					items = append(items, fmt.Sprintf("#### Tool %s\n\nInput: %s", b.Name, clip(string(b.RawInput), inputCap)))
				}
			}
		}
	}
	cut, size := 0, 0
	for i := len(items) - 1; i >= 0; i-- {
		if size += len(items[i]) + 2; size > evidenceCap {
			cut = i + 1
			break
		}
	}
	var b bytes.Buffer
	b.WriteString("## Session evidence, oldest first\n\n")
	if cut > 0 {
		fmt.Fprintf(&b, "[%d older items cut]\n\n", cut)
	}
	for _, item := range items[cut:] {
		b.WriteString(item + "\n\n")
	}
	b.WriteString("## Text to verify\n\n" + text + "\n")
	return b.String()
}

// clip is s cut to limit bytes, with a note of what was cut.
func clip(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + fmt.Sprintf("\n[... %d bytes cut]", len(s)-limit)
}
