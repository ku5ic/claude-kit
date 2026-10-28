// Package enforce builds the full gate: what the kit runs to verify a whole
// project at a plan's end or on request. The project's CI check steps come
// first, read through the tasks they run; then, for each check kind a
// subproject still lacks, its task graph's checks (turbo, nx), its task
// runners' and pre-commit's, and last the checks it states only by
// evidence. Every command's binary goes through resolve.
package enforce

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/git"
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
	// Affected is the body limited to what changed since {base}, run in
	// AffectedDir when set, else Dir, when its source states one; a scoped
	// plan runs it.
	Affected    string
	AffectedDir string
	Unchanged   string // the git base a scoped gate's files didn't change since: it passes unrun
	// Entry is the gap-fill key of the entry it runs, and CoveredBy that of
	// a check whose run already does this one's, once the plan skips it for
	// that.
	Entry, CoveredBy string
	markKey          string // its key before add narrowed it
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
// Managed is true when gap-fill has verified a package manager. Since is
// what changed since the git base, for a scoped plan that found one.
type Plan struct {
	Root         string
	Gates        []Gate
	Unclassified int
	Discovery    string
	Managed      bool
	Since        *git.Since
}

// Options says which root to plan, which subprojects to keep, whether
// gap-fill may ask the classifier for what its cache lacks, and whether to
// scope the gates to what changed since the git base.
type Options struct {
	Root, CacheDir string
	Only           []string
	Ask            bool
	Timeout        time.Duration
	Changed        bool
}

// Discovery is the instruction for a project that states no check.
const Discovery = "nothing in this project states a check: no CI step, task, hook config, or tool evidence. Find how it is verified (README, CONTRIBUTING) and ask before inventing a check"

