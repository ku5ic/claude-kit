package checks

import (
	"cmp"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/ci"
	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/tools"
)

// maxDepth bounds how deep references are followed (bashguard's bound).
const maxDepth = 8

// stateful commands change the shell a later command runs in, in ways the
// kit can't carry over to a leaf it runs on its own.
var stateful = []string{"source", ".", "set", "shopt", "alias", "unalias", "ulimit", "umask", "pushd", "popd"}

// strictOptions are the set options that only decide when a script stops
// or what it echoes (set -euo pipefail), never what one command does.
var strictOptions = []string{"-e", "+e", "-u", "+u", "-x", "+x", "-v", "+v", "-E", "-T", "-eu", "-ue", "-eo", "-euo", "-ueo", "-ex", "-eux", "-euxo", "-exo", "-o", "+o", "pipefail", "errexit", "nounset", "xtrace", "verbose", "errtrace", "functrace"}

// taskRoles is how each task of one directory takes part in run-checks.
type taskRoles struct {
	slots   []string            // per task: the check it fills, by name or by a single-gate body
	covered []int               // per task: the slot-named task whose body already runs it, or -1
	single  []*classify.Command // per task: its body's one gate, when it has exactly one
	tools   [][]string          // per task: the commands its body runs directly
	leaves  []leaf              // gates found inside aggregate tasks, in body order
	away    [][]awayGate        // per task: the gates its body runs in another subproject
	unsafe  []string            // per task: a forbidden command its body runs (eslint --fix), or ""
}

// leaf is one gate inside an aggregate: another task, run as itself, or an
// inline command, run as words. withArgs: a task run with the arguments the
// aggregate passes it, which beats a plain run of the same task. tools: the
// commands it runs, so a gate already running the same tool covers it.
type leaf struct {
	id, slot, label  string
	withArgs, inline bool
	tools            []string
	gate             Gate
	sub              string // CI: the subproject its gate runs in
}

type taskAt struct {
	task project.Task
	dir  string
}

// roles works out tasks' roles in dir: a task whose name matches a check
// is that check, and the slot-named tasks its body runs are covered by it;
// a task whose body is one gate is that gate's check; a task whose body
// runs other tasks or several gates is an aggregate, never run itself, its
// gates run separately; anything else doesn't run.
func roles(cfg *config.Config, root, dir, sfx string, tasks []project.Task, subs map[string]bool) taskRoles {
	r := taskRoles{slots: make([]string, len(tasks)), covered: make([]int, len(tasks)), single: make([]*classify.Command, len(tasks)), tools: make([][]string, len(tasks)), away: make([][]awayGate, len(tasks)), unsafe: make([]string, len(tasks))}
	a := &aggregator{cfg: cfg, root: root, sfx: sfx, home: dir, subs: subs, cache: map[string][]project.Task{dir: tasks}}
	results := make([]classify.Result, len(tasks))
	for i, t := range tasks {
		r.covered[i] = -1
		results[i] = classify.Body(cfg, t.Body, a.lookup(dir))
		r.tools[i] = bodyTools(results[i])
		r.away[i] = away(root, dir, subs, t, results[i])
		r.unsafe[i] = forbidden(results[i])
		r.slots[i], r.single[i] = taskSlot(cfg, t.Name, results[i])
	}
	applyFallbacks(cfg, tasks, &r)
	for i, t := range tasks {
		if globSlot(cfg, t.Name) == "" {
			continue
		}
		for _, j := range a.referenced(results[i], dir, tasks) {
			if j != i && r.slots[j] != "" && r.covered[j] < 0 {
				r.covered[j] = i
			}
		}
	}
	for i, t := range tasks {
		// A single gate an exclude glob turned away (unit-watch: vitest), or
		// any task named like a watch, fix, or e2e task, is never expanded.
		if r.slots[i] != "" || t.Body == "" || r.single[i] != nil || excludedAnywhere(cfg, t.Name) {
			continue
		}
		visited := map[string]bool{taskID(t, dir): true}
		leaves := a.walk(results[i], taskAt{t, dir}, visited, 0)
		// A missing reference is worth naming only in an aggregate that
		// holds gates; in a build or release task it's just noise.
		if slices.ContainsFunc(leaves, func(l leaf) bool { return l.slot != "" }) {
			r.leaves = append(r.leaves, leaves...)
		}
	}
	return r
}

