package hook

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/ku5ic/claude-kit/go/internal/sources"
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
	b, _ := sources.GetPath(p.data, path).(bool)
	return b
}

// Object is the object at a dotted path, nil when absent or not an object.
func (p *Payload) Object(path string) map[string]any {
	m, _ := sources.GetPath(p.data, path).(map[string]any)
	return m
}

// String is the string at a dotted path, "" when absent or not a string.
func (p *Payload) String(path string) string {
	s, _ := sources.GetPath(p.data, path).(string)
	return s
}

// Strings is every string at or under a dotted path, maps in key order,
// passing over any map key named in skip.
func (p *Payload) Strings(path string, skip ...string) []string {
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case string:
			out = append(out, v)
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			for _, k := range slices.Sorted(maps.Keys(v)) {
				if !slices.Contains(skip, k) {
					walk(v[k])
				}
			}
		}
	}
	walk(sources.GetPath(p.data, path))
	return out
}

// SessionID is the payload's session_id.
func (p *Payload) SessionID() string { return p.String(KeySessionID) }

// Cwd is the payload's cwd, else the process's working directory.
func (p *Payload) Cwd() string {
	if cwd := p.String(KeyCwd); cwd != "" {
		return cwd
	}
	cwd, _ := os.Getwd()
	return cwd
}

// Edit is one replacement an Edit or MultiEdit makes.
type Edit struct {
	Old, New   string
	ReplaceAll bool
}

// Edits is an Edit's replacement, or a MultiEdit's edits in order; nil for
// any other tool.
func (p *Payload) Edits() []Edit {
	switch p.String(KeyToolName) {
	case "Edit":
		return []Edit{{p.String(KeyInputOldString), p.String(KeyInputNewString), p.Bool(KeyInputReplaceAll)}}
	case "MultiEdit":
		items, _ := sources.GetPath(p.data, KeyInputEdits).([]any)
		var edits []Edit
		for _, item := range items {
			m, _ := item.(map[string]any)
			old, _ := m["old_string"].(string)
			repl, _ := m["new_string"].(string)
			all, _ := m["replace_all"].(bool)
			edits = append(edits, Edit{old, repl, all})
		}
		return edits
	}
	return nil
}

// FilePath is the tool's target file: file_path from Read, Edit, Write, and
// MultiEdit, notebook_path from NotebookEdit.
func (p *Payload) FilePath() string {
	return cmp.Or(p.String(KeyInputFilePath), p.String(KeyInputNotebookPath))
}
