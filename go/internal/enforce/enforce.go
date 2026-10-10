// Package enforce builds the full gate: what the kit runs to verify a whole
// project at a plan's end or on request. The project's CI check steps come
// first, read through the tasks they run; then, for each check kind a
// subproject still lacks, its task graph's checks (turbo, nx), its task
// runners' and pre-commit's, and last the checks it states only by
// evidence. Every command's binary goes through resolve.
package enforce

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/resolve"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// required are the check kinds a subproject with a manifest gets a line for
// when nothing states them; fillOrder is every kind the full gate fills, in
// the order it runs them, dead code only where something states it.
var (
	required  = []string{"lint", "typecheck", "format-check", "test"}
	fillOrder = append(slices.Clone(required), "deadcode")
)

// Gate is one command the full gate runs, or why it doesn't.
type Gate struct {
	Label   string
	Kinds   []string // the check kinds it runs: several for an aggregate run whole
	Dir     string   // where it runs, absolute
	Env     []string
	Body    string // the shell it runs; {base} is the git base at run time
	Bins    []resolve.Resolution
	Skip    string
	Verdict string // what gap-fill said of its source
	FanOut  bool   // it runs across every subproject
	// FileForm is the body run on given files ({files}, {dirs}) and Globs
	// the files it applies to, when its source states one; Stop runs it.
	FileForm string
	Globs    []string
	Files    []string // the files a Stop gate checks, absolute
}

// Scoped is true for a dead-code gate: only findings on lines changed since
// the git base fail it.
func (g Gate) Scoped() bool { return slices.Contains(g.Kinds, "deadcode") }

// LintLike is true for a gate whose findings, not its exit code, decide it
// at Stop: only those on changed lines block.
func (g Gate) LintLike() bool {
	return slices.ContainsFunc(g.Kinds, func(k string) bool {
		return slices.Contains([]string{"lint", "format-check", "deadcode", "security"}, k)
	})
}

// Plan is the full gate for one root. Discovery is set when the project
// states nothing to check; Unclassified counts the entries with no verdict.
type Plan struct {
	Root         string
	Gates        []Gate
	Unclassified int
	Discovery    string
}

// Options says which root to plan, which subprojects to keep, and whether
// gap-fill may ask the classifier for what its cache lacks.
type Options struct {
	Root, CacheDir string
	Only           []string
	Ask            bool
	Timeout        time.Duration
}

// Discovery is the instruction for a project that states no check.
const Discovery = "nothing in this project states a check: no CI step, task, hook config, or tool evidence. Find how it is verified (README, CONTRIBUTING) and ask before inventing a check"

// Build plans the full gate for o.Root.
func Build(cfg *config.Config, o Options) Plan {
	b, subs := fullGate(cfg, o)
	b.unclassified(func(e sources.Entry) bool { return !gitHook(e.Source) })
	b.resolve()
	skipMarked(b.gates, o.CacheDir, o.Root)
	p := Plan{Root: o.Root, Unclassified: b.skipped}
	for _, g := range b.gates {
		if len(o.Only) == 0 || slices.Contains(o.Only, b.subOf(g.Dir, subs)) {
			p.Gates = append(p.Gates, g)
		}
	}
	if len(b.gates) == 0 && b.skipped == 0 {
		p.Discovery = Discovery
	}
	return p
}

// fullGate is a builder holding o.Root's full gate, unresolved, and the
// subprojects it planned.
func fullGate(cfg *config.Config, o Options) (*builder, []string) {
	subs := project.Subprojects(cfg, o.Root)
	entries := sources.Entries(cfg, o.Root, subs)
	facts := gapfill.Run(cfg, gapfill.Options{Root: o.Root, CacheDir: o.CacheDir, Entries: entries, Ask: o.Ask, Timeout: o.Timeout})
	b := newBuilder(cfg, o.Root, entries, facts)
	b.ci()
	for _, sub := range subs {
		b.fill(sub)
	}
	return b, subs
}

// gitHook is true for a source that only a git hook runs: the full gate
// leaves it out, and Stop runs its pre-commit checks.
func gitHook(source string) bool {
	switch source {
	case "lint-staged", "lefthook", "husky", "commitlint":
		return true
	}
	return false
}

type taskKey struct{ provider, dir, name string }

type builder struct {
	cfg      *config.Config
	nocat    *config.Config // classify reads bodies without the old check catalog
	root     string
	entries  []sources.Entry
	facts    gapfill.Result
	tasks    map[taskKey]sources.Entry
	resolver *resolve.Resolver
	gates    []Gate
	planned  map[string]bool // gates by dir and body
	skipped  int
}