// applyFallbacks gives each check's fallback_tasks their role, per
// provider: covered by the provider's task a Tasks glob matched, else its
// first present runs and covers the rest, so test:unit and test:ci never
// both run.
func applyFallbacks(cfg *config.Config, tasks []project.Task, r *taskRoles) {
	for _, c := range cfg.Checks {
		if len(c.FallbackTasks) == 0 {
			continue
		}
		primary := map[string]int{}
		for i, t := range tasks {
			if _, ok := primary[t.Provider]; !ok && matchesCheck(c, t.Name) {
				primary[t.Provider] = i
			}
		}
		for _, name := range c.FallbackTasks {
			for i, t := range tasks {
				if t.Name != name || excluded(c, t.Name) {
					continue
				}
				r.slots[i] = c.Name
				if p, ok := primary[t.Provider]; !ok {
					primary[t.Provider] = i
				} else if p != i {
					r.covered[i] = p
				}
			}
		}
	}
}

// awayGate is a gate a task's body runs in another subproject, after a
// literal cd into it.
type awayGate struct {
	dir, slot, tool string
}

// away is the gates r runs, as task t's body in home, in a subproject
// other than home: that subproject's own run then treats them as filled.
func away(root, home string, subs map[string]bool, t project.Task, r classify.Result) []awayGate {
	var out []awayGate
	dir, line := home, uint(0)
	for _, cmd := range r.Commands {
		if t.PerLine && cmd.Line != line {
			dir, line = home, cmd.Line
		}
		switch {
		case cmd.Kind == classify.Cd && len(cmd.Words) == 2 && !filepath.IsAbs(cmd.Words[1]):
			dir = filepath.Clean(filepath.Join(dir, cmd.Words[1]))
		case cmd.Kind == classify.Gate:
			if owner := ownerOf(root, subs, dir); owner != home && within(dir, root) {
				out = append(out, awayGate{owner, cmd.Slot, cmd.Tool})
			}
		}
	}
	return out
}

// forbidden is the first command r runs with a flag its check forbids
// (eslint --fix, jest --watch), or "".
func forbidden(r classify.Result) string {
	for _, cmd := range r.Commands {
		if cmd.Forbidden != "" {
			return cmd.Forbidden
		}
	}
	return ""
}

// unsafeSkip is the SKIP reason for a check task whose body runs cmd.
func unsafeSkip(cmd string) string {
	return "runs `" + cmd + "`, which a gate never runs"
}

// bodyTools is the commands r runs directly: each gate's tool, and the
// first word of anything else that runs.
func bodyTools(r classify.Result) []string {
	var out []string
	for _, cmd := range r.Commands {
		switch {
		case cmd.Kind == classify.Gate:
			out = append(out, cmd.Tool)
		case cmd.Kind == classify.Other && len(cmd.Words) > 0 && !cmd.Expansion:
			out = append(out, filepath.Base(cmd.Words[0]))
		}
	}
	return out
}

// aggregator walks aggregates in one subproject (home). subs is every
// subproject directory: a reference into another one is left to that
// subproject's own run, so it doesn't run twice. place is set for CI steps:
// an inline gate there belongs to whichever subproject it runs in, which
// place names with its stack and label suffix. labelsOnly skips resolving
// tools, for a list of what CI holds.
type aggregator struct {
	cfg        *config.Config
	root       string
	sfx        string
	home       string
	subs       map[string]bool
	cache      map[string][]project.Task
	place      func(dir string) (sub, stack, sfx string)
	labelsOnly bool
}

// shellState is what the commands before a gate in a body have set up: the
// directory a literal cd moved to, exported env, and the first command the
// kit can't carry over (a stateful builtin, a cd it can't follow).
type shellState struct {
	dir     string
	env     []string
	blocker string
}

