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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claudecli"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Guard is set in the classifier's environment, with a marker under the
// cache (claudecli.Guarded), so its hooks do nothing. A kit started inside
// any claude run the kit started never asks, so the classifier's own
// session can't recurse into the kit.
const Guard = "KIT_CLASSIFIER"

// promptVersion is part of every cache key: a changed prompt asks again.
const promptVersion = "15"

// ClassifyTimeout caps a foreground run that asks the classifier, waiting
// included.
const ClassifyTimeout = 60 * time.Second

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
	// CoveredBy is the key of a check of the same kind, in the same
	// directory, whose run already does this one's.
	CoveredBy string `json:"covered_by,omitempty"`
	// AlsoKinds are the kinds a check's run does besides Kind: a run that
	// compiles typed code type-checks it too.
	AlsoKinds []string `json:"also_kinds,omitempty"`
	// Reaches are the other directories, relative to the root, whose code
	// a check's run checks, such as a subproject it tests a copy of.
	Reaches []string `json:"reaches,omitempty"`
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
	if len(v.AlsoKinds) > 0 {
		parts = append(parts, "also: "+strings.Join(v.AlsoKinds, ", "))
	}
	if len(v.Reaches) > 0 {
		parts = append(parts, "reaches: "+strings.Join(v.Reaches, ", "))
	}
	return strings.Join(parts, "; ")
}

// Squeeze is s with its whitespace removed, so a segment the model spaced
// unlike its body (<<< "$x" for <<<"$x") still matches it.
func Squeeze(s string) string { return strings.Join(strings.Fields(s), "") }

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
	Dropped   []string        // answers verification threw away, each with why
	Failure   string          // why a classifier call failed, when one did
	Usage     claudecli.Usage // what this run's classifier calls cost
}

