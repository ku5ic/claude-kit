package bashguard

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/pkgcheck"
)

// Package managers: global installs are kit policy (global_installs); the
// rest comes from the verified gap-fill facts of the lockfile directories a
// command runs under, so a cold cache says nothing.

// globalInstall blocks a command global_installs matches. An unreadable
// pattern blocks too: this is a guard. Its reason names the pattern and its
// file, as kit config --check would be held as well.
func (c *command) globalInstall() error {
	patterns, bad := c.st.cfg.InstallPatterns(c.st.h.Paths)
	if len(bad) > 0 {
		lines := make([]string, len(bad))
		for i, w := range bad {
			lines[i] = w.String()
		}
		return c.block(strings.Join(lines, "\n")+"\nfix it with the Edit tool", "global-installs-invalid")
	}
	if slices.ContainsFunc(patterns, func(re *regexp.Regexp) bool { return re.MatchString(c.text) }) {
		return c.block("global package install. Use a project-local install or asdf shim.", "pkg-global-install")
	}
	return nil
}

// packageManager holds a command to the facts of the lockfile directories
// it runs under, nearest first, up to the first whose manager or rivals
// name it: a rival blocks, and the manager's add verb with a package
// operand asks (skills/kit-rules/reference/workflow.md section 2).
func (c *command) packageManager() error {
	// --version and -v never touch project files.
	if rest := strings.TrimSpace(c.rest); rest == "--version" || rest == "-v" {
		return nil
	}
	// Found by walking up for .git, not git rev-parse: this runs for every
	// command the switch doesn't name, and an exec costs more than the check.
	dotGit := fsx.FindUp(c.st.cwd, "/", ".git")
	if dotGit == "" {
		return nil
	}
	root := fsx.PhysicalPath(filepath.Dir(dotGit))
	facts, ok := c.st.managers[root]
	if !ok {
		facts = gapfill.Managers(c.st.cfg, root, c.st.h.Paths.CacheDir())
		c.st.managers[root] = facts
	}
	var dirFlags []string
	for _, m := range facts {
		if m.Manager == c.name {
			dirFlags = append(dirFlags, m.DirFlags...)
		}
	}
	dir := c.pmDir(dirFlags)
	for _, m := range gapfill.Nearest(root, dir, facts) {
		switch {
		case m.Manager == c.name:
			return c.dependencyAdd(m.AddVerbs, dir, root)
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

// dependencyAdd asks when one of verbs ("pnpm add", or "add") has a
// package operand after it, and blocks when its registry says one of them
// doesn't exist, asking it from dir inside the repository at root.
func (c *command) dependencyAdd(verbs []string, dir, root string) error {
	values := c.values()
	for _, verb := range verbs {
		words := strings.Fields(verb)
		if len(words) > 1 && words[0] == c.name {
			words = words[1:]
		}
		for i := range values {
			after := i + len(words)
			if after > len(values) || !slices.Equal(values[i:after], words) {
				continue
			}
			if len(nonOptions(values[after:])) == 0 {
				continue
			}
			valueFlags, registryFlags := c.st.cfg.PackageLookups.Flags(c.name)
			if !c.st.h.RuleDisabled("pkg-missing") && !hasFlag(values, registryFlags) {
				lookups := pkgcheck.NewLookups(c.st.cfg.PackageLookups, dir, root)
				// A flag's value isn't looked up, but still counts above: a
				// flag listed as taking a value may not take one (pnpm -w).
				for _, p := range addOperands(values[after:], valueFlags) {
					if lookups.Missing(c.name, p) {
						return c.block(p+" isn't on its registry; check the name before adding it", "pkg-missing")
					}
				}
			}
			c.st.ask("this adds a dependency; skills/kit-rules/reference/workflow.md section 2 asks before installing: confirm the package")
			return nil
		}
	}
	return nil
}

// addOperands is the operands of words after an add verb, minus the word
// after each of valueFlags, which is that flag's value.
func addOperands(words, valueFlags []string) []string {
	var kept []string
	for i := 0; i < len(words); i++ {
		switch {
		case words[i] == "--":
			return nonOptions(append(kept, words[i:]...))
		case slices.Contains(valueFlags, words[i]):
			i++
		default:
			kept = append(kept, words[i])
		}
	}
	return nonOptions(kept)
}

// hasFlag is true when words hold one of flags, alone or as flag=value.
func hasFlag(words, flags []string) bool {
	return slices.ContainsFunc(words, func(w string) bool {
		name, _, _ := strings.Cut(w, "=")
		return slices.Contains(flags, name)
	})
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