func (a *aggregator) tasksIn(dir string) []project.Task {
	if ts, ok := a.cache[dir]; ok {
		return ts
	}
	ts := project.Tasks(a.cfg, dir)
	a.cache[dir] = ts
	return ts
}

func (a *aggregator) lookup(dir string) classify.Lookup {
	return func(provider, rel, name string) bool {
		_, ok := a.find(dir, classify.TaskRef{Provider: provider, Dir: rel, Name: name})
		return ok
	}
}

// inRoot is the directory a relative path leads to from dir, when it stays
// inside the root; an absolute or ~ path never does.
func (a *aggregator) inRoot(dir, path string) (string, bool) {
	if filepath.IsAbs(path) || strings.HasPrefix(path, "~") {
		return "", false
	}
	target := filepath.Clean(filepath.Join(dir, path))
	return target, within(target, a.root)
}

// taskSlot is the check a task counts as: its name's glob slot, else its
// single gate's slot unless an exclude glob turns it away. single is that
// gate, nil when the body isn't one.
func taskSlot(cfg *config.Config, name string, r classify.Result) (slot string, single *classify.Command) {
	slot = globSlot(cfg, name)
	if gate, ok := r.SingleGate(); ok {
		single = &gate
		if slot == "" && !excludedBy(cfg, gate.Slot, name) {
			slot = gate.Slot
		}
	}
	return slot, single
}

// within is true when path is dir or lies under it.
func within(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, dir+"/")
}

// elsewhere is true when dir belongs to a subproject other than home: one
// nested deeper than home, or outside home altogether. That subproject's own
// run covers it.
func (a *aggregator) elsewhere(dir string) bool {
	return ownerOf(a.root, a.subs, dir) != a.home
}

// ownerOf is the deepest subproject directory of subs holding dir, else
// root.
func ownerOf(root string, subs map[string]bool, dir string) string {
	owner := root
	for sub := range subs {
		if within(dir, sub) && len(sub) > len(owner) {
			owner = sub
		}
	}
	return owner
}

// find is the task ref names, relative to dir; never outside the root.
func (a *aggregator) find(dir string, ref classify.TaskRef) (taskAt, bool) {
	target, ok := a.inRoot(dir, ref.Dir)
	if !ok {
		return taskAt{}, false
	}
	for _, t := range a.tasksIn(target) {
		if t.Provider == ref.Provider && t.Name == ref.Name {
			return taskAt{t, target}, true
		}
	}
	return taskAt{}, false
}

// expandRefs replaces each package-script reference holding a glob (run-s
// lint:*) with the scripts in dir it matches, as npm-run-all does: * within
// one :-separated segment, ** across them.
func (a *aggregator) expandRefs(dir string, refs []classify.TaskRef) []classify.TaskRef {
	var out []classify.TaskRef
	for _, ref := range refs {
		if ref.Provider != "package-scripts" || !strings.Contains(ref.Name, "*") {
			out = append(out, ref)
			continue
		}
		pattern := regexp.QuoteMeta(ref.Name)
		pattern = strings.ReplaceAll(pattern, `\*\*`, ".+")
		pattern = strings.ReplaceAll(pattern, `\*`, "[^:]+")
		re := regexp.MustCompile("^" + pattern + "$")
		target, _ := a.inRoot(dir, ref.Dir)
		for _, t := range a.tasksIn(target) {
			if t.Provider == ref.Provider && re.MatchString(t.Name) {
				out = append(out, classify.TaskRef{Provider: ref.Provider, Dir: ref.Dir, Name: t.Name, Args: ref.Args})
			}
		}
	}
	return out
}

// referenced is the indexes into tasks of the same-directory tasks r runs.
func (a *aggregator) referenced(r classify.Result, dir string, tasks []project.Task) []int {
	var out []int
	for _, cmd := range r.Commands {
		for _, ref := range a.expandRefs(dir, cmd.Refs) {
			if ref.Dir != "" {
				continue
			}
			for j, t := range tasks {
				if t.Provider == ref.Provider && t.Name == ref.Name {
					out = append(out, j)
				}
			}
		}
	}
	return out
}

