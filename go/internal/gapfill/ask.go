package gapfill

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// roles and kinds are the answers an entry, a segment, or a proposal may
// give; only a check has a kind.
var (
	roles = []string{"check", "fixer", "setup", "build", "deploy", "dev", "e2e", "other"}
	kinds = []string{"lint", "typecheck", "test", "format-check", "deadcode", "security"}
)

//go:embed prompt.txt
var systemPrompt string

// budgetUSD caps one classifier call's spend.
const budgetUSD = "0.50"

// request is the classifier's stdin. Facts asks for managers and
// proposals too: the project changed since they were answered.
type request struct {
	Entries []sources.Entry
	View    view
	Facts   bool
}

func (r request) MarshalJSON() ([]byte, error) {
	type entry struct {
		ID        string   `json:"id"`
		Source    string   `json:"source"`
		File      string   `json:"file"`
		Name      string   `json:"name"`
		Dir       string   `json:"dir"`
		Stage     string   `json:"stage,omitempty"`
		Body      string   `json:"body"`
		Files     []string `json:"files,omitempty"`
		PassFiles bool     `json:"pass_files,omitempty"`
	}
	entries := []entry{}
	for _, e := range r.Entries {
		entries = append(entries, entry{Key(e), e.Source, e.File, e.Name, e.Dir, e.Stage, e.Command(), e.Files, e.PassFiles})
	}
	return json.Marshal(struct {
		Entries      []entry           `json:"entries"`
		Facts        bool              `json:"ask_project_facts"`
		Files        []string          `json:"files"`
		Contents     map[string]string `json:"contents"`
		LockfileDirs []lockDir         `json:"lockfile_dirs"`
	}{entries, r.Facts, r.View.Files, r.View.Contents, r.View.LockfileDirs})
}

// response is the classifier's structured answer.
type response struct {
	Entries []struct {
		ID string `json:"id"`
		Verdict
	} `json:"entries"`
	Managers  []Manager  `json:"managers"`
	Proposals []Proposal `json:"proposals"`
}

// classify runs the classifier on req: claude -p with no tools, no MCP, no
// user settings, and a fixed prompt, from the cache dir, unless kit.yml's
// classifier replaces it. Its stdout is claude's --output-format json
// envelope.
func classify(ctx context.Context, cfg *config.Config, o Options, req request) (response, error) {
	stdin, err := json.Marshal(req)
	if err != nil {
		return response{}, err
	}
	argv := cfg.Classifier
	if len(argv) == 0 {
		argv = claude()
	}
	deadline, _ := ctx.Deadline()
	cmd := proc.Command(time.Until(deadline), argv[0], argv[1:]...)
	cmd.Dir = filepath.Join(o.CacheDir, cache.Enforce)
	cmd.Env = append(os.Environ(), Guard+"=1")
	cmd.Stdin = bytes.NewReader(stdin)
	out, err := cmd.Output()
	if cmd.TimedOut() {
		return response{}, errors.New("timed out")
	}
	if err != nil {
		return response{}, err
	}
	var envelope struct {
		IsError    bool            `json:"is_error"`
		Subtype    string          `json:"subtype"`
		Structured json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return response{}, fmt.Errorf("malformed output: %w", err)
	}
	if envelope.IsError || len(envelope.Structured) == 0 {
		return response{}, fmt.Errorf("no answer (%s)", envelope.Subtype)
	}
	var resp response
	if err := json.Unmarshal(envelope.Structured, &resp); err != nil {
		return response{}, fmt.Errorf("malformed answer: %w", err)
	}
	return resp, nil
}

// claude is the default classifier command.
func claude() []string {
	return []string{"claude", "-p", "--model", "haiku", "--tools", "", "--strict-mcp-config", "--disable-slash-commands",
		"--setting-sources", "project", "--system-prompt", systemPrompt, "--no-session-persistence",
		"--max-budget-usd", budgetUSD, "--output-format", "json", "--json-schema", schema()}
}

// schema is the answer's JSON Schema, built from roles and kinds.
func schema() string {
	str := map[string]any{"type": "string"}
	list := map[string]any{"type": "array", "items": str}
	object := func(required []string, props map[string]any) map[string]any {
		return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": props}
	}
	array := func(items map[string]any) map[string]any { return map[string]any{"type": "array", "items": items} }
	role, kind := map[string]any{"enum": roles}, map[string]any{"enum": kinds}
	b, _ := json.Marshal(object([]string{"entries", "managers", "proposals"}, map[string]any{
		"entries": array(object([]string{"id", "role", "mutates"}, map[string]any{
			"id": str, "role": role, "kind": kind, "mutates": map[string]any{"type": "boolean"},
			"segments":  array(object([]string{"text", "role"}, map[string]any{"text": str, "role": role, "kind": kind})),
			"file_form": str, "globs": list, "affected_form": str, "exclusions": str,
		})),
		"managers": array(object([]string{"dir", "cites", "manager", "run_prefix", "add_verbs", "dlx", "dir_flags", "rivals"}, map[string]any{
			"dir": str, "cites": str, "manager": str, "run_prefix": str, "add_verbs": list, "dlx": str, "dir_flags": list, "rivals": list,
		})),
		"proposals": array(object([]string{"role", "command", "dir", "evidence", "mutates"}, map[string]any{
			"role": map[string]any{"enum": []string{"check", "fixer"}}, "kind": kind, "command": str, "file_form": str,
			"globs": list, "dir": str, "evidence": str, "mutates": map[string]any{"type": "boolean"},
		})),
	}))
	return string(b)
}
