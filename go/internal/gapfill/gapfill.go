// Package gapfill answers what a project's configs leave open: each
// entry's role, its check kind, whether it mutates, and its file-scoped and
// affected-only forms; each lockfile directory's package-manager facts; and
// checks for the kinds nothing enforces. A model answers once per entry
// body and the answers are cached; every read verifies them against the
// project's files and the kit's policy, so only verified answers count.
package gapfill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Guard is set in the classifier's environment. A kit started under it
// never asks, and its hooks do nothing, so the classifier's own session
// can't recurse into the kit.
const Guard = "KIT_CLASSIFIER"

// promptVersion is part of every cache key: a changed prompt asks again.
const promptVersion = "10"

// batchSize caps the entries one classifier call answers.
const batchSize = 8

// maxCalls caps the classifier calls running at once.
const maxCalls = 10

// unseenDays is how long an answer for an entry no run has seen is kept.
const unseenDays = 30

// Verdict is one entry's answer.
type Verdict struct {
	Role     string    `json:"role"`
	Kind     string    `json:"kind,omitempty"`
	Mutates  bool      `json:"mutates"`
	Segments []Segment `json:"segments,omitempty"`
	FileForm string    `json:"file_form,omitempty"`
	Globs    []string  `json:"globs,omitempty"`
	Affected string    `json:"affected_form,omitempty"`
}

// String is v in one line: role:kind, then what else it says.
func (v Verdict) String() string {
	parts := []string{RoleKind(v.Role, v.Kind)}
	if v.Mutates {
		parts = append(parts, "mutates")
	}
	for _, s := range v.Segments {
		parts = append(parts, fmt.Sprintf("segment %q %s", s.Text, RoleKind(s.Role, s.Kind)))
	}
	if v.FileForm != "" {
		parts = append(parts, "files: "+v.FileForm)
	}
	if v.Affected != "" {
		parts = append(parts, "affected: "+v.Affected)
	}
	return strings.Join(parts, "; ")
}

// RoleKind is a role with its check kind, as "check:lint".
func RoleKind(role, kind string) string {
	if kind == "" {
		return role
	}
	return role + ":" + kind
}

// Segment is one command of a body that runs several.
type Segment struct {
	Text string `json:"text"`
	Role string `json:"role"`
	Kind string `json:"kind,omitempty"`
}

// Manager is a lockfile directory's package-manager facts.
type Manager struct {
	Dir       string   `json:"dir"`
	Cites     string   `json:"cites"`
	Manager   string   `json:"manager"`
	RunPrefix string   `json:"run_prefix"`
	AddVerbs  []string `json:"add_verbs"`
	DLX       string   `json:"dlx"`
	DirFlags  []string `json:"dir_flags"`
	Rivals    []string `json:"rivals"`
}

// Proposal is a check or formatter the project states by evidence alone:
// a tool's config, a declared dependency, or a language manifest.
type Proposal struct {
	Role     string   `json:"role"`
	Kind     string   `json:"kind,omitempty"`
	Command  string   `json:"command"`
	FileForm string   `json:"file_form,omitempty"`
	Globs    []string `json:"globs,omitempty"`
	Dir      string   `json:"dir"`
	Evidence string   `json:"evidence"`
}

// Key names e's answer: its place, name, and body, and the prompt version,
// so a changed body or prompt asks again.
func Key(e sources.Entry) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{promptVersion, e.Source, e.File, e.Dir, e.Name, e.Body.Text}, "\x00")))
	return hex.EncodeToString(sum[:8])
}

// Options says what to answer and whether the classifier may be asked.
// Entries are all of Root's (sources.Entries): the project facts are keyed
// on their config files, so a subset would ask for them again.
type Options struct {
	Root, CacheDir string
	Entries        []sources.Entry
	// Ask calls the classifier for what the cache lacks, waiting up to
	// Timeout for it and for another run's lock; else the cache answers.
	Ask     bool
	Timeout time.Duration
}

// Result is each entry's verified verdict, or why it has none, and the
// verified project facts.
type Result struct {
	Verdicts  map[string]Verdict // by Key
	Skipped   map[string]string  // by Key: unclassified, rejected, or denied, with why
	Managers  []Manager
	Proposals []Proposal
	Dropped   []string // answers verification threw away, each with why
	Failure   string   // why a classifier call failed, when one did
}

// Unclassified counts the entries with no verdict that the kit's policy
// didn't deny.
func (r Result) Unclassified() int {
	n := 0
	for _, why := range r.Skipped {
		if !Denied(why) {
			n++
		}
	}
	return n
}

// Denied reports whether a Skipped reason is the kit's policy denying the
// entry, not a missing or rejected verdict.
func Denied(why string) bool {
	return strings.HasPrefix(why, deniedReason)
}

const deniedReason = "denied"

// store is the cache file: the model's answers as given, verified on read.
type store struct {
	Entries   map[string]*answer `json:"entries"`
	Context   string             `json:"context"` // the project the facts below answer
	Managers  []Manager          `json:"managers"`
	Proposals []Proposal         `json:"proposals"`
}

type answer struct {
	Verdict Verdict   `json:"verdict"`
	Seen    time.Time `json:"seen"`
}

