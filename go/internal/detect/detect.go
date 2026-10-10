// Package detect writes the compact stack report `kit detect-stack` prints and
// the <repo-context> block carries:
//
//	root: <root>
//	<stack>: yes (<extras>) at <subproject>, ...
//	package-manager [<lockfile dir>]: <manager> (<lockfile>)
//
// Stacks and extras follow kit.yml's document order. Package managers are
// gap-fill's verified facts, read from its cache: none until a run has
// answered. Nothing is printed when no subproject holds any stack.
package detect

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Report is the full text for root, "" when no stack is found.
func Report(cfg *config.Config, root, cacheDir string) string {
	subs := project.Subprojects(cfg, root)
	out := []string{"root: " + root}
	for _, stack := range cfg.StackOrder {
		if line := stackLine(cfg, root, subs, stack); line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 1 {
		return ""
	}
	for _, m := range gapfill.Managers(cfg, root, cacheDir) {
		out = append(out, "package-manager"+project.SubLabel(filepath.Clean(m.Dir))+": "+m.Manager+" ("+m.Cites+")")
	}
	return strings.Join(out, "\n") + "\n"
}

// stackLine is stack's report line, "<stack>: yes (extras) at subs"; ""
// when no subproject holds it.
func stackLine(cfg *config.Config, root string, subs []string, stack string) string {
	var locs, extras []string
	for _, sub := range subs {
		dir := filepath.Join(root, sub)
		if !cfg.HasStack(dir, stack) {
			continue
		}
		locs = append(locs, sub)
		for _, extra := range cfg.Stacks[stack].Extras {
			if matchExtra(extra, dir) && !slices.Contains(extras, extra.Name) {
				extras = append(extras, extra.Name)
			}
		}
	}
	if len(locs) == 0 {
		return ""
	}
	line := stack + ": yes"
	if len(extras) > 0 {
		line += " (" + strings.Join(extras, ",") + ")"
	}
	if len(locs) != 1 || locs[0] != "." {
		line += " at " + strings.Join(locs, ", ")
	}
	return line
}

// matchExtra is true when the extra's own rule or any of its any_of rules
// matches.
func matchExtra(extra config.Extra, dir string) bool {
	return matchRule(extra.Rule, dir) || slices.ContainsFunc(extra.AnyOf, func(r config.Rule) bool { return matchRule(r, dir) })
}

// matchRule evaluates the rule's first kind present, in the order dep,
// pydep, file, grep.
func matchRule(rule config.Rule, dir string) bool {
	switch {
	case rule.Dep != "":
		_, ok := sources.JSSpecs(dir)[rule.Dep]
		return ok
	case rule.PyDep != "":
		return sources.PythonDeps(dir)[rule.PyDep]
	case rule.File != "":
		return fsx.IsFile(filepath.Join(dir, rule.File))
	case rule.Grep != "":
		return grepAny(dir, rule.Grep, rule.In)
	}
	return false
}

// grepAny is `grep -qE pattern` over each file: ^ and $ anchor per line.
func grepAny(dir, pattern string, files []string) bool {
	re, err := regexp.Compile("(?m)" + pattern)
	if err != nil {
		return false
	}
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil && re.Match(data) {
			return true
		}
	}
	return false
}
