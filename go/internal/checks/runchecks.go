// Package checks runs a project's checks: every declared check task in
// every subproject (run-checks), and file-scoped checks on edited files (the
// Stop hook).
package checks

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/extract"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/tools"
)

// Gate is one check run-checks runs, or skips with the reason. Label is
// "<stack>: <check> (<task>) [<subproject>]" (the root has no bracket part).
type Gate struct {
	Label   string
	Dir     string
	Words   []string
	BinLine string // "  bin: <words> (<source>)\n" when the kit resolved the binary
	Note    string // printed as "  note: <note>" under the verdict
	Skip    string
	Unrun   bool   // skipped though the project names its tool: not installed
	CI      string // the CI config it came from, when only CI names it
	Scope   *Scope // set: findings in changed files decide (dead code)
}

// extra is the lines printed under g's verdict: its bin line and note.
func (g Gate) extra() string {
	if g.Note == "" {
		return g.BinLine
	}
	return g.BinLine + "  note: " + g.Note + "\n"
}

// planner accumulates one plan; orchestrated holds the checks an
// orchestrator covers, whose JS tasks are then left to it.
type planner struct {
	cfg          *config.Config
	root         string
	gates        []Gate
	orchestrated map[string]bool
	ci           map[string][]leaf // CI gates by the subproject they run in
	subDirs      map[string]bool   // every subproject's absolute directory
	// Per subproject directory, gates an earlier subproject's task runs
	// there (cd svc && go test). Subprojects plan root first, so a task
	// that cds into a sibling planned before it doesn't count.
	awayFilled map[string]map[string][]filler
}

// filler is a gate that runs for a check, and the tools it runs: an inline
// tool line or a toolchain check running one of the same tools is then
// covered.
type filler struct {
	label string
	tools []string
}

func (p *planner) add(g Gate) { p.gates = append(p.gates, g) }

// Gates is every check of every subproject of root, or only those named in
// only ("." is the root), in the order run-checks runs them.
func Gates(cfg *config.Config, root string, only []string) []Gate {
	p := &planner{cfg: cfg, root: root, orchestrated: map[string]bool{}, awayFilled: map[string]map[string][]filler{}}
	inScope := func(sub string) bool { return len(only) == 0 || slices.Contains(only, sub) }

	// The orchestrator only when a JS subproject is in scope: a run scoped to
	// a Python service has nothing for turbo or nx to do.
	wantsJS := len(only) == 0
	for _, sub := range only {
		for _, pr := range project.Providers(cfg, dirOf(root, sub)) {
			if pr.Stack == "js" {
				wantsJS = true
			}
		}
	}
	if wantsJS {
		p.orchestrate()
	}
	subs := p.subprojects()
	p.ci = p.ciLeaves(false)
	for _, sub := range subs {
		if inScope(sub) {
			p.subproject(sub)
		}
	}
	return p.gates
}

func (p *planner) subprojects() []string {
	subs := project.Subprojects(p.cfg, p.root)
	p.subDirs = map[string]bool{}
	for _, sub := range subs {
		p.subDirs[dirOf(p.root, sub)] = true
	}
	return subs
}

// CIGates is the labels of the gates root's CI config runs, without
// resolving their tools: run-checks runs each one whose tool the project
// has, unless a gate running the same tool already fills its check.
func CIGates(cfg *config.Config, root string) []string {
	if !HasCI(root) {
		return nil
	}
	p := &planner{cfg: cfg, root: root}
	subs := p.subprojects()
	leaves := p.ciLeaves(true)
	var out []string
	for _, sub := range subs {
		for _, l := range leaves[sub] {
			if l.gate.Skip == "" {
				out = append(out, l.label)
			}
		}
	}
	return out
}

// stackFor is the stack CI gates in dir take: the detected stack (go), not
// a stackless provider's name (make), when there is one; else the first
// provider's stack or name; else "ci".
func (p *planner) stackFor(dir string) string {
	for _, name := range p.cfg.StackOrder {
		if name != "monorepo" && p.cfg.HasStack(dir, name) {
			return name
		}
	}
	if prs := project.Providers(p.cfg, dir); len(prs) > 0 {
		if prs[0].Stack != "" {
			return prs[0].Stack
		}
		return prs[0].Name
	}
	return "ci"
}

