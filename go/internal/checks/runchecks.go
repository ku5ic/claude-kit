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

// Runner accumulates one run's results. Output contract, parsed by callers:
// one PASS, FAIL, or SKIP line per check, labeled
// "<stack>: <check> (<task>) [<subproject>]" (the root has no bracket part);
// a check whose binary the kit resolved has an indented "  bin: <words>
// (<source>)" line right under it; then a blank line and "checks: N passed,
// N failed, N skipped".
type Runner struct {
	Out                io.Writer
	Pass, Fail, Skip   int
	orchestratedChecks map[string]bool
}

// exec runs words in dir and reports it, with binLine (when not empty)
// under the verdict; a failure then prints the first 30 lines of the
// command's output.
func (r *Runner) exec(label, dir string, words []string, binLine string) {
	var out bytes.Buffer
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, &out, &out
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(r.Out, "FAIL %s (%s)\n%s", label, strings.Join(words, " "), binLine)
		if _, isExit := err.(*exec.ExitError); !isExit {
			fmt.Fprintf(&out, "%s: %v\n", words[0], err)
		}
		lines := strings.SplitAfter(out.String(), "\n")
		fmt.Fprint(r.Out, strings.Join(lines[:min(len(lines), 30)], ""))
		r.Fail++
		return
	}
	fmt.Fprintf(r.Out, "PASS %s\n%s", label, binLine)
	r.Pass++
}

func (r *Runner) skip(msg string) {
	fmt.Fprintf(r.Out, "SKIP %s\n", msg)
	r.Skip++
}

// matchesCheck is true when task counts as check c: it matches one of the
// check's task globs and none of its exclude globs.
func matchesCheck(c config.Check, task string) bool {
	match := func(globs []string) bool {
		return slices.ContainsFunc(globs, func(g string) bool { return guard.Glob(g, task) })
	}
	return match(c.Tasks) && !match(c.Exclude)
}

// RunAll runs the checks of every subproject of root, or only those named
// in only ("." is the root), and returns the failure count.
func RunAll(cfg *config.Config, root string, only []string, out io.Writer) int {
	r := &Runner{Out: out, orchestratedChecks: map[string]bool{}}
	subs := project.Subprojects(cfg, root)
	inScope := func(sub string) bool { return len(only) == 0 || slices.Contains(only, sub) }

	// The orchestrator only when a JS subproject is in scope: a run scoped to
	// a Python service has nothing for turbo or nx to do.
	wantsJS := len(only) == 0
	for _, sub := range only {
		for _, p := range project.Providers(cfg, dirOf(root, sub)) {
			if p.Stack == "js" {
				wantsJS = true
			}
		}
	}
	if wantsJS {
		r.orchestrate(cfg, root)
	}
	for _, sub := range subs {
		if inScope(sub) {
			r.subproject(cfg, root, sub)
		}
	}
	fmt.Fprintf(out, "\nchecks: %d passed, %d failed, %d skipped\n", r.Pass, r.Fail, r.Skip)
	return r.Fail
}

func dirOf(root, sub string) string {
	if sub == "." {
		return root
	}
	return filepath.Join(root, sub)
}

// orchestrate runs each check the first usable orchestrator declares a
// task for, once, at the root: its signal file is there and its binary is in
// the root's node_modules/.bin (never npx). Those checks' JS provider tasks
// are then skipped per package.
func (r *Runner) orchestrate(cfg *config.Config, root string) {
	for _, o := range cfg.Orchestrators {
		signal := filepath.Join(root, o.Signal)
		bin := filepath.Join(root, "node_modules", ".bin", o.Name)
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
		for _, c := range cfg.Checks {
			for _, task := range tasks {
				if !matchesCheck(c, task) {
					continue
				}
				label := fmt.Sprintf("js: %s (%s affected: %s)", c.Name, o.Name, task)
				if cfg.CheckDisabled(c.Name, label) {
					r.skip(label + " (disabled_checks)")
					continue
				}
				r.orchestratedChecks[c.Name] = true
				words := tools.Fill(strings.Fields(strings.ReplaceAll(o.Run, "{task}", task)), "{bin}", []string{bin})
				origin := tools.Resolution{Words: []string{bin}, Source: tools.SourceLocal}
				r.exec(label, root, words, origin.BinLine())
			}
		}
		return
	}
}

func (r *Runner) subproject(cfg *config.Config, root, sub string) {
	dir, sfx := dirOf(root, sub), ""
	if sub != "." {
		sfx = " [" + sub + "]"
	}
	tasks := project.Tasks(cfg, dir)

	// SKIP lines take the first present provider's stack (else its name),
	// so a package.json with no scripts still reports what it lacks.
	skipLabel := ""
	for _, p := range project.Providers(cfg, dir) {
		if p.Stack != "" {
			skipLabel = p.Stack
			break
		}
		if skipLabel == "" {
			skipLabel = p.Name
		}
	}

	for _, c := range cfg.Checks {
		matched := false
		for _, t := range tasks {
			label := t.Stack
			if label == "" {
				label = t.Provider
			}
			if !matchesCheck(c, t.Name) || (r.orchestratedChecks[c.Name] && label == "js") {
				continue
			}
			matched = true
			full := fmt.Sprintf("%s: %s (%s)%s", label, c.Name, t.Name, sfx)
			if cfg.CheckDisabled(c.Name, full) {
				r.skip(full + " (disabled_checks)")
				continue
			}
			r.exec(full, dir, strings.Fields(t.Cmd), "")
		}
		if !matched && skipLabel != "" && !(r.orchestratedChecks[c.Name] && skipLabel == "js") {
			reason := "no " + c.Name + " task"
			if cfg.CheckDisabled(c.Name, "") {
				reason = "disabled_checks"
			}
			r.skip(fmt.Sprintf("%s: %s%s (%s)", skipLabel, c.Name, sfx, reason))
		}
	}

	for _, tc := range cfg.ToolchainChecks {
		if !cfg.HasStack(dir, tc.Stack) {
			continue
		}
		label := tc.Stack + ": " + tc.Name + sfx
		if !cfg.ToolchainEnabled(tc) {
			r.skip(label + " (disabled_toolchain_checks)")
			continue
		}
		run := tools.ResolveToolchain(cfg, tc, dir, root)
		if run.Words == nil {
			r.skip(label + " (" + run.Skip + ")")
			continue
		}
		r.exec(label, dir, run.Words, run.BinLine())
	}
}
