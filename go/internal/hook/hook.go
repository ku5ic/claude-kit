// Package hook is the runtime every Claude Code hook shares: the payload,
// blocking, permission decisions, JSONL logs, and the fail-open contract.
//
// Exit codes follow Claude Code's hook protocol: 0 allows (stdout may carry
// a decision or context), 2 blocks with stderr as the reason shown to
// Claude. Any internal failure exits 0: a hook bug must never block a
// legitimate tool call.
package hook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/extract"
)

// Payload is the hook's stdin JSON. A payload that doesn't parse is kept as
// Raw with Err set, so each check can fail open on its own.
type Payload struct {
	Raw  []byte
	Err  error
	data map[string]any
}

// ParsePayload decodes stdin; an empty or invalid payload sets Err.
func ParsePayload(raw []byte) *Payload {
	p := &Payload{Raw: raw}
	if err := json.Unmarshal(raw, &p.data); err != nil {
		p.Err = fmt.Errorf("payload: %w", err)
	} else if p.data == nil {
		p.Err = fmt.Errorf("payload: not a JSON object")
	}
	return p
}

// Bool is the boolean at a dotted path, false when absent or not a bool.
func (p *Payload) Bool(path string) bool {
	b, _ := extract.GetPath(p.data, path).(bool)
	return b
}

// String is the string at a dotted path, "" when absent or not a string.
func (p *Payload) String(path string) string {
	s, _ := extract.GetPath(p.data, path).(string)
	return s
}

// SessionID is the payload's session_id.
func (p *Payload) SessionID() string { return p.String("session_id") }

// Cwd is the payload's cwd, else the process's working directory.
func (p *Payload) Cwd() string {
	if cwd := p.String("cwd"); cwd != "" {
		return cwd
	}
	cwd, _ := os.Getwd()
	return cwd
}

// FilePath is the tool's target file; Read, Edit, Write, and MultiEdit all
// send it as file_path.
func (p *Payload) FilePath() string {
	return p.String("tool_input.file_path")
}

// Blocked ends a check with exit 2. Returned, not panicked, so a check reads
// top to bottom. Rule is the slug, for kit explain.
type Blocked struct{ Reason, Rule string }

func (b *Blocked) Error() string { return b.Reason }

// Hook is one hook invocation.
type Hook struct {
	Name    string   // e.g. "guard-edit"; names the hook in blocks and logs
	Args    []string // arguments after the hook name in hooks.json
	Payload *Payload
	Paths   config.Paths
	Stdout  io.Writer
	Stderr  io.Writer
	Now     func() time.Time
	// DryRun skips every log write: kit explain evaluates without leaving
	// a trace in guards.jsonl.
	DryRun bool

	cfg      *config.Config
	warnings []config.Warning
	loaded   bool
	context  string // printed after a block reason, e.g. "Path: <path>"
	decision string // the strongest Decide so far; Run prints it once
	reason   string
}

// Config loads kit.yml on first use; hooks that never need it never pay.
// A config that fails to load is nil: callers treat that as "nothing
// configured" and fail open.
func (h *Hook) Config() *config.Config {
	if !h.loaded {
		h.loaded = true
		cfg, warnings, err := config.Load(h.Paths)
		h.warnings = warnings
		if err == nil {
			h.cfg = cfg
		} else {
			h.warnings = append(h.warnings, baseNotLoaded(h.Paths.Base, warnings, err))
		}
	}
	return h.cfg
}

// baseNotLoaded says the guards run without config, adding Load's error
// only when no warning already names kit.yml with it.
func baseNotLoaded(base string, warnings []config.Warning, err error) config.Warning {
	const off = "not loaded, so every guard runs with no config"
	for _, w := range warnings {
		if w.File == base {
			return config.Warning{File: base, Err: errors.New(off)}
		}
	}
	return config.Warning{File: base, Err: fmt.Errorf("%s: %w", off, err)}
}

// Warnings are the problems Config found loading kit.yml and the overlay.
func (h *Hook) Warnings() []config.Warning {
	h.Config()
	return h.warnings
}

// SetConfig injects a config, for tests and for dispatchers that load once.
func (h *Hook) SetConfig(cfg *config.Config) { h.cfg, h.loaded = cfg, true }

// SetContext sets the line printed under every block reason.
func (h *Hook) SetContext(context string) { h.context = context }

// ruleDisabled is true when rule is in kit.yml's disabled_rules.
func (h *Hook) ruleDisabled(rule string) bool {
	cfg := h.Config()
	return cfg != nil && slices.Contains(cfg.DisabledRules, rule)
}

// Block returns a *Blocked for rule, logging it to guards.jsonl. A rule in
// disabled_rules is logged as disabled and returns nil, so the check carries
// on as if it hadn't matched.
func (h *Hook) Block(reason, rule string) error {
	if rule != "" && h.ruleDisabled(rule) {
		h.Log(GuardsLog, "disabled", "rule", rule)
		return nil
	}
	h.Log(GuardsLog, "block", "rule", rule)
	msg := "Blocked by " + h.Name + ": " + reason
	if h.context != "" {
		msg += "\n" + h.context
	}
	return &Blocked{msg, rule}
}

// Decide records a PreToolUse permission decision (allow or ask). Several
// checks in one run may decide: ask outranks allow, and asks join their
// reasons. Run prints the result once, since stdout takes one JSON object.
func (h *Hook) Decide(decision, reason string) {
	switch {
	case h.decision == "" || decision == "ask" && h.decision == "allow":
		h.decision, h.reason = decision, reason
	case decision == "ask" && h.decision == "ask":
		h.reason += "; " + reason
	}
}