// RunAll runs Gates' checks and returns the failure count. Output contract,
// parsed by callers: one PASS, FAIL, or SKIP line per check, labeled as Gate
// says; a check whose binary the kit resolved has an indented "  bin:
// <words> (<source>)" line right under it; then a blank line and "checks: N
// passed, N failed, N skipped", and, when the project's own tools weren't
// installed, a "not run: ..." line saying so.
func RunAll(cfg *config.Config, root string, only []string, out io.Writer) int {
	pass, fail, skip, unrun := 0, 0, 0, 0
	var ch *changes
	for _, g := range Gates(cfg, root, only) {
		switch {
		case g.Skip != "":
			fmt.Fprintf(out, "SKIP %s (%s)\n", g.Label, g.Skip)
			skip++
			if g.Unrun {
				unrun++
			}
		case g.Scope != nil:
			if ch == nil {
				c := changedSince(root)
				ch = &c
			}
			switch runScoped(g, *ch, out) {
			case "pass":
				pass++
			case "fail":
				fail++
			default:
				skip++
			}
		case run(g, out):
			pass++
		default:
			fail++
		}
	}
	fmt.Fprintf(out, "\nchecks: %d passed, %d failed, %d skipped\n", pass, fail, skip)
	if unrun > 0 {
		fmt.Fprintf(out, "not run: %d check%s the project declares a tool for, not installed; install the dependencies, then rerun\n", unrun, plural(unrun))
	}
	return fail
}

// PrintPlan writes Gates' checks without running any: "RUN <label>" with
// the command and its bin line, or "SKIP <label> (<reason>)".
func PrintPlan(cfg *config.Config, root string, only []string, out io.Writer) {
	for _, g := range Gates(cfg, root, only) {
		if g.Skip != "" {
			fmt.Fprintf(out, "SKIP %s (%s)\n", g.Label, g.Skip)
			continue
		}
		fmt.Fprintf(out, "RUN %s\n  cmd: %s\n", g.Label, tools.ShellJoin(g.Words))
		if g.Dir != root && g.Dir != "" {
			fmt.Fprintf(out, "  dir: %s\n", tools.Rel(root, g.Dir))
		}
		switch {
		case g.Scope != nil && g.Scope.Advisory:
			fmt.Fprint(out, "  scope: advisory; its findings never fail run-checks\n")
		case g.Scope != nil:
			fmt.Fprint(out, "  scope: only findings on lines changed since the git base fail it\n")
		}
		fmt.Fprint(out, g.extra())
	}
}

// gateCommand is g's command, run in its directory. pnpm 10+ installs
// before `pnpm run` when node_modules is out of sync with the lockfile, and
// run-checks never installs.
func gateCommand(g Gate) *exec.Cmd {
	cmd := exec.Command(g.Words[0], g.Words[1:]...)
	cmd.Dir = g.Dir
	cmd.Env = append(cmd.Environ(), "pnpm_config_verify_deps_before_run=false")
	return cmd
}

// run runs g and reports it, with its bin line under the verdict; a failure
// then prints the first 30 lines of the command's output.
func run(g Gate, w io.Writer) bool {
	var out bytes.Buffer
	cmd := gateCommand(g)
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(w, "FAIL %s (%s)\n%s", g.Label, strings.Join(g.Words, " "), g.extra())
		if _, isExit := err.(*exec.ExitError); !isExit {
			fmt.Fprintf(&out, "%s: %v\n", g.Words[0], err)
		}
		lines := strings.SplitAfter(out.String(), "\n")
		fmt.Fprint(w, strings.Join(lines[:min(len(lines), 30)], ""))
		return false
	}
	fmt.Fprintf(w, "PASS %s\n%s", g.Label, g.extra())
	return true
}

// matchesCheck is true when task counts as check c: it matches one of the
// check's task globs and none of its exclude globs.
func matchesCheck(c config.Check, task string) bool {
	match := func(globs []string) bool {
		return slices.ContainsFunc(globs, func(g string) bool { return guard.Glob(g, task) })
	}
	return match(c.Tasks) && !match(c.Exclude)
}

func excluded(c config.Check, task string) bool {
	return slices.ContainsFunc(c.Exclude, func(g string) bool { return guard.Glob(g, task) })
}

// excludedDir is the first path segment of subproject sub matching an
// exclude_dirs glob of check slot, or "".
func excludedDir(cfg *config.Config, slot, sub string) string {
	dirs := checkNamed(cfg, slot).ExcludeDirs
	for _, seg := range strings.Split(filepath.ToSlash(sub), "/") {
		if slices.ContainsFunc(dirs, func(g string) bool { return guard.Glob(g, seg) }) {
			return seg
		}
	}
	return ""
}

