package bashguard

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/git"
)

// Package managers: global installs are kit policy (global_installs); the
// rest comes from the verified gap-fill facts of the lockfile directories a
// command runs under, so a cold cache says nothing.

// globalInstall blocks a command global_installs matches. An unreadable
// pattern blocks too: this is a guard.
func (c *command) globalInstall() error {
	for _, pattern := range c.st.cfg.GlobalInstalls {
		if re, err := regexp.Compile(pattern); err != nil || re.MatchString(c.text) {
			return c.block("global package install. Use a project-local install or asdf shim.", "pkg-global-install")
		}
	}
	return nil
}

// packageManager holds a command to the facts of the lockfile directories
// it runs under, nearest first, up to the first whose manager or rivals
// name it: a rival blocks, and the manager's add verb with a package
// operand asks (rules/workflow.md section 2).
func (c *command) packageManager() error {
	// --version and -v never touch project files.
	if rest := strings.TrimSpace(c.rest); rest == "--version" || rest == "-v" {
		return nil
	}
	root := git.Toplevel(c.st.cwd)
	if root == "" {
		return nil
	}
	facts := gapfill.Managers(c.st.cfg, root, c.st.h.Paths.CacheDir())
	var dirFlags []string
	for _, m := range facts {
		if m.Manager == c.name {
			dirFlags = append(dirFlags, m.DirFlags...)
		}
	}
	for _, m := range nearest(root, c.pmDir(dirFlags), facts) {
		switch {
		case m.Manager == c.name:
			c.dependencyAdd(m.AddVerbs)
			return nil
		case slices.Contains(m.Rivals, c.name):
			return c.block(rival(m, c.name), "pm-mismatch")
		}
	}
	return nil
}

// pmDir is the directory a package manager works in: its own directory
// flag, else the segment's cwd. Only a long flag takes =value.
func (c *command) pmDir(dirFlags []string) string {
	dir, wantDir := c.st.cwd, false
	for _, w := range c.values() {
		if wantDir {
			dir, wantDir = resolveDir(c.st.home, c.st.cwd, w), false
			continue
		}
		flag, value, hasValue := strings.Cut(w, "=")
		switch {
		case slices.Contains(dirFlags, w):
			wantDir = true
		case hasValue && slices.Contains(dirFlags, flag) && strings.HasPrefix(flag, "--"):
			dir = resolveDir(c.st.home, c.st.cwd, value)
		}
	}
	return dir
}

// nearest are the facts of the lockfile directories holding dir, the
// deepest first.
func nearest(root, dir string, facts []gapfill.Manager) []gapfill.Manager {
	var out []gapfill.Manager
	for _, m := range facts {
		if d := filepath.Join(root, m.Dir); dir == d || strings.HasPrefix(dir, d+"/") {
			out = append(out, m)
		}
	}
	// Every one holds dir, so the longer path is the deeper.
	slices.SortStableFunc(out, func(a, b gapfill.Manager) int {
		return len(filepath.Join(root, b.Dir)) - len(filepath.Join(root, a.Dir))
	})
	return out
}

// dependencyAdd asks when one of verbs ("pnpm add", or "add") has a
// package operand after it.
func (c *command) dependencyAdd(verbs []string) {
	values := c.values()
	for _, verb := range verbs {
		words := strings.Fields(verb)
		if len(words) > 1 && words[0] == c.name {
			words = words[1:]
		}
		for i := range values {
			after := i + len(words)
			if after <= len(values) && slices.Equal(values[i:after], words) && slices.ContainsFunc(values[after:], func(v string) bool { return !strings.HasPrefix(v, "-") }) {
				c.st.ask("this adds a dependency; rules/workflow.md section 2 asks before installing: confirm the package")
				return
			}
		}
	}
}

// rival is why a rival of m's manager blocks, with m's facts for the rerun.
func rival(m gapfill.Manager, name string) string {
	where := "this repo"
	if d := filepath.Clean(m.Dir); d != "." {
		where = d
	}
	reason := fmt.Sprintf("%s uses %s (%s), not %s; rerun it with %s", where, m.Manager, m.Cites, name, m.Manager)
	var how []string
	if len(m.AddVerbs) > 0 {
		how = append(how, "add a dependency: "+m.AddVerbs[0])
	}
	if m.DLX != "" {
		how = append(how, "run a package: "+m.DLX)
	}
	if len(m.DirFlags) > 0 {
		how = append(how, "another directory: "+strings.Join(m.DirFlags, ", "))
	}
	if len(how) > 0 {
		reason += " (" + strings.Join(how, "; ") + ")"
	}
	return reason
}