// Decision is the permission decision and reason recorded so far, "" when
// no check decided.
func (h *Hook) Decision() (decision, reason string) { return h.decision, h.reason }

func (h *Hook) printDecision() {
	if h.decision == "" {
		return
	}
	type specific struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	}
	WriteJSON(h.Stdout, struct {
		HookSpecificOutput specific `json:"hookSpecificOutput"`
	}{specific{"PreToolUse", h.decision, h.reason}})
}

// AddContext prints context for Claude on event, plus systemMessage for the
// user when it isn't empty.
func AddContext(w io.Writer, event, systemMessage, context string) {
	type specific struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	}
	WriteJSON(w, struct {
		SystemMessage      string   `json:"systemMessage,omitempty"`
		HookSpecificOutput specific `json:"hookSpecificOutput"`
	}{systemMessage, specific{event, context}})
}

// WriteJSON writes v as one compact line with <, >, and & left as they are:
// hook output often carries <tag> blocks, which json.Marshal would escape.
func WriteJSON(w io.Writer, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if enc.Encode(v) == nil {
		_, _ = w.Write(buf.Bytes())
	}
}

// The kit's JSONL logs, the skills log's events that mark a skill as only
// surfaced, and a line's ts layout.
const (
	SkillsLog           = "skills"
	GuardsLog           = "guards"
	EventRequiredSkill  = "required-skill"
	EventSuggestedSkill = "suggested-skill"
	TimeLayout          = "2006-01-02T15:04:05Z"
)

// Entry is one log line as the readers use it; a pointer keeps a null
// distinct from "".
type Entry struct {
	TS            *string `json:"ts"`
	Event         string  `json:"event"`
	SessionID     *string `json:"session_id"`
	ExpansionType string  `json:"expansion_type"`
	CommandName   *string `json:"command_name"`
	SkillFile     *string `json:"skill_file"`
	ToolName      string  `json:"tool_name"`
	Rule          *string `json:"rule"`
}

// Surfaced is true for a skills log line that says a skill was shown, not
// loaded.
func (e Entry) Surfaced() bool {
	return e.Event == EventRequiredSkill || e.Event == EventSuggestedSkill
}

// Log appends one line to <log dir>/<log>.jsonl: ts, hook, event, the
// payload's session_id, then each key/value pair in order, an empty value
// as null. Never fails its caller: a log that can't be written is skipped.
func (h *Hook) Log(log, event string, pairs ...string) {
	if h.DryRun {
		return
	}
	fields := []string{
		"ts", h.now().UTC().Format(TimeLayout),
		"hook", h.Name,
		"event", event,
		"session_id", h.Payload.SessionID(),
	}
	fields = append(fields, pairs...)

	var line bytes.Buffer
	line.WriteByte('{')
	for i := 0; i+1 < len(fields); i += 2 {
		if i > 0 {
			line.WriteByte(',')
		}
		writeJSONValue(&line, fields[i])
		line.WriteByte(':')
		if fields[i+1] == "" {
			line.WriteString("null")
		} else {
			writeJSONValue(&line, fields[i+1])
		}
	}
	line.WriteString("}\n")

	dir := h.Paths.LogDir()
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	f, err := os.OpenFile(h.Paths.LogFile(log), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(line.Bytes())
}

func writeJSONValue(buf *bytes.Buffer, s string) {
	var tmp bytes.Buffer
	enc := json.NewEncoder(&tmp)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // a string always encodes
	buf.Write(bytes.TrimSuffix(tmp.Bytes(), []byte("\n")))
}

func (h *Hook) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// FailOpen prints the notice for a hook that allows because it failed.
func FailOpen(stderr io.Writer, name string) {
	fmt.Fprintf(stderr, "%s: unexpected error, failing open\n", name)
}

// Check is one policy check. It returns *Blocked to block, nil to allow, and
// any other error to fail open.
type Check func(*Hook) error

// RunCheck runs check under name, failing open on a panic or error: the
// notice goes to stderr, which only the debug log sees, so it is also logged
// to guards.jsonl, and the result is "allow". It reports whether the check
// blocked, having printed the block to stderr.
func RunCheck(h *Hook, name string, check Check) (blocked bool) {
	saved := h.Name
	h.Name = name
	failOpen := func() {
		FailOpen(h.Stderr, name)
		h.Log(GuardsLog, "fail-open")
	}
	defer func() {
		if r := recover(); r != nil {
			failOpen()
			blocked = false
		}
		h.Name = saved
	}()
	err := check(h)
	if err == nil {
		return false
	}
	if b, ok := err.(*Blocked); ok {
		fmt.Fprintln(h.Stderr, b.Reason)
		return true
	}
	failOpen()
	return false
}

// Run executes checks in order against one payload read, the dispatcher
// contract: each check is isolated (one failing open never skips the next),
// and the first block ends the run with exit 2.
func Run(h *Hook, checks ...NamedCheck) int {
	for _, c := range checks {
		if RunCheck(h, c.Name, c.Check) {
			return 2
		}
	}
	h.printDecision()
	return 0
}

// NamedCheck pairs a check with the hook name it reports under.
type NamedCheck struct {
	Name  string
	Check Check
}
