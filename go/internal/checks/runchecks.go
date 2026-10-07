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
// orchestrator covers, whose JS tasks are then left to it; ci holds the CI
// steps by the subproject they run in.
type planner struct {
	cfg          *config.Config
	root         string
	gates        []Gate
	orchestrated map[string]bool
	ci           map[string][]ciStep
}

func (p *planner) add(g Gate) { p.gates = append(p.gates, g) }

// Gates is every check of every subproject of root, or only those named in
// only ("." is the root), in the order run-checks runs them.
func Gates(cfg *config.Config, root string, only []string) []Gate {
	p := &planner{cfg: cfg, root: root, orchestrated: map[string]bool{}}
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
	subs := project.Subprojects(cfg, root)
	p.ci = stepsBySubproject(ciSteps(cfg, root), subs)
	for _, sub := range subs {
		if inScope(sub) {
			p.subproject(sub)
		}
	}
	return p.gates
}

// stepsBySubproject files each CI step under the deepest subproject its
// directory is in.
func stepsBySubproject(steps []ciStep, subs []string) map[string][]ciStep {
	out := map[string][]ciStep{}
	for _, s := range steps {
		best := "."
		for _, sub := range subs {
			if sub != "." && (s.dir == sub || strings.HasPrefix(s.dir, sub+"/")) && len(sub) > len(best) {
				best = sub
			}
		}
		out[best] = append(out[best], s)
	}
	return out
}

// RunAll runs Gates' checks and returns the failure count. Output contract,
// parsed by callers: one PASS, FAIL, or SKIP line per check, labeled as Gate
// says; a check whose binary the kit resolved has an indented "  bin:
// <words> (<source>)" line right under it; then a blank line and "checks: N
// passed, N failed, N skipped".
func RunAll(cfg *config.Config, root string, only []string, out io.Writer) int {
	pass, fail, skip := 0, 0, 0
	var ch *changes
	for _, g := range Gates(cfg, root, only) {
		switch {
		case g.Skip != "":
			fmt.Fprintf(out, "SKIP %s (%s)\n", g.Label, g.Skip)
			skip++
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
			fmt.Fprintf(out, "  dir: %s\n", rel(root, g.Dir))
		}
		switch {
		case g.Scope != nil && g.Scope.Advisory:
			fmt.Fprint(out, "  scope: advisory; its findings never fail run-checks\n")
		case g.Scope != nil:
			fmt.Fprint(out, "  scope: only findings in files changed since the git base fail it\n")
		}
		fmt.Fprint(out, g.extra())
	}
}

// run runs g and reports it, with its bin line under the verdict; a failure
// then prints the first 30 lines of the command's output.
func run(g Gate, w io.Writer) bool {
	var out bytes.Buffer
	cmd := exec.Command(g.Words[0], g.Words[1:]...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = g.Dir, &out, &out
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

	r := roles(cfg, p.root, dir, sfx, tasks)
	ciStack := skipLabel
	for _, name := range cfg.StackOrder {
		if ciStack == "" && name != "monorepo" && cfg.HasStack(dir, name) {
			ciStack = name
		}
	}
	ci := p.ciLeaves(sub, sfx, ciStack)
	seen := map[string]bool{}
	argLeaves := map[string]Gate{}
	for _, l := range r.leaves {
		if l.withArgs {
			argLeaves[l.id] = l.gate
		}
	}
	// filled is, per check, the first gate that runs for it: a CI tool line
	// or a toolchain check standing in for that check is then covered.
	filled := map[string]string{}
	fill := func(slot, label string) {
		if filled[slot] == "" {
			filled[slot] = label
		}
	}
	for _, c := range cfg.Checks {
		matched := false
		for i, t := range tasks {
			label := t.Stack
			if label == "" {
				label = t.Provider
			}
			if r.slots[i] != c.Name || (p.orchestrated[c.Name] && label == "js") {
				continue
			}
			matched = true
			seen[taskID(t, dir)] = true
			full := fmt.Sprintf("%s: %s (%s)%s", label, c.Name, t.Name, sfx)
			switch {
			case r.covered[i] != "":
				p.add(Gate{Label: full, Skip: "covered by " + r.covered[i]})
			case cfg.CheckDisabled(c.Name, full):
				p.add(Gate{Label: full, Skip: "disabled_checks"})
			default:
				g := Gate{Label: full, Dir: dir, Words: strings.Fields(t.Cmd), Scope: scopeFor(c, r.single[i])}
				if withArgs, ok := argLeaves[taskID(t, dir)]; ok {
					// An aggregate passes it arguments (npm run unit -- --coverage).
					g.Words = withArgs.Words
				}
				p.add(g)
				fill(c.Name, full)
			}
		}
		// Gates inside aggregates, then those only CI names: tasks CI runs,
		// then tools it runs directly, which yield to anything that filled
		// the check already.
		for _, l := range append(slices.Clone(r.leaves), ci...) {
			if l.slot != c.Name || seen[l.id] || (p.orchestrated[c.Name] && strings.HasPrefix(l.label, "js: ")) {
				continue
			}
			seen[l.id] = true
			switch {
			case l.gate.CI != "" && l.inline && filled[c.Name] != "":
				l.gate = Gate{Label: l.label, Skip: "covered by " + filled[c.Name], CI: l.gate.CI}
			case l.gate.Skip == "" && cfg.CheckDisabled(c.Name, l.label):
				l.gate = Gate{Label: l.label, Skip: "disabled_checks", CI: l.gate.CI}
			}
			if l.gate.Skip == "" {
				matched = true
				fill(c.Name, l.label)
			}
			p.add(l.gate)
		}
		if !matched && skipLabel != "" && !(p.orchestrated[c.Name] && skipLabel == "js") {
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
		if tc.Slot != "" && filled[tc.Slot] != "" {
			p.add(Gate{Label: label, Skip: "covered by " + filled[tc.Slot]})
			continue
		}
		run := tools.ResolveToolchain(cfg, tc, dir, p.root)
		if run.Words == nil {
			p.add(Gate{Label: label, Skip: run.Skip})
			continue
		}
		p.add(Gate{Label: label, Dir: dir, Words: run.Words, BinLine: run.BinLine()})
	}
}
