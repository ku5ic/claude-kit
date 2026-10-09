package checks

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/gitbase"
	"github.com/ku5ic/claude-kit/go/internal/tools"
)

// Scope is how a whole-program check (dead code) is judged: only findings
// on lines changed since the git base fail it. Advisory checks, whose
// output can't be mapped to files, never fail.
type Scope struct {
	Findings *tools.Findings
	Advisory bool
}

// scopeFor is the scope of a gate in check c, given the tool command it
// runs (nil when the check isn't scoped).
func scopeFor(c config.Check, cmd *classify.Command) *Scope {
	if c.Scope != "changed" {
		return nil
	}
	if cmd == nil || cmd.Pattern == nil || cmd.Pattern.Advisory || cmd.Pattern.Findings == "" {
		return &Scope{Advisory: true}
	}
	p := cmd.Pattern
	re, err := regexp.Compile(p.Findings)
	if err != nil || slices.ContainsFunc(cmd.Words, func(w string) bool {
		return slices.ContainsFunc(p.Unmapped, func(f string) bool { return w == f || strings.HasPrefix(w, f+"=") })
	}) {
		return &Scope{Advisory: true}
	}
	return &Scope{Findings: &tools.Findings{Item: re}}
}

func checkNamed(cfg *config.Config, slot string) config.Check {
	for _, c := range cfg.Checks {
		if c.Name == slot {
			return c
		}
	}
	return config.Check{}
}

// changes is the git base and the files changed since its merge-base with
// HEAD: committed, staged, unstaged, and untracked, deletions left out.
type changes struct {
	root, base, mergeBase string
	files                 map[string]bool // absolute paths
	ok                    bool
}

func changedSince(root string) changes {
	base, ok := gitbase.ResolveIn(root, "")
	if !ok {
		return changes{}
	}
	run := func(args ...string) []string {
		lines, err := git.Lines(root, append([]string{"-c", "core.quotePath=false"}, args...)...)
		if err != nil {
			return nil
		}
		return lines
	}
	mb := run("merge-base", base, "HEAD")
	if len(mb) != 1 {
		return changes{}
	}
	c := changes{root: root, base: base, mergeBase: mb[0], files: map[string]bool{}, ok: true}
	for _, f := range append(run("diff", "--name-only", "--diff-filter=d", mb[0]), run("ls-files", "--others", "--exclude-standard")...) {
		c.files[filepath.Join(root, f)] = true
	}
	return c
}

// runScoped runs a scoped gate. With no git base it skips; with nothing
// changed it passes without running. Otherwise the findings decide, not
// the exit code (vulture exits 3, deadcode 0, with findings alike): any in
// a changed file fails it; a non-zero exit with none parsed is a tool
// error. Only a finding on a changed line counts, so touching a file doesn't
// inherit its old dead code. It returns the verdict: "pass", "fail", or
// "skip".
func runScoped(g Gate, ch changes, w io.Writer) string {
	if !ch.ok {
		fmt.Fprint(w, skipLine(g.Label, "no git base"))
		return "skip"
	}
	if len(ch.files) == 0 {
		fmt.Fprintf(w, "PASS %s (nothing changed since %s)\n", g.Label, ch.base)
		return "pass"
	}
	out, err := capture(g)
	var found []tools.Finding
	if g.Scope.Findings != nil {
		found, _ = g.Scope.Findings.Parse(out)
	}
	if g.Scope.Advisory {
		what := "output not mapped to files"
		if g.Scope.Findings != nil {
			what = fmt.Sprintf("%d finding%s", len(found), plural(len(found)))
		}
		fmt.Fprintf(w, "PASS %s (advisory: %s)\n%s", g.Label, what, g.extra())
		return "pass"
	}
	paths := make([]string, len(found))
	var touched []string
	for i, f := range found {
		paths[i] = absUnder(g.Dir, f.File)
		if ch.files[paths[i]] {
			touched = append(touched, paths[i])
		}
	}
	var lines map[string]map[int]bool
	if len(touched) > 0 {
		// Each file once: thousands of findings in one file mustn't swell git's argv.
		slices.Sort(touched)
		lines = changedLines(ch.root, ch.mergeBase, slices.Compact(touched))
	}
	var blocking []string
	for i, f := range found {
		if ch.files[paths[i]] && onChangedLine(lines, paths[i], f.Line) {
			blocking = append(blocking, f.Text)
		}
	}
	switch {
	case len(blocking) > 0:
		fmt.Fprint(w, failLine(g)+strings.Join(head(blocking), "\n")+"\n")
		return "fail"
	case err != nil && len(found) == 0:
		fmt.Fprint(w, failLine(g)+strings.Join(head(strings.SplitAfter(out, "\n")), ""))
		return "fail"
	case len(found) > 0:
		fmt.Fprintf(w, "PASS %s (%d finding%s on unchanged lines)\n%s", g.Label, len(found), plural(len(found)), g.extra())
	default:
		fmt.Fprintf(w, "PASS %s\n%s", g.Label, g.extra())
	}
	return "pass"
}
