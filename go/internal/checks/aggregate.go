package checks

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

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

// taskRoles is how each task of one directory takes part in run-checks.
type taskRoles struct {
	slots   []string            // per task: the check it fills, by name or by a single-gate body
	covered []string            // per task: the slot-named task whose body already runs it
	single  []*classify.Command // per task: its body's one gate, when it has exactly one
	leaves  []leaf              // gates found inside aggregate tasks, in body order
}

// leaf is one gate inside an aggregate: another task, run as itself, or an
// inline command, run as words. withArgs: a task run with the arguments the
// aggregate passes it, which beats a plain run of the same task.
type leaf struct {
	id, slot, label  string
	withArgs, inline bool
	gate             Gate
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
func roles(cfg *config.Config, root, dir, sfx string, tasks []project.Task) taskRoles {
	r := taskRoles{slots: make([]string, len(tasks)), covered: make([]string, len(tasks)), single: make([]*classify.Command, len(tasks))}
	a := &aggregator{cfg: cfg, root: root, sfx: sfx, cache: map[string][]project.Task{dir: tasks}}
	results := make([]classify.Result, len(tasks))
	for i, t := range tasks {
		results[i] = classify.Body(cfg, t.Body, a.lookup(dir))
		gate, single := results[i].SingleGate()
		if single {
			r.single[i] = &gate
		}
		if slot := globSlot(cfg, t.Name); slot != "" {
			r.slots[i] = slot
		} else if single && !excludedBy(cfg, gate.Slot, t.Name) {
			r.slots[i] = gate.Slot
		}
	}
	applyFallbacks(cfg, tasks, &r)
	for i, t := range tasks {
		if globSlot(cfg, t.Name) == "" {
			continue
		}
		for _, j := range a.referenced(results[i], dir, tasks) {
			if j != i && r.slots[j] != "" && r.covered[j] == "" {
				r.covered[j] = t.Name
			}
		}
	}
	for i, t := range tasks {
		// A single gate an exclude glob turned away (unit-watch: vitest), or
		// any task named like a watch, fix, or e2e task, is never expanded.
		if _, single := results[i].SingleGate(); r.slots[i] != "" || t.Body == "" || single || excludedAnywhere(cfg, t.Name) {
			continue
		}
		visited := map[string]bool{taskID(t, dir): true}
		r.leaves = append(r.leaves, a.walk(results[i], taskAt{t, dir}, visited, 0)...)
	}
	return r
}

// applyFallbacks gives each check's fallback_tasks their role: covered by
// the task a Tasks glob matched, else the first present runs and covers
// the rest, so test:unit and test:ci never both run.
func applyFallbacks(cfg *config.Config, tasks []project.Task, r *taskRoles) {
	for _, c := range cfg.Checks {
		if len(c.FallbackTasks) == 0 {
			continue
		}
		primary := ""
		for _, t := range tasks {
			if matchesCheck(c, t.Name) {
				primary = t.Name
				break
			}
		}
		for _, name := range c.FallbackTasks {
			for i, t := range tasks {
				if t.Name != name || excluded(c, t.Name) {
					continue
				}
				r.slots[i] = c.Name
				if primary == "" {
					primary = t.Name
				} else {
					r.covered[i] = primary
				}
			}
		}
	}
}

type aggregator struct {
	cfg   *config.Config
	root  string
	sfx   string
	cache map[string][]project.Task
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

// find is the task ref names, relative to dir; never outside the root.
func (a *aggregator) find(dir string, ref classify.TaskRef) (taskAt, bool) {
	target := filepath.Clean(filepath.Join(dir, ref.Dir))
	if target != a.root && !strings.HasPrefix(target, a.root+"/") {
		return taskAt{}, false
	}
	for _, t := range a.tasksIn(target) {
		if t.Provider == ref.Provider && t.Name == ref.Name {
			return taskAt{t, target}, true
		}
	}
	return taskAt{}, false
}

// referenced is the indexes into tasks of the same-directory tasks r runs.
func (a *aggregator) referenced(r classify.Result, dir string, tasks []project.Task) []int {
	var out []int
	for _, cmd := range r.Commands {
		for _, ref := range cmd.Refs {
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
// Commands that are neither (a build, codegen) are named on the leaf.
func (a *aggregator) walk(r classify.Result, agg taskAt, visited map[string]bool, depth int) []leaf {
	var out []leaf
	dir, env, blocker := agg.dir, []string(nil), ""
	var setup []string
	stack := agg.task.Stack
	if stack == "" {
		stack = agg.task.Provider
	}
	for _, cmd := range r.Commands {
		switch cmd.Kind {
		case classify.Cd:
			target := ""
			if len(cmd.Words) == 2 {
				target = filepath.Clean(filepath.Join(dir, cmd.Words[1]))
			}
			if target == "" || (target != a.root && !strings.HasPrefix(target, a.root+"/")) {
				blocker = strings.Join(cmd.Words, " ")
				continue
			}
			dir = target
		case classify.Export:
			env = append(env, cmd.Env...)
		case classify.Other:
			if len(cmd.Words) > 0 && slices.Contains(stateful, cmd.Words[0]) {
				blocker = strings.Join(cmd.Words, " ")
			} else if len(cmd.Words) > 0 {
				setup = append(setup, strings.Join(cmd.Words, " "))
			}
		case classify.Gate:
			out = append(out, a.inline(cmd, agg, stack, dir, env, blocker, setup))
		case classify.Ref:
			for _, ref := range cmd.Refs {
				leaves, gateless := a.reference(ref, agg, stack, visited, depth)
				out = append(out, leaves...)
				if gateless {
					setup = append(setup, strings.Join(cmd.Words, " "))
				}
			}
		}
	}
	return out
}

func (a *aggregator) inline(cmd classify.Command, agg taskAt, stack, dir string, env []string, blocker string, setup []string) leaf {
	label := fmt.Sprintf("%s: %s (%s: %s)%s", stack, cmd.Slot, agg.task.Name, cmd.Tool, a.sfx)
	l := leaf{id: "inline\x00" + dir + "\x00" + cmd.Slot + "\x00" + strings.Join(cmd.Words, " "), slot: cmd.Slot, label: label, inline: true}
	if blocker != "" {
		l.gate = Gate{Label: label, Skip: "depends on `" + blocker + "` in " + agg.task.Name}
		return l
	}
	res := tools.Resolve(a.cfg, dir, a.root, cmd.Words[0], tools.Default)
	if res.Words == nil {
		l.gate = Gate{Label: label, Skip: res.Skip}
		return l
	}
	words := append(slices.Clone(res.Words), cmd.Words[1:]...)
	if all := append(slices.Clone(env), cmd.Env...); len(all) > 0 {
		words = append(append([]string{"env"}, all...), words...)
	}
	l.gate = Gate{Label: label, Dir: dir, Words: words, BinLine: res.BinLine(), Note: setupNote(agg.task.Name, setup),
		Scope: scopeFor(checkNamed(a.cfg, cmd.Slot), &cmd)}
	return l
}

// reference follows one task reference of an aggregate: a slot-named or
// single-gate task is a leaf run as itself; another aggregate is walked in
// turn; anything else runs nothing and is gateless (a build step).
func (a *aggregator) reference(ref classify.TaskRef, agg taskAt, stack string, visited map[string]bool, depth int) (leaves []leaf, gateless bool) {
	target, ok := a.find(agg.dir, ref)
	if !ok {
		label := fmt.Sprintf("%s: %s%s", stack, agg.task.Name, a.sfx)
		return []leaf{{id: "missing\x00" + agg.dir + "\x00" + ref.Name, label: label,
			gate: Gate{Label: label, Skip: "references missing task " + ref.Name}}}, false
	}
	id := taskID(target.task, target.dir)
	if visited[id] || depth >= maxDepth {
		return nil, false
	}
	visited[id] = true
	r := classify.Body(a.cfg, target.task.Body, a.lookup(target.dir))
	slot := globSlot(a.cfg, target.task.Name)
	var single *classify.Command
	if gate, ok := r.SingleGate(); ok {
		single = &gate
		if slot == "" && !excludedBy(a.cfg, gate.Slot, target.task.Name) {
			slot = gate.Slot
		}
	}
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
		words = append(append(words, "--"), ref.Args...)
	}
	sfx := a.sfx
	if target.dir != agg.dir {
		sfx = " [" + rel(a.root, target.dir) + "]"
	}
	label := fmt.Sprintf("%s: %s (%s)%s", stack, slot, target.task.Name, sfx)
	gate := Gate{Label: label, Dir: target.dir, Words: words, Scope: scopeFor(checkNamed(a.cfg, slot), single)}
	return []leaf{{id: id, slot: slot, label: label, withArgs: withArgs, gate: gate}}, false
}

// ciLeaves reads the CI steps that run in subproject sub like aggregates:
// the tasks a step runs are leaves run as themselves, and a tool it runs
// directly is a leaf only once it resolves from the project. A step that
// can't be read, or holds a deny_commands word, gives nothing; so does a
// reference to a task the project doesn't have (npm ci, make build).
func (p *planner) ciLeaves(sub, sfx, stack string) []leaf {
	if stack == "" {
		stack = "ci"
	}
	a := &aggregator{cfg: p.cfg, root: p.root, sfx: sfx, cache: map[string][]project.Task{}}
	var out []leaf
	for _, step := range p.ci[sub] {
		dir := filepath.Clean(filepath.Join(p.root, step.dir))
		if dir != p.root && !strings.HasPrefix(dir, p.root+"/") {
			continue
		}
		body := step.run
		if len(step.env) > 0 {
			quoted := make([]string, len(step.env))
			for i, kv := range step.env {
				k, v, _ := strings.Cut(kv, "=")
				quoted[i] = k + "='" + strings.ReplaceAll(v, "'", `'\''`) + "'"
			}
			body = "export " + strings.Join(quoted, " ") + "\n" + body
		}
		r := classify.Body(p.cfg, body, a.lookup(dir))
		if r.Opaque != "" || slices.ContainsFunc(r.Commands, func(c classify.Command) bool { return deniedCommand(p.cfg, c.Words) }) {
			continue
		}
		task := project.Task{Provider: "ci", Stack: stack, Name: step.file}
		for _, l := range a.walk(r, taskAt{task, dir}, map[string]bool{}, 0) {
			if l.slot == "" {
				continue
			}
			l.gate.CI = step.file
			out = append(out, l)
		}
	}
	return out
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
	for _, c := range cfg.Checks {
		if c.Name == slot {
			return excluded(c, name)
		}
	}
	return false
}

func rel(root, path string) string {
	if path == root {
		return "."
	}
	return strings.TrimPrefix(path, root+"/")
}