func newBuilder(cfg *config.Config, root string, entries []sources.Entry, facts gapfill.Result) *builder {
	nocat := *cfg
	nocat.Checks = nil
	b := &builder{cfg: cfg, nocat: &nocat, root: root, entries: entries, facts: facts, tasks: map[taskKey]sources.Entry{}, planned: map[string]bool{}}
	for _, e := range entries {
		b.tasks[taskKey{e.Source, filepath.Join(root, e.Dir), e.Name}] = e
	}
	var managers []string
	for _, m := range facts.Managers {
		managers = append(managers, m.Manager)
		if runner, _, _ := strings.Cut(m.RunPrefix, " "); runner != "" {
			managers = append(managers, runner)
		}
	}
	b.resolver = resolve.New(root, managers)
	return b
}

// verdict is e's verified verdict, or why it has none.
func (b *builder) verdict(e sources.Entry) (gapfill.Verdict, string) {
	key := gapfill.Key(e)
	if v, ok := b.facts.Verdicts[key]; ok {
		return v, ""
	}
	return gapfill.Verdict{}, b.facts.Skipped[key]
}

// add appends g unless the same body already runs in the same directory.
func (b *builder) add(g Gate) {
	key := g.Dir + "\x00" + g.Body
	if b.planned[key] {
		return
	}
	b.planned[key] = true
	b.gates = append(b.gates, g)
}

// covered is true when a gate already runs kind in dir, or across every
// subproject.
func (b *builder) covered(dir, kind string) bool {
	return slices.ContainsFunc(b.gates, func(g Gate) bool {
		return slices.Contains(g.Kinds, kind) && (g.Dir == dir || g.FanOut)
	})
}

// ci plans every CI step that checks: a step another rule denied runs
// nothing, and one gap-fill couldn't classify is reported by unclassified.
func (b *builder) ci() {
	for _, e := range b.entries {
		if e.Source != "github-actions" && e.Source != "gitlab-ci" {
			continue
		}
		if v, why := b.verdict(e); why == "" && checks(v) {
			b.expand(e, v, filepath.Join(b.root, e.Dir), e.Env)
		}
	}
}

// fill plans, for each kind no gate runs in sub yet, the task graph's
// checks, then sub's own tasks and pre-commit's hooks, then evidence; a
// kind none states gets one line.
func (b *builder) fill(sub string) {
	dir := filepath.Join(b.root, sub)
	var missing []string
	for _, kind := range fillOrder {
		for _, tier := range []func(string, string){b.graph, b.taskRunners, b.preCommit, b.evidence} {
			if !b.covered(dir, kind) {
				tier(sub, kind)
			}
		}
		if !b.covered(dir, kind) && slices.Contains(required, kind) {
			missing = append(missing, kind)
		}
	}
	if len(missing) > 0 && sources.HasManifest(dir) {
		b.gates = append(b.gates, Gate{Label: strings.Join(missing, ", ") + suffix(sub), Dir: dir, Skip: "nothing enforces it"})
	}
}

// graph plans turbo's and nx's checks of kind, at the root, across every
// workspace package.
func (b *builder) graph(sub, kind string) {
	if sub != "." {
		return
	}
	for _, e := range b.entries {
		if v, why := b.verdict(e); (e.Source == "turbo" || e.Source == "nx") && why == "" && slices.Contains(kindsOf(v), kind) {
			b.add(b.entryGate(e, v, b.root, e.Env))
		}
	}
}

// taskRunners plans sub's tasks of kind, each run by its runner: those of
// that kind alone first, an aggregate only when none is.
func (b *builder) taskRunners(sub, kind string) {
	dir := filepath.Join(b.root, sub)
	var aggregates []sources.Entry
	for _, e := range b.entries {
		tp, isTask := provider(e.Source)
		v, why := b.verdict(e)
		if !isTask || e.Dir != sub || why != "" {
			continue
		}
		kinds := b.taskKinds(e, v, 0)
		switch {
		case len(kinds) == 1 && kinds[0] == kind:
			b.add(Gate{Label: label(kinds, e.File+": "+e.Name, sub), Kinds: kinds, Dir: dir, Body: b.runCommand(tp, sub, e.Name), Verdict: v.String(), FileForm: v.FileForm, Globs: v.Globs})
		case slices.Contains(kinds, kind):
			aggregates = append(aggregates, e)
		}
	}
	if !b.covered(dir, kind) {
		for _, e := range aggregates {
			v, _ := b.verdict(e)
			b.expand(e, v, dir, nil)
		}
	}
}

