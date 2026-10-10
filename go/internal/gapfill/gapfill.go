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
	"strings"
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
const promptVersion = "7"

// unseenDays is how long an answer for an entry no run has seen is kept.
const unseenDays = 30

// Verdict is one entry's answer.
type Verdict struct {
	Role       string    `json:"role"`
	Kind       string    `json:"kind,omitempty"`
	Mutates    bool      `json:"mutates"`
	Segments   []Segment `json:"segments,omitempty"`
	FileForm   string    `json:"file_form,omitempty"`
	Globs      []string  `json:"globs,omitempty"`
	Affected   string    `json:"affected_form,omitempty"`
	Exclusions string    `json:"exclusions,omitempty"`
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
	Mutates  bool     `json:"mutates"`
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
}

// Unclassified counts the entries with no verdict that the kit's policy
// didn't deny.
func (r Result) Unclassified() int {
	n := 0
	for _, why := range r.Skipped {
		if !strings.HasPrefix(why, "denied") {
			n++
		}
	}
	return n
}

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
	path := filepath.Join(o.CacheDir, cache.Enforce, cache.RootKey(o.Root))
	p := readView(cfg, o.Root, o.Entries)
	s := load(path + ".json")
	// A read is use: cache.Prune drops only a file no run has read.
	now := time.Now()
	_ = os.Chtimes(path+".json", now, now)
	failure := ""
	if o.Ask && os.Getenv(Guard) != "1" && s.stale(cfg, o.Entries, p.Key) {
		s, failure = ask(cfg, o, p, path, s)
	}
	return verify(cfg, o.Root, o.Entries, s, failure)
}

// Managers are root's cached package-manager facts that verify, read
// without asking: a cold cache has none.
func Managers(cfg *config.Config, root, cacheDir string) []Manager {
	ms, _ := managers(cfg, root, load(filepath.Join(cacheDir, cache.Enforce, cache.RootKey(root))+".json").Managers)
	return ms
}

// ask calls the classifier under the repo's lock, stores its answers, and
// returns the store they're in; failure says why it couldn't, and s comes
// back as it was.
func ask(cfg *config.Config, o Options, p view, path string, s store) (_ store, failure string) {
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	unlock, err := cache.Lock(ctx, path+".lock")
	if err != nil {
		return s, "waiting for another run: " + err.Error()
	}
	defer unlock()
	// Another run may have answered while this one waited.
	locked := load(path + ".json")
	if !locked.stale(cfg, o.Entries, p.Key) {
		return locked, ""
	}
	resp, err := classify(ctx, cfg, o, request{Entries: locked.missing(cfg, o.Entries), View: p, Facts: locked.Context != p.Key})
	if err != nil {
		return locked, "classifier: " + err.Error()
	}
	now := time.Now()
	for _, a := range resp.Entries {
		locked.Entries[a.ID] = &answer{Verdict: a.Verdict, Seen: now}
	}
	if locked.Context != p.Key {
		locked.Context, locked.Managers, locked.Proposals = p.Key, resp.Managers, resp.Proposals
	}
	locked.save(path+".json", o.Entries, now)
	return locked, ""
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
		if now.Sub(a.Seen) > unseenDays*24*time.Hour {
			delete(s.Entries, key)
		}
	}
	data, err := json.Marshal(s)
	if err == nil {
		_ = fsx.WriteAtomic(file, data, 0o644)
	}
}
