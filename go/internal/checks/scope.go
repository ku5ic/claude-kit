package checks

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/gitbase"
	"github.com/ku5ic/claude-kit/go/internal/tools"
)

// Scope is how a whole-program check (dead code) is judged: only findings
// in files changed since the git base fail it. Advisory checks, whose
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
	base  string
	files map[string]bool // absolute paths
	ok    bool
}

func changedSince(root string) changes {
	base, ok := gitbase.ResolveIn(root, "")
	if !ok {
		return changes{}
	}
	git := func(args ...string) []string {
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "core.quotePath=false"}, args...)...)
		out, err := cmd.Output()
		if err != nil {
			return nil
		}
		return strings.Fields(string(out))
	}
	mb := git("merge-base", base, "HEAD")
	if len(mb) != 1 {
		return changes{}
	}
	c := changes{base: base, files: map[string]bool{}, ok: true}
	for _, f := range append(git("diff", "--name-only", "--diff-filter=d", mb[0]), git("ls-files", "--others", "--exclude-standard")...) {
		c.files[filepath.Join(root, f)] = true
	}
	return c
}

// runScoped runs a scoped gate. With no git base it skips; with nothing
// changed it passes without running. Otherwise the findings decide, not
// the exit code (vulture exits 3, deadcode 0, with findings alike): any in
// a changed file fails it; a non-zero exit with none parsed is a tool
// error. It returns the verdict: "pass", "fail", or "skip".
func runScoped(g Gate, ch changes, w io.Writer) string {
	if !ch.ok {
		fmt.Fprintf(w, "SKIP %s (no git base)\n", g.Label)
		return "skip"
	}
	if len(ch.files) == 0 {
		fmt.Fprintf(w, "PASS %s (nothing changed since %s)\n", g.Label, ch.base)
		return "pass"
	}
	var out bytes.Buffer
	cmd := exec.Command(g.Words[0], g.Words[1:]...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = g.Dir, &out, &out
	err := cmd.Run()
	if _, isExit := err.(*exec.ExitError); err != nil && !isExit {
		fmt.Fprintf(&out, "%s: %v\n", g.Words[0], err)
	}
	var found []tools.Finding
	if g.Scope.Findings != nil {
		found, _ = g.Scope.Findings.Parse(out.String())
	}
	if g.Scope.Advisory {
		what := "output not mapped to files"
		if g.Scope.Findings != nil {
			what = fmt.Sprintf("%d finding%s", len(found), plural(len(found)))
		}
		fmt.Fprintf(w, "PASS %s (advisory: %s)\n%s", g.Label, what, g.extra())
		return "pass"
	}
	var blocking []string
	for _, f := range found {
		path := f.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(g.Dir, path)
		}
		if ch.files[filepath.Clean(path)] {
			blocking = append(blocking, f.Text)
		}
	}
	switch {
	case len(blocking) > 0:
		fmt.Fprintf(w, "FAIL %s (%s)\n%s%s\n", g.Label, strings.Join(g.Words, " "), g.extra(), strings.Join(blocking[:min(len(blocking), 30)], "\n"))
		return "fail"
	case err != nil && len(found) == 0:
		lines := strings.SplitAfter(out.String(), "\n")
		fmt.Fprintf(w, "FAIL %s (%s)\n%s%s", g.Label, strings.Join(g.Words, " "), g.extra(), strings.Join(lines[:min(len(lines), 30)], ""))
		return "fail"
	case len(found) > 0:
		fmt.Fprintf(w, "PASS %s (%d finding%s in unchanged files)\n%s", g.Label, len(found), plural(len(found)), g.extra())
	default:
		fmt.Fprintf(w, "PASS %s\n%s", g.Label, g.extra())
	}
	return "pass"
}