// preCommit plans pre-commit's check hooks of kind, at the root, over the
// whole tree.
func (b *builder) preCommit(sub, kind string) {
	if sub != "." {
		return
	}
	for _, e := range b.entries {
		if v, why := b.verdict(e); e.Source == "pre-commit" && why == "" && slices.Contains(kindsOf(v), kind) {
			b.add(Gate{Label: label(kindsOf(v), e.File+": "+e.Name, sub), Kinds: kindsOf(v), Dir: b.root, Body: "pre-commit run " + e.Name + " --all-files", Verdict: v.String(), FanOut: true})
		}
	}
}

// evidence plans the checks of kind the project states only by evidence in
// sub: a tool's config, a declared dependency, its language's manifest.
func (b *builder) evidence(sub, kind string) {
	for _, p := range b.facts.Proposals {
		if p.Role == "check" && p.Kind == kind && filepath.Clean(p.Dir) == sub {
			dir := filepath.Join(b.root, sub)
			b.add(Gate{Label: label([]string{kind}, "evidence "+p.Evidence, sub), Kinds: []string{kind}, Dir: dir, Body: p.Command, Verdict: "proposed: " + p.Command, FileForm: p.FileForm, Globs: p.Globs})
		}
	}
}

// unclassified reports every entry the plan could run, as runs says, that
// has no verdict.
func (b *builder) unclassified(runs func(sources.Entry) bool) {
	for _, e := range b.entries {
		if !runs(e) {
			continue
		}
		if _, why := b.verdict(e); why != "" && !strings.HasPrefix(why, "denied") {
			b.gates = append(b.gates, Gate{Label: e.File + ": " + e.Name + suffix(e.Dir), Dir: filepath.Join(b.root, e.Dir), Skip: why})
			b.skipped++
		}
	}
}

// resolve finds each gate's binaries; the first that can't run skips it.
func (b *builder) resolve() {
	for i := range b.gates {
		g := &b.gates[i]
		if g.Skip != "" {
			continue
		}
		calls, ok := classify.Calls(g.Body)
		if !ok {
			g.Skip = "can't read the command"
			continue
		}
		for _, words := range calls {
			if words[0] == "$" {
				g.Skip = "a variable names the command it runs"
				break
			}
			res := b.resolver.Command(g.Dir, words)
			if res.Skip != "" {
				g.Skip, g.Bins = res.Skip, nil
				break
			}
			if !slices.ContainsFunc(g.Bins, func(r resolve.Resolution) bool { return r.Path == res.Path }) {
				g.Bins = append(g.Bins, res)
			}
		}
	}
}

// subOf is the subproject dir is in: the deepest one holding it.
func (b *builder) subOf(dir string, subs []string) string {
	best := "."
	for _, sub := range subs {
		abs := filepath.Join(b.root, sub)
		if (dir == abs || strings.HasPrefix(dir, abs+"/")) && len(sub) > len(best) {
			best = sub
		}
	}
	return best
}

// provider is the task runner a source names.
func provider(source string) (sources.TaskProvider, bool) {
	i := slices.IndexFunc(sources.TaskProviders, func(tp sources.TaskProvider) bool { return tp.Name == source })
	if i < 0 || source == "pre-commit" {
		return sources.TaskProvider{}, false
	}
	return sources.TaskProviders[i], true
}

// runCommand is how tp runs task in sub: through the package manager the
// nearest verified lockfile names, else package.json's own npm.
func (b *builder) runCommand(tp sources.TaskProvider, sub, task string) string {
	pm := b.manager(sub)
	run := tp.Run
	if override, ok := tp.RunByPM[pm]; ok {
		run = override
	}
	if pm == "" {
		pm = "npm"
	}
	return strings.NewReplacer("{pm}", pm, "{task}", task).Replace(run)
}

// manager is the verified manager of the nearest lockfile directory at or
// above sub, "" when none is.
func (b *builder) manager(sub string) string {
	best, name := -1, ""
	for _, m := range b.facts.Managers {
		d := filepath.Clean(m.Dir)
		if (d == "." || sub == d || strings.HasPrefix(sub, d+"/")) && len(d) > best {
			best, name = len(d), m.Manager
		}
	}
	return name
}

func suffix(sub string) string {
	if sub == "." || sub == "" {
		return ""
	}
	return " [" + sub + "]"
}

// label is a gate's line: the kinds it checks ("check" for an aggregate the
// verdict gives none), where it comes from, and its subproject.
func label(kinds []string, origin, sub string) string {
	what := strings.Join(kinds, "+")
	if what == "" {
		what = "check"
	}
	return what + " (" + origin + ")" + suffix(sub)
}

// rel is dir relative to the root, for a label.
func (b *builder) rel(dir string) string { return fsx.Rel(b.root, dir) }
