package gapfill

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claudecli"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// roles and kinds are the answers an entry, a segment, or a proposal may
// give; only a check has a kind.
var (
	roles = []string{"check", "fixer", "setup", "build", "deploy", "dev", "e2e", "other"}
	kinds = config.CheckKinds
)

//go:embed prompt.txt
var systemPrompt string

// budgetUSD caps one classifier call's spend.
const budgetUSD = "0.50"

// request is the classifier's stdin. Facts asks for managers and
// proposals too: the project changed since they were answered.
type request struct {
	Entries []sources.Entry
	// Others are the project's entries outside Entries, in their
	// directories, sent unasked so covered_by can name one.
	Others []sources.Entry
	View   view
	Facts  bool
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
	encode := func(es []sources.Entry) []entry {
		out := []entry{}
		for _, e := range es {
			out = append(out, entry{Key(e), e.Source, e.File, e.Name, e.Dir, e.Stage, e.Command(), e.Files, e.PassFiles})
		}
		return out
	}
	entries, others := encode(r.Entries), encode(r.Others)
	return json.Marshal(struct {
		Entries      []entry           `json:"entries"`
		Others       []entry           `json:"other_entries"`
		Facts        bool              `json:"ask_project_facts"`
		Files        []string          `json:"files"`
		Contents     map[string]string `json:"contents"`
		LockfileDirs []lockDir         `json:"lockfile_dirs"`
	}{entries, others, r.Facts, r.View.Files, r.View.Contents, r.View.LockfileDirs})
}

// response is the classifier's structured answer.
type response struct {
	Entries   []entryVerdict `json:"entries"`
	Managers  []Manager      `json:"managers"`
	Proposals []Proposal     `json:"proposals"`
}

type entryVerdict struct {
	ID string `json:"id"`
	Verdict
}

// classify runs the classifier on req: claude -p with no tools, no MCP, no
// user settings, and a fixed prompt, from the cache dir, unless kit.yml's
// classifier replaces it. Its stdout is claude's --output-format json
// envelope.
func classify(ctx context.Context, cfg *config.Config, o Options, req request) (response, claudecli.Usage, error) {
	stdin, err := json.Marshal(req)
	if err != nil {
		return response{}, claudecli.Usage{}, err
	}
	argv := cfg.Classifier
	if len(argv) == 0 {
		argv = claude()
	}
	deadline, _ := ctx.Deadline()
	answer, usage, err := claudecli.Run(time.Until(deadline), argv, filepath.Join(o.CacheDir, cache.Enforce), bytes.NewReader(stdin),
		filepath.Join(o.CacheDir, cache.Guard), Guard)
	if err != nil {
		return response{}, usage, err
	}
	var resp response
	if err := json.Unmarshal(answer, &resp); err != nil {
		return response{}, usage, fmt.Errorf("malformed answer: %w", err)
	}
	// A verdict counts only for an entry whose body req sent.
	resp.Entries = slices.DeleteFunc(resp.Entries, func(a entryVerdict) bool {
		return !slices.ContainsFunc(req.Entries, func(e sources.Entry) bool { return Key(e) == a.ID })
	})
	return resp, usage, nil
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
			"file_form": str, "globs": list, "affected_form": str, "covered_by": str,
			"also_kinds": array(kind), "reaches": list,
		})),
		"managers": array(object([]string{"dir", "cites", "manager", "run_prefix", "add_verbs", "dlx", "dir_flags", "rivals"}, map[string]any{
			"dir": str, "cites": str, "manager": str, "run_prefix": str, "add_verbs": list, "dlx": str, "dir_flags": list, "rivals": list,
		})),
		"proposals": array(object([]string{"role", "command", "dir", "evidence"}, map[string]any{
			"role": map[string]any{"enum": []string{"check", "fixer"}}, "kind": kind, "command": str, "file_form": str,
			"globs": list, "dir": str, "evidence": str,
		})),
	}))
	return string(b)
}

// askRequests is the classifier calls a run makes: the facts in a call of
// their own, since they take the model longest, then each batch of the
// entries the store lacks. coverage keeps a coverer only in its
// claimant's directory.
func askRequests(cfg *config.Config, o Options, p view, s store) []request {
	var reqs []request
	if s.Context != p.Key && !s.held(cfg, factsKey(p.Key), time.Now()) {
		reqs = append(reqs, request{View: p, Facts: true})
	}
	allowed := slices.DeleteFunc(slices.Clone(o.Entries), func(e sources.Entry) bool { return denied(cfg, e) != "" })
	for batch := range slices.Chunk(s.missing(cfg, o.Entries), batchSize) {
		asked, dirs := map[string]bool{}, map[string]bool{}
		for _, e := range batch {
			asked[Key(e)], dirs[e.Dir] = true, true
		}
		others := slices.DeleteFunc(slices.Clone(allowed), func(e sources.Entry) bool { return asked[Key(e)] || !dirs[e.Dir] })
		reqs = append(reqs, request{Entries: batch, Others: others, View: p})
	}
	return reqs
}

// take stores each answered request's verdicts and facts, and returns the
// first failure, the keys whose call failed, for the cooldown, and those
// answered. A call the classifier wasn't asked gets no cooldown: the next
// run asks it.
func (s *store) take(cfg *config.Config, viewKey string, reqs []request, resps []response, errs []error, now time.Time) (failure string, failed map[string]string, answered []string) {
	failed = map[string]string{}
	notAsked := false
	for i, resp := range resps {
		switch {
		case errors.Is(errs[i], errNotAsked):
			notAsked = true
		case errs[i] != nil:
			if failure == "" {
				failure = "classifier: " + errs[i].Error()
			}
			for _, key := range reqs[i].keys(cfg, viewKey) {
				failed[key] = errs[i].Error()
			}
		default:
			answered = append(answered, s.answer(cfg, viewKey, reqs[i], resp, now)...)
		}
	}
	if failure == "" && notAsked {
		failure = "classifier: " + errNotAsked.Error()
	}
	return failure, failed, answered
}

// keys is the cooldown keys of what r asks: its entries, and the facts.
func (r request) keys(cfg *config.Config, viewKey string) []string {
	var keys []string
	for _, e := range r.Entries {
		keys = append(keys, failKey(cfg, Key(e)))
	}
	if r.Facts {
		keys = append(keys, failKey(cfg, factsKey(viewKey)))
	}
	return keys
}

// answer stores resp, r's answer, and returns the cooldown keys it
// answered.
func (s *store) answer(cfg *config.Config, viewKey string, r request, resp response, now time.Time) []string {
	var answered []string
	for _, a := range resp.Entries {
		s.Entries[a.ID] = &answer{Verdict: a.Verdict, Seen: now}
		answered = append(answered, failKey(cfg, a.ID))
	}
	if r.Facts {
		s.Context, s.Managers, s.Proposals = viewKey, resp.Managers, resp.Proposals
		answered = append(answered, failKey(cfg, factsKey(viewKey)))
	}
	return answered
}