// Unclassified counts the entries the cache has no answer for and asking
// could answer now. A denied or rejected one has an answer, and asking
// again returns the same one; one cooling after a failure waits.
func (r Result) Unclassified() int {
	n := 0
	for _, why := range r.Skipped {
		if strings.HasPrefix(why, unclassifiedReason) && !strings.HasPrefix(why, coolingReason) {
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

const (
	deniedReason       = "denied"
	unclassifiedReason = "unclassified"
	// coolingReason is an unclassified entry the classifier failed on
	// within claudecli.CooldownPeriod: asking again now would likely fail
	// again.
	coolingReason = unclassifiedReason + " (the classifier failed on it: "
)

// store is the cache file: the model's answers as given, verified on read.
type store struct {
	Entries   map[string]*answer `json:"entries"`
	Context   string             `json:"context"` // the project the facts below answer
	Managers  []Manager          `json:"managers"`
	Proposals []Proposal         `json:"proposals"`
	// cool is the classifier's failures for this root, by failKey.
	cool claudecli.Cooldown
}

type answer struct {
	Verdict Verdict   `json:"verdict"`
	Seen    time.Time `json:"seen"`
}

// Run answers o.Entries from the cache, first asking the classifier for
// what it lacks when o.Ask allows.
func Run(cfg *config.Config, o Options) Result {
	file := seededCacheFile(o.CacheDir, o.Root)
	s := load(file)
	s.cool = classifierCooldown(o.CacheDir, o.Root)
	// A read is use, and cache.Prune drops only a file no run has read. Once a
	// day is enough for that, and each touch makes the stack report stale.
	now := time.Now()
	if info, err := os.Stat(file); err == nil && cache.Older(info.ModTime(), 1, now) {
		_ = os.Chtimes(file, now, now)
	}
	failure, usage := "", claudecli.Usage{}
	if o.Ask && os.Getenv(claudecli.DepthVar) == "" {
		// The view reads the whole repo: only a run that may ask builds it.
		if p := readView(cfg, o.Root, o.Entries); s.stale(cfg, o.Entries, p.Key) {
			s, failure, usage = ask(cfg, o, p, file, s)
		}
	}
	r := verify(cfg, o.Root, o.Entries, s, failure)
	r.Usage = usage
	return r
}

// CacheFile holds root's answers; a run that asks takes the .lock beside it.
func CacheFile(cacheDir, root string) string {
	return filepath.Join(cacheDir, cache.Enforce, cache.RootKey(root)+".json")
}

// seededCacheFile is CacheFile, copied first from the main checkout's when
// root is a linked worktree with none yet. Answers are keyed by each
// entry's content and verified on every read, so the main checkout's hold
// wherever its entries do.
func seededCacheFile(cacheDir, root string) string {
	file := CacheFile(cacheDir, root)
	// A linked worktree's .git is a file. The stat keeps the git exec off
	// the bash guard's path everywhere else.
	if fsx.IsFile(file) || !fsx.IsFile(filepath.Join(root, ".git")) {
		return file
	}
	// git names the main checkout once; the marker beside file keeps its
	// cache file's path, "" for a root with none, so a worktree whose main
	// checkout has no cache yet stops running git on every guarded command.
	marker := file + ".main"
	mainFile, err := os.ReadFile(marker)
	if err != nil {
		found, ok := mainCacheFile(cacheDir, root)
		if !ok {
			return file
		}
		mainFile = []byte(found)
		_ = fsx.WriteAtomic(marker, mainFile, 0o600)
	}
	if len(mainFile) == 0 {
		return file
	}
	if data, err := os.ReadFile(string(mainFile)); err == nil {
		_ = fsx.WriteAtomic(file, data, 0o600) // unwritten, the worktree starts cold, as before
	}
	return file
}

// mainCacheFile is the cache file of the main checkout root is a linked
// worktree of, "" when it's none (a submodule's .git file); ok is false
// when git couldn't say, so the answer isn't kept.
func mainCacheFile(cacheDir, root string) (string, bool) {
	common, err := git.Line(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", false
	}
	if main := filepath.Dir(common); filepath.Base(common) == ".git" && main != root {
		return CacheFile(cacheDir, main), true
	}
	return "", true
}

// Managers are root's cached package-manager facts that verify, read
// without asking: a cold cache has none.
func Managers(cfg *config.Config, root, cacheDir string) []Manager {
	ms, _ := managers(cfg, root, load(seededCacheFile(cacheDir, root)).Managers)
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
// store; failure says why a batch couldn't, and usage what the calls cost.
func ask(cfg *config.Config, o Options, p view, file string, s store) (_ store, failure string, usage claudecli.Usage) {
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	unlock, err := cache.Lock(ctx, strings.TrimSuffix(file, ".json")+".lock")
	if err != nil {
		return s, "waiting for another run: " + err.Error(), usage
	}
	defer unlock()
	// Another run may have answered while this one waited.
	locked := load(file)
	locked.cool = s.cool
	if !locked.stale(cfg, o.Entries, p.Key) {
		return locked, "", usage
	}
	// The facts get a call of their own: they take the model longest.
	var reqs []request
	if locked.Context != p.Key && !locked.held(cfg, factsKey(p.Key), time.Now()) {
		reqs = append(reqs, request{View: p, Facts: true})
	}
	allowed := slices.DeleteFunc(slices.Clone(o.Entries), func(e sources.Entry) bool { return denied(cfg, e) != "" })
	for batch := range slices.Chunk(locked.missing(cfg, o.Entries), batchSize) {
		asked, dirs := map[string]bool{}, map[string]bool{}
		for _, e := range batch {
			asked[Key(e)], dirs[e.Dir] = true, true
		}
		// coverage keeps a coverer only in its claimant's directory.
		others := slices.DeleteFunc(slices.Clone(allowed), func(e sources.Entry) bool { return asked[Key(e)] || !dirs[e.Dir] })
		reqs = append(reqs, request{Entries: batch, Others: others, View: p})
	}
	resps, usages, errs := classifyAll(ctx, cfg, o, reqs)
	for _, u := range usages {
		usage = usage.Add(u)
	}
	now := time.Now()
	notAsked := false
	failed, answered := map[string]string{}, []string{}
	for i, resp := range resps {
		if errors.Is(errs[i], errNotAsked) {
			notAsked = true
			continue // no cooldown: the next run asks it
		}
		if errs[i] != nil {
			if failure == "" {
				failure = "classifier: " + errs[i].Error()
			}
			for _, e := range reqs[i].Entries {
				failed[failKey(cfg, Key(e))] = errs[i].Error()
			}
			if reqs[i].Facts {
				failed[failKey(cfg, factsKey(p.Key))] = errs[i].Error()
			}
			continue
		}
		for _, a := range resp.Entries {
			locked.Entries[a.ID] = &answer{Verdict: a.Verdict, Seen: now}
			answered = append(answered, failKey(cfg, a.ID))
		}
		if reqs[i].Facts {
			locked.Context, locked.Managers, locked.Proposals = p.Key, resp.Managers, resp.Proposals
			answered = append(answered, failKey(cfg, factsKey(p.Key)))
		}
	}
	// Best effort: unrecorded, the next run asks again.
	_ = locked.cool.Record(now, failed, answered)
	if failure == "" && notAsked {
		failure = "classifier: " + errNotAsked.Error()
	}
	locked.save(file, o.Entries, now)
	return locked, failure, usage
}

// errNotAsked is a request still queued for a call slot when the deadline
// passed.
var errNotAsked = errors.New("not asked before the deadline")

// classifyAll runs reqs, at most maxCalls at once.
func classifyAll(ctx context.Context, cfg *config.Config, o Options, reqs []request) ([]response, []claudecli.Usage, []error) {
	resps, usages, errs := make([]response, len(reqs)), make([]claudecli.Usage, len(reqs)), make([]error, len(reqs))
	var wg sync.WaitGroup
	slots := make(chan struct{}, maxCalls)
	for i, req := range reqs {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			if ctx.Err() != nil {
				errs[i] = errNotAsked
				return
			}
			defer proc.Recover(func(v any) { errs[i] = fmt.Errorf("classifier panicked: %v", v) })
			resps[i], usages[i], errs[i] = classify(ctx, cfg, o, req)
		})
	}
	wg.Wait()
	return resps, usages, errs
}

func load(file string) store {
	s := sources.Decode[store](file, json.Unmarshal)
	if s.Entries == nil {
		s.Entries = map[string]*answer{}
	}
	return s
}

// held is true while a classifier failure on key holds it back.
func (s store) held(cfg *config.Config, key string, now time.Time) bool {
	_, ok := s.cool.Cooling(failKey(cfg, key), now)
	return ok
}

// classifierCooldown is the classifier's failures for root.
func classifierCooldown(cacheDir, root string) claudecli.Cooldown {
	return claudecli.NewCooldown(cacheDir, "gapfill-"+cache.RootKey(root))
}

// failKey is key's cooldown key under cfg's classifier command, so a
// changed command asks again at once. Key hashes an entry's body, so an
// edited entry is asked at once too.
func failKey(cfg *config.Config, key string) string {
	sum := sha256.Sum256([]byte(strings.Join(cfg.Classifier, "\x00")))
	return hex.EncodeToString(sum[:])[:12] + "-" + key
}

// stale is true when an entry the policy allows has no answer, or the
// project facts answer another project, and no classifier failure holds
// it back.
func (s store) stale(cfg *config.Config, entries []sources.Entry, context string) bool {
	facts := s.Context != context && !s.held(cfg, factsKey(context), time.Now())
	return facts || len(s.missing(cfg, entries)) > 0
}

// factsKey is the cooldown key of the project facts for context.
func factsKey(context string) string { return "facts:" + context }

// missing is the entries the policy allows that have no answer, and that
// no classifier failure holds back.
func (s store) missing(cfg *config.Config, entries []sources.Entry) []sources.Entry {
	var out []sources.Entry
	now := time.Now()
	for _, e := range entries {
		if s.Entries[Key(e)] == nil && denied(cfg, e) == "" && !s.held(cfg, Key(e), now) {
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