// walk collects the gates aggregate agg's body r runs. Inline gates carry
// the literal cd and exports before them in the body; one after another
// stateful command can't run on its own and is skipped with the reason.
// Commands that are neither (a build, codegen) are named on the leaf. In a
// recipe whose lines each run in their own shell, that state ends with its
// line.
func (a *aggregator) walk(r classify.Result, agg taskAt, visited map[string]bool, depth int) []leaf {
	var out []leaf
	st := shellState{dir: agg.dir}
	var setup []string
	stack := cmp.Or(agg.task.Stack, agg.task.Provider)
	block := func(cmd classify.Command) {
		if st.blocker == "" {
			st.blocker = strings.Join(cmd.Words, " ")
		}
	}
	line := uint(0)
	for _, cmd := range r.Commands {
		if agg.task.PerLine && cmd.Line != line {
			st, line = shellState{dir: agg.dir}, cmd.Line
		}
		switch cmd.Kind {
		case classify.Cd:
			target, ok := "", false
			if len(cmd.Words) == 2 {
				target, ok = a.inRoot(st.dir, cmd.Words[1])
			}
			if !ok {
				block(cmd)
				continue
			}
			st.dir = target
		case classify.Export:
			st.env = append(st.env, cmd.Env...)
		case classify.Other:
			switch {
			case len(cmd.Words) == 0:
			case cmd.Words[0] == "set" && len(cmd.Words) > 1 && !slices.ContainsFunc(cmd.Words[1:], func(w string) bool { return !slices.Contains(strictOptions, w) }):
			case slices.Contains(stateful, cmd.Words[0]),
				// cd "$DIR", FOO=$BAR: a directory or value the kit can't know.
				cmd.Expansion && (cmd.Words[0] == "cd" || strings.HasSuffix(cmd.Words[0], "=$")):
				block(cmd)
			default:
				setup = append(setup, strings.Join(cmd.Words, " "))
			}
		case classify.Gate:
			if a.place != nil || !a.elsewhere(st.dir) {
				out = append(out, a.inline(cmd, agg, stack, st, setup))
			}
		case classify.Ref:
			for _, ref := range a.expandRefs(st.dir, cmd.Refs) {
				leaves, gateless := a.reference(ref, cmd.Globs, agg, stack, st, visited, depth)
				out = append(out, leaves...)
				if gateless {
					setup = append(setup, strings.Join(cmd.Words, " "))
				}
			}
		}
	}
	return out
}

func (a *aggregator) inline(cmd classify.Command, agg taskAt, stack string, st shellState, setup []string) leaf {
	sub, sfx := "", a.sfx
	if a.place != nil {
		sub, stack, sfx = a.place(st.dir)
	}
	label := fmt.Sprintf("%s: %s (%s: %s)%s", stack, cmd.Slot, agg.task.Name, cmd.Tool, sfx)
	l := leaf{id: "inline\x00" + st.dir + "\x00" + cmd.Slot + "\x00" + strings.Join(cmd.Words, " "), slot: cmd.Slot, label: label, inline: true, tools: []string{cmd.Tool}, sub: sub}
	if st.blocker != "" {
		l.gate = Gate{Label: label, Skip: "depends on `" + st.blocker + "` in " + agg.task.Name}
		return l
	}
	if a.labelsOnly {
		l.gate = Gate{Label: label, Words: cmd.Words}
		return l
	}
	res := a.resolveWord(st.dir, cmd.Words[0])
	if res.Words == nil {
		l.gate = Gate{Label: label, Skip: res.Skip, Unrun: res.Project}
		return l
	}
	words := append(slices.Clone(res.Words), expandGlobs(cmd.Words[1:], cmd.Globs, st.dir)...)
	if all := append(slices.Clone(st.env), cmd.Env...); len(all) > 0 {
		words = append(append([]string{"env"}, all...), words...)
	}
	l.gate = Gate{Label: label, Dir: st.dir, Words: words, BinLine: res.BinLine(), Note: setupNote(agg.task.Name, setup),
		Scope: scopeFor(checkNamed(a.cfg, cmd.Slot), &cmd)}
	return l
}

