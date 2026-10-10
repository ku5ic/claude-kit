// Package checks runs a project's checks: every declared check task in
// every subproject (run-checks), and file-scoped checks on edited files (the
// Stop hook).
package checks

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
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

// gates is every check of every subproject of root, or only those named in
// only ("." is the root), in the order run-checks runs them.
func gates(cfg *config.Config, root string, only []string) []Gate {
	p := &planner{cfg: cfg, root: root, orchestrated: map[string]bool{}, awayFilled: map[string]map[string][]filler{}}
	inScope := func(sub string) bool { return len(only) == 0 || slices.Contains(only, sub) }

	// The orchestrator only when a JS subproject is in scope: a run scoped to
	// a Python service has nothing for turbo or nx to do.
	wantsJS := len(only) == 0
	for _, sub := range only {
		for _, pr := range project.Providers(cfg, filepath.Join(root, sub)) {
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
		p.subDirs[filepath.Join(p.root, sub)] = true
	}
	return subs
}

// CIGates is the labels of the gates root's CI config runs, without
// resolving their tools: run-checks runs each one whose tool the project
// has, unless a gate running the same tool already fills its check.
func CIGates(cfg *config.Config, root string) []string {
	if !sources.Has(root) {
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
		return cmp.Or(prs[0].Stack, prs[0].Name)
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
	for _, g := range gates(cfg, root, only) {
		switch {
		case g.Skip != "":
			fmt.Fprint(out, skipLine(g.Label, g.Skip))
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
	fmt.Fprintf(out, "\n%s\n", summary(pass, fail, skip))
	if unrun > 0 {
		fmt.Fprintf(out, "not run: %d check%s the project declares a tool for, not installed; install the dependencies, then rerun\n", unrun, plural(unrun))
	}
	return fail
}

// PrintPlan writes Gates' checks without running any: "RUN <label>" with
// the command and its bin line, or "SKIP <label> (<reason>)".
func PrintPlan(cfg *config.Config, root string, only []string, out io.Writer) {
	for _, g := range gates(cfg, root, only) {
		if g.Skip != "" {
			fmt.Fprint(out, skipLine(g.Label, g.Skip))
			continue
		}
		fmt.Fprintf(out, "RUN %s\n  cmd: %s\n", g.Label, tools.ShellJoin(g.Words))
		if g.Dir != root && g.Dir != "" {
			fmt.Fprintf(out, "  dir: %s\n", fsx.Rel(root, g.Dir))
		}
		switch {
		case g.Scope != nil && g.Scope.Findings == nil:
			fmt.Fprint(out, "  scope: advisory; its findings never fail run-checks\n")
		case g.Scope != nil:
			fmt.Fprint(out, "  scope: only findings on lines changed since the git base fail it\n")
		}
		fmt.Fprint(out, g.extra())
	}
}

// gateTimeout bounds one gate: the Bash tool's own ceiling, past which the
// whole run would be killed anyway.
const gateTimeout = 10 * time.Minute

// gateCommand is g's command, run in its directory. pnpm 10+ installs
// before `pnpm run` when node_modules is out of sync with the lockfile, and
// run-checks never installs.
func gateCommand(g Gate) *proc.Cmd {
	cmd := proc.Command(gateTimeout, g.Words[0], g.Words[1:]...)
	cmd.Dir = g.Dir
	cmd.Env = append(cmd.Environ(), "pnpm_config_verify_deps_before_run=false")
	return cmd
}

// maxOutputLines bounds the output a failing check shows.
const maxOutputLines = 30

func skipLine(label, reason string) string { return "SKIP " + label + " (" + reason + ")\n" }

func summary(pass, fail, skip int) string {
	return fmt.Sprintf("checks: %d passed, %d failed, %d skipped", pass, fail, skip)
}

// Summary matches the line summary prints; --plan doesn't print it.
var Summary = regexp.MustCompile(`checks: \d+ passed, \d+ failed, \d+ skipped`)

// head is lines' first maxOutputLines.
func head(lines []string) []string { return lines[:min(len(lines), maxOutputLines)] }

// capture runs g's command for its combined output; a command that didn't
// run at all (not a non-zero exit) says why at the end of it.
func capture(g Gate) (string, error) {
	var out bytes.Buffer
	cmd := gateCommand(g)
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if _, isExit := err.(*exec.ExitError); err != nil && !isExit {
		fmt.Fprintf(&out, "%s: %v\n", g.Words[0], err)
	}
	return out.String(), err
}

// failLine is a gate's FAIL line with its command, and its bin line under it.
func failLine(g Gate) string {
	return fmt.Sprintf("FAIL %s (%s)\n%s", g.Label, strings.Join(g.Words, " "), g.extra())
}

// run runs g and reports it, with its bin line under the verdict; a failure
// then prints the first lines of the command's output.
func run(g Gate, w io.Writer) bool {
	out, err := capture(g)
	if err != nil {
		fmt.Fprint(w, failLine(g)+strings.Join(head(strings.SplitAfter(out, "\n")), ""))
		return false
	}
	fmt.Fprintf(w, "PASS %s\n%s", g.Label, g.extra())
	return true
}

// matchesCheck is true when task counts as check c: it matches one of the
// check's task globs and none of its exclude globs.
func matchesCheck(c config.Check, task string) bool {
	return guard.GlobAny(c.Tasks, task) && !excluded(c, task)
}

func excluded(c config.Check, task string) bool {
	return guard.GlobAny(c.Exclude, task)
}

// globIndex is the index of the first of globs matching name, or -1.
func globIndex(globs []string, name string) int {
	return slices.IndexFunc(globs, func(g string) bool { return guard.Glob(g, name) })
}

// excludedTask is the first of tasks named like check c (its task or
// fallback globs) that an exclude glob turns away, and that glob.
func excludedTask(c config.Check, tasks []project.Task) (name, glob string) {
	names := slices.Concat(c.Tasks, c.FallbackTasks)
	for _, t := range tasks {
		if !guard.GlobAny(names, t.Name) {
			continue
		}
		if i := globIndex(c.Exclude, t.Name); i >= 0 {
			return t.Name, c.Exclude[i]
		}
	}
	return "", ""
}

// ToolchainSkip is why toolchain check tc doesn't run in subproject sub, or
// "" when it runs. coveredBy names a gate already filling the check's slot;
// it needs the planner's state, so callers without one pass nil.
func ToolchainSkip(cfg *config.Config, tc config.ToolchainCheck, sub string, coveredBy func(slot string, tools []string) string) string {
	label := tc.Stack + ": " + tc.Name + project.SubLabel(sub)
	switch {
	case !cfg.ToolchainEnabled(tc):
		return "disabled_toolchain_checks"
	case cfg.CheckDisabled(tc.Slot, label):
		return "disabled_checks"
	}
	if coveredBy != nil && tc.Slot != "" {
		if by := coveredBy(tc.Slot, tc.Bin); by != "" {
			return "covered by " + by
		}
	}
	if seg := excludedDir(cfg, tc.Slot, sub); seg != "" {
		return fmt.Sprintf("%s looks like a %s suite to leave out (exclude_dirs)", seg, tc.Slot)
	}
	return ""
}

// excludedDir is the first path segment of subproject sub matching an
// exclude_dirs glob of check slot, or "".
func excludedDir(cfg *config.Config, slot, sub string) string {
	dirs := checkNamed(cfg, slot).ExcludeDirs
	for _, seg := range strings.Split(filepath.ToSlash(sub), "/") {
		if guard.GlobAny(dirs, seg) {
			return seg
		}
	}
	return ""
}

// orchestrate plans each check the first usable orchestrator declares a
// task for, once, at the root: its signal file is there and its binary is in
// the root's node_modules/.bin (never npx). Those checks' JS provider tasks
// are then skipped per package.
func (p *planner) orchestrate() {
	for _, o := range p.cfg.Orchestrators {
		signal := filepath.Join(p.root, o.Signal)
		bin := filepath.Join(p.root, "node_modules", ".bin", o.Name)
		if !fsx.IsFile(signal) || !fsx.IsExecutable(bin) {
			continue
		}
		var tasks []string
		for _, path := range o.TaskPaths {
			tasks = append(tasks, sources.JSONKeys(signal, path)...)
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

// subPlan is one subproject's planning state: its tasks, their roles, and
// which gates fill each check so far.
type subPlan struct {
	name, dir, sfx string
	tasks          []project.Task
	r              taskRoles
	skipLabel      string              // SKIP lines' stack, else provider
	seen           map[string]bool     // leaves already planned, by id
	argLeaves      map[string]Gate     // tasks an aggregate passes arguments
	filled         map[string][]filler // per check, the gates that run for it
}

func (p *planner) subproject(name string) {
	s := p.newSub(name)
	for _, c := range p.cfg.Checks {
		matched := p.taskGates(s, c)
		if p.leafGates(s, c) {
			matched = true
		}
		if !matched {
			p.missingGate(s, c)
		}
	}
	// An aggregate's reference to a task that doesn't exist has no slot.
	for _, l := range s.r.leaves {
		if l.slot == "" && !s.seen[l.id] {
			s.seen[l.id] = true
			p.add(l.gate)
		}
	}
	p.toolchainGates(s)
}

func (p *planner) newSub(name string) *subPlan {
	dir := filepath.Join(p.root, name)
	s := &subPlan{name: name, dir: dir, sfx: project.SubLabel(name), tasks: project.Tasks(p.cfg, dir),
		seen: map[string]bool{}, argLeaves: map[string]Gate{}, filled: map[string][]filler{}}
	// SKIP lines take the first present provider's stack (else its name),
	// so a package.json with no scripts still reports what it lacks.
	for _, pr := range project.Providers(p.cfg, dir) {
		if pr.Stack != "" {
			s.skipLabel = pr.Stack
			break
		}
		if s.skipLabel == "" {
			s.skipLabel = pr.Name
		}
	}
	s.r = roles(p.cfg, p.root, dir, s.sfx, s.tasks, p.subDirs)
	for _, l := range s.r.leaves {
		// A leaf a blocker skipped has no words to run the task with.
		if l.withArgs && l.gate.Skip == "" {
			s.argLeaves[l.id] = l.gate
		}
	}
	for slot, fs := range p.awayFilled[dir] {
		s.filled[slot] = slices.Clone(fs)
	}
	return s
}

func (s *subPlan) labelOf(i int) string {
	return fmt.Sprintf("%s: %s (%s)%s", cmp.Or(s.tasks[i].Stack, s.tasks[i].Provider), s.r.slots[i], s.tasks[i].Name, s.sfx)
}

// coverer is the task whose run covers task i: the one whose body runs it,
// or, when that one is disabled or unsafe, whatever covers that one; else
// -1.
func (s *subPlan) coverer(cfg *config.Config, i int) int {
	for j, n := s.r.covered[i], 0; j >= 0 && n < len(s.tasks); j, n = s.r.covered[j], n+1 {
		if !cfg.CheckDisabled(s.r.slots[j], s.labelOf(j)) && s.r.unsafe[j] == "" {
			return j
		}
	}
	return -1
}

func (s *subPlan) fill(slot, label string, tools []string) {
	s.filled[slot] = append(s.filled[slot], filler{label, tools})
}

// coveredBy is the label of a gate already filling slot with one of tools.
func (s *subPlan) coveredBy(slot string, tools []string) string {
	for _, f := range s.filled[slot] {
		if slices.ContainsFunc(f.tools, func(t string) bool { return slices.Contains(tools, t) }) {
			return f.label
		}
	}
	return ""
}

// taskGates plans each task filling check c; it reports whether any did.
func (p *planner) taskGates(s *subPlan, c config.Check) bool {
	cfg, matched := p.cfg, false
	for i, t := range s.tasks {
		if s.r.slots[i] != c.Name || (p.orchestrated[c.Name] && cmp.Or(t.Stack, t.Provider) == "js") {
			continue
		}
		matched = true
		s.seen[taskID(t, s.dir)] = true
		full := s.labelOf(i)
		switch by := s.coverer(cfg, i); {
		case by >= 0:
			p.add(Gate{Label: full, Skip: "covered by " + s.tasks[by].Name})
		case cfg.CheckDisabled(c.Name, full):
			p.add(Gate{Label: full, Skip: "disabled_checks"})
		case s.r.unsafe[i] != "":
			// It never fills the check, so a clean gate (a CI step) still runs.
			p.add(Gate{Label: full, Skip: unsafeSkip(s.r.unsafe[i])})
		default:
			g := Gate{Label: full, Dir: s.dir, Words: strings.Fields(t.Cmd), Scope: scopeFor(c, s.r.single[i])}
			if withArgs, ok := s.argLeaves[taskID(t, s.dir)]; ok {
				// An aggregate passes it arguments (npm run unit -- --coverage).
				g.Words = withArgs.Words
			}
			p.add(g)
			s.fill(c.Name, full, s.r.tools[i])
			for _, ag := range s.r.away[i] {
				if p.awayFilled[ag.dir] == nil {
					p.awayFilled[ag.dir] = map[string][]filler{}
				}
				p.awayFilled[ag.dir][ag.slot] = append(p.awayFilled[ag.dir][ag.slot], filler{full, []string{ag.tool}})
			}
		}
	}
	return matched
}

// leafGates plans check c's gates inside aggregates, then those only CI
// names: tasks CI runs, then tools it runs directly, which yield to a gate
// already running the same tool for the check. It reports whether any did.
func (p *planner) leafGates(s *subPlan, c config.Check) bool {
	matched := false
	for _, l := range append(slices.Clone(s.r.leaves), p.ci[s.name]...) {
		if l.slot != c.Name || s.seen[l.id] || (p.orchestrated[c.Name] && strings.HasPrefix(l.label, "js: ")) {
			continue
		}
		s.seen[l.id] = true
		by := ""
		if l.inline {
			by = s.coveredBy(c.Name, l.tools)
		}
		skip := ""
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
			skip = "covered by an earlier step running " + l.tools[0] + "; skipped `" + strings.Join(words, " ") + "`"
		case by != "":
			skip = "covered by " + by
		case l.gate.Skip == "" && p.cfg.CheckDisabled(c.Name, l.label):
			skip = "disabled_checks"
		}
		if skip != "" {
			l.gate = Gate{Label: l.label, Skip: skip, CI: l.gate.CI}
		}
		// A skipped leaf still reports the check, so no "no task" line.
		matched = true
		if l.gate.Skip == "" {
			s.fill(c.Name, l.label, l.tools)
		}
		p.add(l.gate)
	}
	return matched
}

// missingGate reports check c as having no task, unless the subproject has
// no task provider or an orchestrator runs the check.
func (p *planner) missingGate(s *subPlan, c config.Check) {
	if s.skipLabel == "" || (p.orchestrated[c.Name] && s.skipLabel == "js") {
		return
	}
	reason := "no " + c.Name + " task"
	if name, glob := excludedTask(c, s.tasks); name != "" {
		reason += "; " + name + " matches the exclude glob " + strconv.Quote(glob)
	}
	if p.cfg.CheckDisabled(c.Name, "") {
		reason = "disabled_checks"
	}
	p.add(Gate{Label: fmt.Sprintf("%s: %s%s", s.skipLabel, c.Name, s.sfx), Skip: reason})
}

func (p *planner) toolchainGates(s *subPlan) {
	for _, tc := range p.cfg.ToolchainChecks {
		if !p.cfg.HasStack(s.dir, tc.Stack) {
			continue
		}
		label := tc.Stack + ": " + tc.Name + s.sfx
		if skip := ToolchainSkip(p.cfg, tc, s.name, s.coveredBy); skip != "" {
			p.add(Gate{Label: label, Skip: skip})
			continue
		}
		run := tools.ResolveToolchain(p.cfg, tc, s.dir, p.root)
		if run.Words == nil {
			p.add(Gate{Label: label, Skip: run.Skip, Unrun: run.Project})
			continue
		}
		p.add(Gate{Label: label, Dir: s.dir, Words: run.Words, BinLine: run.BinLine(), Scope: toolchainScope(p.cfg, tc)})
	}
}