func dirOf(root, sub string) string {
	if sub == "." {
		return root
	}
	return filepath.Join(root, sub)
}

// orchestrate plans each check the first usable orchestrator declares a
// task for, once, at the root: its signal file is there and its binary is in
// the root's node_modules/.bin (never npx). Those checks' JS provider tasks
// are then skipped per package.
func (p *planner) orchestrate() {
	for _, o := range p.cfg.Orchestrators {
		signal := filepath.Join(p.root, o.Signal)
		bin := filepath.Join(p.root, "node_modules", ".bin", o.Name)
		if info, err := os.Stat(signal); err != nil || !info.Mode().IsRegular() {
			continue
		}
		if info, err := os.Stat(bin); err != nil || info.Mode()&0o111 == 0 {
			continue
		}
		var tasks []string
		for _, path := range o.TaskPaths {
			tasks = append(tasks, extract.JSONKeys(signal, path)...)
		}
		origin := tools.Resolution{Words: []string{bin}, Source: tools.SourceLocal}
		for _, c := range p.cfg.Checks {
			for _, task := range tasks {
				if !matchesCheck(c, task) {
					continue
				}
				label := fmt.Sprintf("js: %s (%s affected: %s)", c.Name, o.Name, task)
				if p.cfg.CheckDisabled(c.Name, label) {
					p.add(Gate{Label: label, Skip: "disabled_checks"})
					continue
				}
				p.orchestrated[c.Name] = true
				words := tools.Fill(strings.Fields(strings.ReplaceAll(o.Run, "{task}", task)), "{bin}", []string{bin})
				p.add(Gate{Label: label, Dir: p.root, Words: words, BinLine: origin.BinLine()})
			}
		}
		return
	}
}