// resolveWord resolves a command's first word: a path (./node_modules/.bin/
// eslint, bin/rubocop) is that file when it's an executable inside the
// repo; a bare name goes through tools.Resolve.
func (a *aggregator) resolveWord(dir, word string) tools.Resolution {
	if !strings.Contains(word, "/") {
		return tools.Resolve(a.cfg, dir, a.root, word, tools.Default)
	}
	path := absUnder(dir, word)
	if project.IsExecutable(path) && strings.HasPrefix(path, a.root+"/") {
		return tools.Resolution{Words: []string{path}, Source: tools.SourceLocal}
	}
	return tools.Resolution{Skip: word + " not found in the repo"}
}

// expandGlobs expands each word of words in globs as a shell would, from
// dir; one matching nothing stays as written, as in sh.
func expandGlobs(words, globs []string, dir string) []string {
	if len(globs) == 0 {
		return words
	}
	var out []string
	for _, w := range words {
		matches, _ := filepath.Glob(filepath.Join(dir, w))
		if !slices.Contains(globs, w) || len(matches) == 0 {
			out = append(out, w)
			continue
		}
		for _, m := range matches {
			if r, err := filepath.Rel(dir, m); err == nil && !filepath.IsAbs(w) {
				m = r
			}
			out = append(out, m)
		}
	}
	return out
}

// reference follows one task reference of an aggregate, from the
// directory and env the commands before it set up: a slot-named or
// single-gate task is a leaf run as itself; another aggregate is walked in
// turn; anything else runs nothing and is gateless (a build step). A task
// in another subproject is left to that subproject's own run. Arguments
// passed to a gate count only when the gate still is one with them
// (npm run test -- --coverage); otherwise (-- --fix, -- --watch) the
// reference runs no gate.
func (a *aggregator) reference(ref classify.TaskRef, globs []string, agg taskAt, stack string, st shellState, visited map[string]bool, depth int) (leaves []leaf, gateless bool) {
	target, ok := a.find(st.dir, ref)
	if !ok {
		label := fmt.Sprintf("%s: %s%s", stack, agg.task.Name, a.sfx)
		return []leaf{{id: "missing\x00" + st.dir + "\x00" + ref.Name, label: label,
			gate: Gate{Label: label, Skip: "references missing task " + ref.Name}}}, false
	}
	if a.elsewhere(target.dir) {
		return nil, false
	}
	id := taskID(target.task, target.dir)
	if visited[id] || depth >= maxDepth {
		return nil, false
	}
	visited[id] = true
	r := classify.Body(a.cfg, target.task.Body, a.lookup(target.dir))
	slot, single := taskSlot(a.cfg, target.task.Name, r)
	if slot == "" {
		if excludedAnywhere(a.cfg, target.task.Name) {
			return nil, true
		}
		leaves = a.walk(r, target, visited, depth+1)
		return leaves, len(leaves) == 0
	}
	words := strings.Fields(target.task.Cmd)
	withArgs := len(ref.Args) > 0 && target.task.Provider == "package-scripts"
	if withArgs {
		passed := classify.Body(a.cfg, target.task.Body+" "+tools.ShellJoin(ref.Args), a.lookup(target.dir))
		if !sameGates(r, passed) {
			return nil, false
		}
		words = append(append(words, "--"), expandGlobs(ref.Args, globs, st.dir)...)
	}
	if len(st.env) > 0 {
		words = append(append([]string{"env"}, st.env...), words...)
	}
	sub, sfx := "", a.sfx
	if a.place != nil {
		sub, stack, sfx = a.place(target.dir)
	}
	if target.dir != a.home {
		sfx = " [" + project.Rel(a.root, target.dir) + "]"
	}
	label := fmt.Sprintf("%s: %s (%s)%s", stack, slot, target.task.Name, sfx)
	gate := Gate{Label: label, Dir: target.dir, Words: words, Scope: scopeFor(checkNamed(a.cfg, slot), single)}
	switch unsafe := forbidden(r); {
	case unsafe != "":
		gate = Gate{Label: label, Skip: unsafeSkip(unsafe)}
	case st.blocker != "":
		gate = Gate{Label: label, Skip: "depends on `" + st.blocker + "` in " + agg.task.Name}
	}
	return []leaf{{id: id, slot: slot, label: label, withArgs: withArgs, tools: bodyTools(r), gate: gate, sub: sub}}, false
}

