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

	"github.com/ku5ic/claude-kit/go/internal/classify"
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
	Skip    string
}

// planner accumulates one plan; orchestrated holds the checks an
// orchestrator covers, whose JS tasks are then left to it.
type planner struct {
	cfg          *config.Config
	root         string
	gates        []Gate
	orchestrated map[string]bool
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
	for _, sub := range project.Subprojects(cfg, root) {
		if inScope(sub) {
			p.subproject(sub)
		}
	}
	return p.gates
}

// RunAll runs Gates' checks and returns the failure count. Output contract,
// parsed by callers: one PASS, FAIL, or SKIP line per check, labeled as Gate
// says; a check whose binary the kit resolved has an indented "  bin:
// <words> (<source>)" line right under it; then a blank line and "checks: N
// passed, N failed, N skipped".
func RunAll(cfg *config.Config, root string, only []string, out io.Writer) int {
	pass, fail, skip := 0, 0, 0
	for _, g := range Gates(cfg, root, only) {
		switch {
		case g.Skip != "":
			fmt.Fprintf(out, "SKIP %s (%s)\n", g.Label, g.Skip)
			skip++
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
		fmt.Fprintf(out, "RUN %s\n  cmd: %s\n%s", g.Label, tools.ShellJoin(g.Words), g.BinLine)
	}
}

// run runs g and reports it, with its bin line under the verdict; a failure
// then prints the first 30 lines of the command's output.
func run(g Gate, w io.Writer) bool {
	var out bytes.Buffer
	cmd := exec.Command(g.Words[0], g.Words[1:]...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = g.Dir, &out, &out
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(w, "FAIL %s (%s)\n%s", g.Label, strings.Join(g.Words, " "), g.BinLine)
		if _, isExit := err.(*exec.ExitError); !isExit {
			fmt.Fprintf(&out, "%s: %v\n", g.Words[0], err)
		}
		lines := strings.SplitAfter(out.String(), "\n")
		fmt.Fprint(w, strings.Join(lines[:min(len(lines), 30)], ""))
		return false
	}
	fmt.Fprintf(w, "PASS %s\n%s", g.Label, g.BinLine)
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

// bodySlots is, per task, the check its body is when its name matches no
// check's globs and its body is a single gate (verify-style: eslint .);
// "" otherwise. Such a task still runs as itself, through its provider.
func bodySlots(cfg *config.Config, dir string, tasks []project.Task) []string {
	lookup := func(provider, rel, name string) bool {
		for _, t := range project.Tasks(cfg, filepath.Join(dir, rel)) {
			if t.Provider == provider && t.Name == name {
				return true
			}
		}
		return false
	}
	slots := make([]string, len(tasks))
	for i, t := range tasks {
		if t.Body == "" || slices.ContainsFunc(cfg.Checks, func(c config.Check) bool { return matchesCheck(c, t.Name) }) {
			continue
		}
		if gate, ok := classify.Body(cfg, t.Body, lookup).SingleGate(); ok {
			slots[i] = gate.Slot
		}
	}
	return slots
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

	bodySlots := bodySlots(cfg, dir, tasks)
	for _, c := range cfg.Checks {
		matched := false
		for i, t := range tasks {
			label := t.Stack
			if label == "" {
				label = t.Provider
			}
			counts := matchesCheck(c, t.Name) || (bodySlots[i] == c.Name && !excluded(c, t.Name))
			if !counts || (p.orchestrated[c.Name] && label == "js") {
				continue
			}
			matched = true
			full := fmt.Sprintf("%s: %s (%s)%s", label, c.Name, t.Name, sfx)
			if cfg.CheckDisabled(c.Name, full) {
				p.add(Gate{Label: full, Skip: "disabled_checks"})
				continue
			}
			p.add(Gate{Label: full, Dir: dir, Words: strings.Fields(t.Cmd)})
		}
		if !matched && skipLabel != "" && !(p.orchestrated[c.Name] && skipLabel == "js") {
			reason := "no " + c.Name + " task"
			if cfg.CheckDisabled(c.Name, "") {
				reason = "disabled_checks"
			}
			p.add(Gate{Label: fmt.Sprintf("%s: %s%s", skipLabel, c.Name, sfx), Skip: reason})
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
		run := tools.ResolveToolchain(cfg, tc, dir, p.root)
		if run.Words == nil {
			p.add(Gate{Label: label, Skip: run.Skip})
			continue
		}
		p.add(Gate{Label: label, Dir: dir, Words: run.Words, BinLine: run.BinLine()})
	}
}