// Build plans the full gate for o.Root.
func Build(cfg *config.Config, o Options) Plan {
	b, subs := fullGate(cfg, o)
	b.unclassified(func(e sources.Entry) bool { return !sources.SourceOf(e.Source).GitHook })
	b.settle(o.CacheDir)
	b.skipCovered()
	p := Plan{Root: o.Root, Unclassified: b.skipped, Managed: len(b.facts.Managers) > 0, Since: b.since}
	// A fan-out gate belongs to each member it runs across.
	member := func(sub string) bool { return b.members[filepath.Join(o.Root, sub)] }
	for _, g := range b.gates {
		if len(o.Only) == 0 || slices.Contains(o.Only, b.subOf(g.Dir, subs)) || g.FanOut && slices.ContainsFunc(o.Only, member) {
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
	b, subs := plan(cfg, o)
	b.ci()
	for _, sub := range subs {
		b.fill(sub)
	}
	return b, subs
}

// skipCovered skips each gate another gate covers, naming it, when that
// gate runs its whole or affected body: one skipped, passed unrun, or
// narrowed to a file form may not do the covered gate's work.
func (b *builder) skipCovered() {
	label := map[string]string{}
	for _, g := range b.gates {
		if g.Entry != "" && g.Skip == "" && g.Unchanged == "" && len(g.Files) == 0 && g.CoveredBy == "" {
			label[g.Entry] = g.Label
		}
	}
	for i := range b.gates {
		if g := &b.gates[i]; g.Skip == "" && label[g.CoveredBy] != "" {
			g.Skip = "covered by " + label[g.CoveredBy]
		} else {
			g.CoveredBy = ""
		}
	}
}

// Covered is true for a gate skipped because another gate's run does its
// work; it runs after all when that gate doesn't.
func (g Gate) Covered() bool { return g.CoveredBy != "" }

// plan is an empty builder for o.Root: its entries and their verified
// verdicts, and its subprojects.
func plan(cfg *config.Config, o Options) (*builder, []string) {
	subs := project.Subprojects(o.Root)
	entries := sources.Entries(cfg, o.Root, subs)
	facts := gapfill.Run(cfg, gapfill.Options{Root: o.Root, CacheDir: o.CacheDir, Entries: entries, Ask: o.Ask, Timeout: o.Timeout})
	b := newBuilder(cfg, o.Root, entries, facts)
	if o.Changed {
		b.scopeTo(o.Root)
	}
	return b, subs
}

// hookPlan is plan narrowed to the git-hook managers' entries, never
// asked: all an edit or a commit message needs, without reading CI, task
// graphs, or subprojects.
func hookPlan(cfg *config.Config, o Options) *builder {
	entries := sources.HookEntries(o.Root)
	facts := gapfill.Run(cfg, gapfill.Options{Root: o.Root, CacheDir: o.CacheDir, Entries: entries})
	return newBuilder(cfg, o.Root, entries, facts)
}

// Task is one task a task runner holds in a subproject, and the command the
// full gate runs it by.
type Task struct {
	Provider, Stack, Name, Cmd string
}

// Tasks are sub's tasks, each run through the nearest package manager
// gap-fill verified, as the full gate runs it; npm for package.json's
// scripts when none is.
func Tasks(cfg *config.Config, root, cacheDir, sub string) []Task {
	entries := sources.Entries(cfg, root, []string{sub})
	b := newBuilder(cfg, root, entries, gapfill.Result{Managers: gapfill.Managers(cfg, root, cacheDir)})
	var out []Task
	for _, e := range entries {
		if tp, ok := provider(e.Source); ok && e.Dir == sub {
			out = append(out, Task{tp.Name, tp.Stack, e.Name, b.runCommand(tp, sub, e.Name)})
		}
	}
	return out
}

type taskKey struct{ provider, dir, name string }

type builder struct {
	cfg      *config.Config
	root     string
	entries  []sources.Entry
	facts    gapfill.Result
	tasks    map[taskKey]sources.Entry
	resolver *resolve.Resolver
	gates    []Gate
	planned  map[string]bool // gates by dir and command
	members  map[string]bool // the workspace's member dirs, which a fan-out gate covers
	skipped  int
	since    *git.Since // what changed since the git base, in a scoped plan
	changed  []string   // since's files, sorted
}

func newBuilder(cfg *config.Config, root string, entries []sources.Entry, facts gapfill.Result) *builder {
	b := &builder{cfg: cfg, root: root, entries: entries, facts: facts, tasks: map[taskKey]sources.Entry{}, planned: map[string]bool{}, members: map[string]bool{}}
	for _, member := range project.Workspace(root) {
		b.members[filepath.Join(root, member)] = true
	}
	for _, e := range entries {
		b.tasks[b.key(e)] = e
	}
	var managers []string
	for _, m := range facts.Managers {
		managers = append(managers, m.Manager)
		if runner, _, _ := strings.Cut(m.RunPrefix, " "); runner != "" {
			managers = append(managers, runner)
		}
	}
	b.resolver = resolve.New(root, managers, cfg.UserPins)
	return b
}

// key is task e's place in b.tasks.
func (b *builder) key(e sources.Entry) taskKey {
	return taskKey{e.Source, filepath.Join(b.root, e.Dir), e.Name}
}

// verdict is e's verified verdict, or why it has none.
func (b *builder) verdict(e sources.Entry) (gapfill.Verdict, string) {
	key := gapfill.Key(e)
	if v, ok := b.facts.Verdicts[key]; ok {
		return v, ""
	}
	return gapfill.Verdict{}, b.facts.Skipped[key]
}

// add appends g, narrowed to what changed when the plan is scoped, unless
// the same command already runs in the same directory.
func (b *builder) add(g Gate) {
	// Its marks stay those of the gate as planned whole.
	g.markKey = g.key(b.root)
	b.narrow(&g)
	key := g.Dir + "\x00" + g.Command()
	if b.planned[key] {
		return
	}
	b.planned[key] = true
	b.gates = append(b.gates, g)
}

// covered is true when a gate already runs kind in dir, or across the
// workspace dir is a member of.
func (b *builder) covered(dir, kind string) bool {
	return slices.ContainsFunc(b.gates, func(g Gate) bool {
		return slices.Contains(g.Kinds, kind) && (g.Dir == dir || g.FanOut && b.members[dir])
	})
}

// ci plans every CI step that checks: a step another rule denied runs
// nothing, and one gap-fill couldn't classify is reported by unclassified.
func (b *builder) ci() {
	for _, e := range b.entries {
		if !sources.SourceOf(e.Source).CI {
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
		b.gates = append(b.gates, Gate{Label: strings.Join(missing, ", ") + project.SubLabel(sub), Dir: dir, Skip: "nothing enforces it"})
	}
}

// graph plans turbo's and nx's checks of kind, at the root, across every
// workspace package.
func (b *builder) graph(sub, kind string) {
	if sub != "." {
		return
	}
	for _, e := range b.entries {
		if v, why := b.verdict(e); sources.SourceOf(e.Source).Graph && why == "" && slices.Contains(kindsOf(v), kind) {
			b.add(b.entryGate(e, v, b.root, e.Env))
		}
	}
}

// taskRunners plans sub's tasks of kind, each run by its runner: those of
// that kind alone first, each once (one another of them calls runs inside
// it), an aggregate only when none is.
func (b *builder) taskRunners(sub, kind string) {
	dir := filepath.Join(b.root, sub)
	var tasks, aggregates []sources.Entry
	called := map[taskKey]bool{}
	for _, e := range b.entries {
		_, isTask := provider(e.Source)
		v, why := b.verdict(e)
		if !isTask || e.Dir != sub || why != "" {
			continue
		}
		kinds := b.taskKinds(e, v, 0)
		switch {
		case len(kinds) == 1 && kinds[0] == kind:
			tasks = append(tasks, e)
			parts, _ := b.split(e.Body.Text, dir, nil, v, 0)
			for _, p := range parts {
				if p.task != nil {
					called[b.key(*p.task)] = true
				}
			}
		case slices.Contains(kinds, kind):
			aggregates = append(aggregates, e)
		}
	}
	for _, e := range tasks {
		if !called[b.key(e)] {
			tp, _ := provider(e.Source)
			v, _ := b.verdict(e)
			g := verdictGate(e, v, dir)
			g.Label, g.Kinds, g.Body = label([]string{kind}, e.File+": "+e.Name, sub), []string{kind}, b.runCommand(tp, sub, e.Name)
			b.add(g)
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
	tp, _ := row("pre-commit")
	for _, e := range b.entries {
		if v, why := b.verdict(e); e.Source == tp.Name && why == "" && slices.Contains(kindsOf(v), kind) {
			b.add(Gate{Label: label(kindsOf(v), e.File+": "+e.Name, sub), Kinds: kindsOf(v), Dir: b.root, Body: b.runCommand(tp, sub, e.Name), Verdict: v.String(), FanOut: true})
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
		if _, why := b.verdict(e); why != "" && !gapfill.Denied(why) {
			b.gates = append(b.gates, Gate{Label: e.File + ": " + e.Name + project.SubLabel(e.Dir), Dir: filepath.Join(b.root, e.Dir), Skip: why})
			b.skipped++
		}
	}
}

// settle skips each gate disabled_checks names, by a kind it checks or by
// its label, gives each gate_env, then finds the rest's binaries, then skips
// each gate seen changing files.
func (b *builder) settle(cacheDir string) {
	for i := range b.gates {
		g := &b.gates[i]
		if g.Skip == "" && (b.cfg.CheckDisabled("", g.Label) || slices.ContainsFunc(g.Kinds, func(k string) bool { return b.cfg.CheckDisabled(k, g.Label) })) {
			g.Skip = "disabled_checks"
		}
		g.Env = append(slices.Clone(b.cfg.GateEnv), g.Env...)
	}
	b.resolve()
	skipMarked(b.gates, cacheDir, b.root)
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
		if fsx.Within(abs, dir) && len(sub) > len(best) {
			best = sub
		}
	}
	return best
}

// provider is the task runner a source names, when its entries are tasks.
func provider(source string) (sources.TaskProvider, bool) {
	tp, ok := row(source)
	return tp, ok && tp.Tasks != nil
}

// row is the TaskProviders row named name.
func row(name string) (sources.TaskProvider, bool) {
	i := slices.IndexFunc(sources.TaskProviders, func(tp sources.TaskProvider) bool { return tp.Name == name })
	if i < 0 {
		return sources.TaskProvider{}, false
	}
	return sources.TaskProviders[i], true
}

// runCommand is how tp runs task in sub: through the package manager the
// nearest verified lockfile names, when tp runs by it, else tp.PM.
func (b *builder) runCommand(tp sources.TaskProvider, sub, task string) string {
	pm := ""
	for _, m := range gapfill.Nearest(b.root, filepath.Join(b.root, sub), b.facts.Managers) {
		_, override := tp.RunByPM[m.Manager]
		// {pm} is tp.PM, or a manager that rivals it.
		if override || tp.PM != "" && (m.Manager == tp.PM || slices.Contains(m.Rivals, tp.PM)) {
			pm = m.Manager
			break
		}
	}
	run := tp.Run
	if override, ok := tp.RunByPM[pm]; ok {
		run = override
	}
	pm = cmp.Or(pm, tp.PM)
	return strings.NewReplacer("{pm}", pm, "{task}", task).Replace(run)
}

// label is a gate's line: the kinds it checks ("check" for an aggregate the
// verdict gives none), where it comes from, and its subproject.
func label(kinds []string, origin, sub string) string {
	what := strings.Join(kinds, "+")
	if what == "" {
		what = "check"
	}
	return what + " (" + origin + ")" + project.SubLabel(sub)
}

// rel is dir relative to the root, for a label.
func (b *builder) rel(dir string) string { return fsx.Rel(b.root, dir) }