// sameGates is true when b runs the same commands as a, its gates still
// gates of the same checks.
func sameGates(a, b classify.Result) bool {
	return b.Opaque == "" && slices.EqualFunc(a.Commands, b.Commands, func(x, y classify.Command) bool {
		return x.Kind == y.Kind && x.Slot == y.Slot
	})
}

// ciLeaves reads every CI step like an aggregate and files each gate under
// the subproject it runs in, which a cd inside the step can move it to:
// the tasks a step runs are leaves run as themselves, and a tool it runs
// directly is a leaf that runs only once it resolves from the project. A
// step that can't be read, or holds a deny_commands word, gives nothing;
// so does a reference to a task the project doesn't have (npm ci, make
// build). labelsOnly leaves tools unresolved.
func (p *planner) ciLeaves(labelsOnly bool) map[string][]leaf {
	out := map[string][]leaf{}
	place := func(dir string) (string, string, string) {
		owner := ownerOf(p.root, p.subDirs, dir)
		sub := project.Rel(p.root, owner)
		return sub, p.stackFor(owner), project.SubLabel(sub)
	}
	cache := map[string][]project.Task{}
	for _, step := range ci.Steps(p.cfg, p.root) {
		dir := filepath.Clean(filepath.Join(p.root, step.Dir))
		if !within(dir, p.root) {
			continue
		}
		a := &aggregator{cfg: p.cfg, root: p.root, home: ownerOf(p.root, p.subDirs, dir), subs: p.subDirs, cache: cache, place: place, labelsOnly: labelsOnly}
		body := step.Run
		if len(step.Env) > 0 {
			quoted := make([]string, len(step.Env))
			for i, kv := range step.Env {
				k, v, _ := strings.Cut(kv, "=")
				quoted[i] = k + "='" + strings.ReplaceAll(v, "'", `'\''`) + "'"
			}
			body = "export " + strings.Join(quoted, " ") + "\n" + body
		}
		r := classify.Body(p.cfg, body, a.lookup(dir))
		if r.Opaque != "" || slices.ContainsFunc(r.Commands, func(c classify.Command) bool { return deniedCommand(p.cfg, c.Words) }) {
			continue
		}
		task := project.Task{Provider: "ci", Name: step.File}
		for _, l := range a.walk(r, taskAt{task, dir}, map[string]bool{}, 0) {
			if l.slot == "" {
				continue
			}
			l.gate.CI = step.File
			out[l.sub] = append(out[l.sub], l)
		}
	}
	return out
}

// deniedCommand is true when words hold a deny_commands token: one word
// anywhere, or several in a row.
func deniedCommand(cfg *config.Config, words []string) bool {
	for _, token := range cfg.GateDiscovery.DenyCommands {
		t := strings.Fields(token)
		for i := 0; i+len(t) <= len(words); i++ {
			if slices.Equal(words[i:i+len(t)], t) {
				return true
			}
		}
	}
	return false
}

func setupNote(agg string, setup []string) string {
	if len(setup) == 0 {
		return ""
	}
	return agg + " runs `" + strings.Join(setup, "`, `") + "` first; run-checks doesn't"
}

func taskID(t project.Task, dir string) string {
	return t.Provider + "\x00" + dir + "\x00" + t.Name
}

// globSlot is the first check whose task globs t's name matches.
func globSlot(cfg *config.Config, name string) string {
	for _, c := range cfg.Checks {
		if matchesCheck(c, name) {
			return c.Name
		}
	}
	return ""
}

// excludedAnywhere is true when any check's exclude globs cover name.
func excludedAnywhere(cfg *config.Config, name string) bool {
	return slices.ContainsFunc(cfg.Checks, func(c config.Check) bool { return excluded(c, name) })
}

// excludedBy is true when check slot's exclude globs cover name.
func excludedBy(cfg *config.Config, slot, name string) bool {
	return excluded(checkNamed(cfg, slot), name)
}