func (p *planner) subproject(sub string) {
	cfg := p.cfg
	dir, sfx := dirOf(p.root, sub), ""
	if sub != "." {
		sfx = " [" + sub + "]"
	}
	tasks := project.Tasks(cfg, dir)

	// SKIP lines take the first present provider's stack (else its name),
	// so a package.json with no scripts still reports what it lacks.
	skipLabel := ""
	for _, pr := range project.Providers(cfg, dir) {
		if pr.Stack != "" {
			skipLabel = pr.Stack
			break
		}
		if skipLabel == "" {
			skipLabel = pr.Name
		}
	}

	r := roles(cfg, p.root, dir, sfx, tasks, p.subDirs)
	ci := p.ci[sub]
	seen := map[string]bool{}
	argLeaves := map[string]Gate{}
	for _, l := range r.leaves {
		// A leaf a blocker skipped has no words to run the task with.
		if l.withArgs && l.gate.Skip == "" {
			argLeaves[l.id] = l.gate
		}
	}
	stackOf := func(t project.Task) string {
		if t.Stack != "" {
			return t.Stack
		}
		return t.Provider
	}
	labelOf := func(i int) string {
		return fmt.Sprintf("%s: %s (%s)%s", stackOf(tasks[i]), r.slots[i], tasks[i].Name, sfx)
	}
	// coverer is the task whose run covers task i: the one whose body runs
	// it, or, when that one is disabled or unsafe, whatever covers that one;
	// else -1.
	coverer := func(i int) int {
		for j, n := r.covered[i], 0; j >= 0 && n < len(tasks); j, n = r.covered[j], n+1 {
			if !cfg.CheckDisabled(r.slots[j], labelOf(j)) && r.unsafe[j] == "" {
				return j
			}
		}
		return -1
	}
	// filled is, per check, the gates that run for it.
	filled := map[string][]filler{}
	for slot, fs := range p.awayFilled[dir] {
		filled[slot] = slices.Clone(fs)
	}
	fill := func(slot, label string, tools []string) {
		filled[slot] = append(filled[slot], filler{label, tools})
	}
	coveredBy := func(slot string, tools []string) string {
		for _, f := range filled[slot] {
			if slices.ContainsFunc(f.tools, func(t string) bool { return slices.Contains(tools, t) }) {
				return f.label
			}
		}
		return ""
	}
	for _, c := range cfg.Checks {
		matched := false
		for i, t := range tasks {
			if r.slots[i] != c.Name || (p.orchestrated[c.Name] && stackOf(t) == "js") {
				continue
			}
			matched = true
			seen[taskID(t, dir)] = true
			full := labelOf(i)
			switch by := coverer(i); {
			case by >= 0:
				p.add(Gate{Label: full, Skip: "covered by " + tasks[by].Name})
			case cfg.CheckDisabled(c.Name, full):
				p.add(Gate{Label: full, Skip: "disabled_checks"})
			case r.unsafe[i] != "":
				// It never fills the check, so a clean gate (a CI step) still runs.
				p.add(Gate{Label: full, Skip: unsafeSkip(r.unsafe[i])})
			default:
				g := Gate{Label: full, Dir: dir, Words: strings.Fields(t.Cmd), Scope: scopeFor(c, r.single[i])}
				if withArgs, ok := argLeaves[taskID(t, dir)]; ok {
					// An aggregate passes it arguments (npm run unit -- --coverage).
					g.Words = withArgs.Words
				}
				p.add(g)
				fill(c.Name, full, r.tools[i])
				for _, ag := range r.away[i] {
					if p.awayFilled[ag.dir] == nil {
						p.awayFilled[ag.dir] = map[string][]filler{}
					}
					p.awayFilled[ag.dir][ag.slot] = append(p.awayFilled[ag.dir][ag.slot], filler{full, []string{ag.tool}})
				}
			}
		}
		// Gates inside aggregates, then those only CI names: tasks CI runs,
		// then tools it runs directly, which yield to a gate already
		// running the same tool for the check.
		for _, l := range append(slices.Clone(r.leaves), ci...) {
			if l.slot != c.Name || seen[l.id] || (p.orchestrated[c.Name] && strings.HasPrefix(l.label, "js: ")) {
				continue
			}
			seen[l.id] = true
			by := ""
			if l.inline {
				by = coveredBy(c.Name, l.tools)
			}
			switch {
			case by == l.label && len(l.gate.Words) > 0:
				// Two steps alike but for arguments: name what this one ran.
				words := make([]string, len(l.gate.Words))
				for k, w := range l.gate.Words {
					if filepath.IsAbs(w) {
						w = filepath.Base(w)
					}
					words[k] = w
				}
				l.gate = Gate{Label: l.label, Skip: "covered by an earlier step running " + l.tools[0] + "; skipped `" + strings.Join(words, " ") + "`", CI: l.gate.CI}
			case by != "":
				l.gate = Gate{Label: l.label, Skip: "covered by " + by, CI: l.gate.CI}
			case l.gate.Skip == "" && cfg.CheckDisabled(c.Name, l.label):
				l.gate = Gate{Label: l.label, Skip: "disabled_checks", CI: l.gate.CI}
			}
			// A skipped leaf still reports the check, so no "no task" line.
			matched = true
			if l.gate.Skip == "" {
				fill(c.Name, l.label, l.tools)
			}
			p.add(l.gate)
		}
		if !matched && skipLabel != "" && (!p.orchestrated[c.Name] || skipLabel != "js") {
			reason := "no " + c.Name + " task"
			if cfg.CheckDisabled(c.Name, "") {
				reason = "disabled_checks"
			}
			p.add(Gate{Label: fmt.Sprintf("%s: %s%s", skipLabel, c.Name, sfx), Skip: reason})
		}
	}
	// An aggregate's reference to a task that doesn't exist has no slot.
	for _, l := range r.leaves {
		if l.slot == "" && !seen[l.id] {
			seen[l.id] = true
			p.add(l.gate)
		}
	}

	for _, tc := range cfg.ToolchainChecks {
		if !cfg.HasStack(dir, tc.Stack) {
			continue
		}
		label := tc.Stack + ": " + tc.Name + sfx
		if !cfg.ToolchainEnabled(tc) {
			p.add(Gate{Label: label, Skip: "disabled_toolchain_checks"})
			continue
		}
		if cfg.CheckDisabled(tc.Slot, label) {
			p.add(Gate{Label: label, Skip: "disabled_checks"})
			continue
		}
		if by := coveredBy(tc.Slot, tc.Bin); tc.Slot != "" && by != "" {
			p.add(Gate{Label: label, Skip: "covered by " + by})
			continue
		}
		if seg := excludedDir(cfg, tc.Slot, sub); seg != "" {
			p.add(Gate{Label: label, Skip: fmt.Sprintf("%s looks like a %s suite to leave out (exclude_dirs)", seg, tc.Slot)})
			continue
		}
		run := tools.ResolveToolchain(cfg, tc, dir, p.root)
		if run.Words == nil {
			p.add(Gate{Label: label, Skip: run.Skip, Unrun: run.Project})
			continue
		}
		p.add(Gate{Label: label, Dir: dir, Words: run.Words, BinLine: run.BinLine()})
	}
}