// Run answers o.Entries from the cache, first asking the classifier for
// what it lacks when o.Ask allows.
func Run(cfg *config.Config, o Options) Result {
	file := CacheFile(o.CacheDir, o.Root)
	s := load(file)
	// A read is use: cache.Prune drops only a file no run has read.
	now := time.Now()
	_ = os.Chtimes(file, now, now)
	failure := ""
	if o.Ask && os.Getenv(Guard) != "1" {
		// The view reads the whole repo: only a run that may ask builds it.
		if p := readView(cfg, o.Root, o.Entries); s.stale(cfg, o.Entries, p.Key) {
			s, failure = ask(cfg, o, p, file, s)
		}
	}
	return verify(cfg, o.Root, o.Entries, s, failure)
}

// CacheFile holds root's answers; a run that asks takes the .lock beside it.
func CacheFile(cacheDir, root string) string {
	return filepath.Join(cacheDir, cache.Enforce, cache.RootKey(root)+".json")
}

// Managers are root's cached package-manager facts that verify, read
// without asking: a cold cache has none.
func Managers(cfg *config.Config, root, cacheDir string) []Manager {
	ms, _ := managers(cfg, root, load(CacheFile(cacheDir, root)).Managers)
	return ms
}

// Nearest are the managers whose lockfile directory holds dir, the deepest
// first; ties keep ms's order.
func Nearest(root, dir string, ms []Manager) []Manager {
	var out []Manager
	for _, m := range ms {
		if fsx.Within(filepath.Join(root, m.Dir), dir) {
			out = append(out, m)
		}
	}
	// Every one holds dir, so the longer path is the deeper.
	slices.SortStableFunc(out, func(a, b Manager) int {
		return len(filepath.Join(root, b.Dir)) - len(filepath.Join(root, a.Dir))
	})
	return out
}

// ask calls the classifier under the repo's lock, a batch per call in
// parallel, stores each answered batch's own entries, and returns the
// store; failure says why a batch couldn't.
func ask(cfg *config.Config, o Options, p view, file string, s store) (_ store, failure string) {
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	unlock, err := cache.Lock(ctx, strings.TrimSuffix(file, ".json")+".lock")
	if err != nil {
		return s, "waiting for another run: " + err.Error()
	}
	defer unlock()
	// Another run may have answered while this one waited.
	locked := load(file)
	if !locked.stale(cfg, o.Entries, p.Key) {
		return locked, ""
	}
	// The facts get a call of their own: they take the model longest.
	var reqs []request
	if locked.Context != p.Key {
		reqs = append(reqs, request{View: p, Facts: true})
	}
	for batch := range slices.Chunk(locked.missing(cfg, o.Entries), batchSize) {
		reqs = append(reqs, request{Entries: batch, View: p})
	}
	resps, errs := classifyAll(ctx, cfg, o, reqs)
	now := time.Now()
	for i, resp := range resps {
		if errs[i] != nil {
			if failure == "" {
				failure = "classifier: " + errs[i].Error()
			}
			continue
		}
		for _, a := range resp.Entries {
			locked.Entries[a.ID] = &answer{Verdict: a.Verdict, Seen: now}
		}
		if reqs[i].Facts {
			locked.Context, locked.Managers, locked.Proposals = p.Key, resp.Managers, resp.Proposals
		}
	}
	if slices.Contains(errs, nil) {
		locked.save(file, o.Entries, now)
	}
	return locked, failure
}

// classifyAll runs reqs, at most maxCalls at once.
func classifyAll(ctx context.Context, cfg *config.Config, o Options, reqs []request) ([]response, []error) {
	resps, errs := make([]response, len(reqs)), make([]error, len(reqs))
	var wg sync.WaitGroup
	slots := make(chan struct{}, maxCalls)
	for i, req := range reqs {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			resps[i], errs[i] = classify(ctx, cfg, o, req)
		})
	}
	wg.Wait()
	return resps, errs
}

func load(file string) store {
	s := sources.Decode[store](file, json.Unmarshal)
	if s.Entries == nil {
		s.Entries = map[string]*answer{}
	}
	return s
}

// stale is true when an entry the policy allows has no answer, or the
// project facts answer another project.
func (s store) stale(cfg *config.Config, entries []sources.Entry, context string) bool {
	return s.Context != context || len(s.missing(cfg, entries)) > 0
}

// missing is the entries the policy allows that have no answer.
func (s store) missing(cfg *config.Config, entries []sources.Entry) []sources.Entry {
	var out []sources.Entry
	for _, e := range entries {
		if s.Entries[Key(e)] == nil && denied(cfg, e) == "" {
			out = append(out, e)
		}
	}
	return out
}

// save marks entries seen, drops answers unseen for unseenDays, and writes
// the file whole.
func (s store) save(file string, entries []sources.Entry, now time.Time) {
	for _, e := range entries {
		if a := s.Entries[Key(e)]; a != nil {
			a.Seen = now
		}
	}
	for key, a := range s.Entries {
		if cache.Older(a.Seen, unseenDays, now) {
			delete(s.Entries, key)
		}
	}
	data, err := json.Marshal(s)
	if err == nil {
		_ = fsx.WriteAtomic(file, data, 0o644)
	}
}
