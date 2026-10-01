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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
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
	b, _ := p.value(path).(bool)
	return b
}

func (p *Payload) value(path string) any {
	var value any = p.data
	for _, part := range strings.Split(path, ".") {
		m, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = m[part]
	}
	return value
}

// String is the string at a dotted path, "" when absent or not a string.
func (p *Payload) String(path string) string {
	s, _ := p.value(path).(string)
	return s
}

// Alt is jq's `a // b // ...`: the first path whose value is neither null
// nor false, as a string ("" is a value and wins).
func (p *Payload) Alt(paths ...string) string {
	for _, path := range paths {
		switch v := p.value(path).(type) {
		case nil:
		case bool:
			if v {
				return "true"
			}
		case string:
			return v
		default:
			return fmt.Sprint(v)
		}
	}
	return ""
}

// FilePath is the tool's target file: file_path, else path, else target_file.
func (p *Payload) FilePath() string {
	return p.Alt("tool_input.file_path", "tool_input.path", "tool_input.target_file")
}

// Blocked ends a check with exit 2. Returned, not panicked, so a check reads
// top to bottom. Rule is the slug, for kit explain.
type Blocked struct{ Reason, Rule string }

func (b *Blocked) Error() string { return b.Reason }

// Hook is one hook invocation.
type Hook struct {
	Name    string // e.g. "guard-edit.sh"; names the hook in blocks and logs
	Payload *Payload
	Paths   config.Paths
	Stdout  io.Writer
	Stderr  io.Writer
	Now     func() time.Time
	// DryRun skips every log write: kit explain evaluates without leaving
	// a trace in guards.jsonl.
	DryRun bool

	cfg     *config.Config
	loaded  bool
	context string // printed after a block reason, e.g. "Path: <path>"
}

// Config loads kit.yml on first use; hooks that never need it never pay.
// A config that fails to load is nil: callers treat that as "nothing
// configured" and fail open.
func (h *Hook) Config() *config.Config {
	if !h.loaded {
		h.loaded = true
		cfg, _, err := config.Load(h.Paths)
		if err == nil {
			h.cfg = cfg
		}
	}
	return h.cfg
}

// SetConfig injects a config, for tests and for dispatchers that load once.
func (h *Hook) SetConfig(cfg *config.Config) { h.cfg, h.loaded = cfg, true }

// SetContext sets the line printed under every block reason.
func (h *Hook) SetContext(context string) { h.context = context }

// RuleDisabled is true when rule is in kit.yml's disabled_rules.
func (h *Hook) RuleDisabled(rule string) bool {
	cfg := h.Config()
	return cfg != nil && slices.Contains(cfg.DisabledRules, rule)
}

// Block returns a *Blocked for rule, logging it to guards.jsonl. A rule in
// disabled_rules is logged as disabled and returns nil, so the check carries
// on as if it hadn't matched.
func (h *Hook) Block(reason, rule string) error {
	if rule != "" && h.RuleDisabled(rule) {
		h.Log("guards", "disabled", "rule", rule)
		return nil
	}
	h.Log("guards", "block", "rule", rule)
	msg := "Blocked by " + h.Name + ": " + reason
	if h.context != "" {
		msg += "\n" + h.context
	}
	return &Blocked{msg, rule}
}

// Decide prints a PreToolUse permission decision (allow or ask).
func (h *Hook) Decide(decision, reason string) {
	type specific struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	}
	out := struct {
		HookSpecificOutput specific `json:"hookSpecificOutput"`
	}{specific{"PreToolUse", decision, reason}}
	WriteJSON(h.Stdout, out)
}

// WriteJSON writes v as one compact line, <, >, and & left as they are (as
// jq -c prints them; hook output often carries <tag> blocks).
func WriteJSON(w io.Writer, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if enc.Encode(v) == nil {
		w.Write(buf.Bytes())
	}
}

// Log appends one line to <log dir>/<log>.jsonl: ts, hook, event, the
// payload's session_id, then each key/value pair in order, an empty value
// as null. Never fails its caller: a log that can't be written is skipped.
func (h *Hook) Log(log, event string, pairs ...string) {
	if h.DryRun {
		return
	}
	fields := []string{
		"ts", h.now().UTC().Format("2006-01-02T15:04:05Z"),
		"hook", h.Name,
		"event", event,
		"session_id", h.Payload.String("session_id"),
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
	f, err := os.OpenFile(filepath.Join(dir, log+".jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(line.Bytes())
}

func writeJSONValue(buf *bytes.Buffer, s string) {
	var tmp bytes.Buffer
	enc := json.NewEncoder(&tmp)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	buf.Write(bytes.TrimSuffix(tmp.Bytes(), []byte("\n")))
}

func (h *Hook) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// Check is one policy check. It returns *Blocked to block, nil to allow, and
// any other error to fail open.
type Check func(*Hook) error

// RunCheck runs check under name, failing open on a panic or error: the
// notice goes to stderr and the result is "allow". It reports whether the
// check blocked, having printed the block to stderr.
func RunCheck(h *Hook, name string, check Check) (blocked bool) {
	saved := h.Name
	h.Name = name
	defer func() {
		h.Name = saved
		if r := recover(); r != nil {
			fmt.Fprintf(h.Stderr, "%s: unexpected error, failing open\n", name)
			blocked = false
		}
	}()
	err := check(h)
	if err == nil {
		return false
	}
	if b, ok := err.(*Blocked); ok {
		fmt.Fprintln(h.Stderr, b.Reason)
		return true
	}
	fmt.Fprintf(h.Stderr, "%s: unexpected error, failing open\n", name)
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
	return 0
}

// NamedCheck pairs a check with the hook name it reports under.
type NamedCheck struct {
	Name  string
	Check Check
}
