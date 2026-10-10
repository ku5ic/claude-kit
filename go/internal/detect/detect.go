// Package detect writes the compact stack report `kit detect-stack` prints and
// the <repo-context> block carries:
//
//	root: <root>
//	<stack>: yes (<extras>) [<package manager>] at <subproject>, ...
//	versions [<subproject>]: <name> <version> (<label>), ...
//	node: <version>
//
// Stacks, extras, and versions follow kit.yml's document order. Nothing is
// printed when no subproject holds any stack.
package detect

import (
	"cmp"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/tools"
)

// Report is the full text for root, "" when no stack is found.
func Report(cfg *config.Config, root string) string {
	subs := project.Subprojects(cfg, root)
	out := []string{"root: " + root}
	jsLoc := ""
	for _, stack := range cfg.StackOrder {
		if line, first := stackLine(cfg, root, subs, stack); line != "" {
			out = append(out, line)
			if stack == "js" {
				jsLoc = first
			}
		}
	}
	if len(out) == 1 {
		return ""
	}
	for _, sub := range subs {
		if parts := versions(cfg, root, filepath.Join(root, sub)); len(parts) > 0 {
			out = append(out, "versions"+project.SubLabel(sub)+": "+strings.Join(parts, ", "))
		}
	}
	if jsLoc != "" {
		if node, ok := nvmrc(filepath.Join(root, jsLoc, ".nvmrc")); ok {
			out = append(out, "node: "+node)
		} else if node, ok := nvmrc(filepath.Join(root, ".nvmrc")); ok {
			out = append(out, "node: "+node)
		}
	}
	return strings.Join(out, "\n") + "\n"
}

// stackLine is stack's report line, "<stack>: yes (extras) [pm] at subs",
// and the first subproject holding it; "" when none does.
func stackLine(cfg *config.Config, root string, subs []string, stack string) (line, first string) {
	var locs, extras []string
	pm := ""
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
		if pm == "" && hasEcosystem(cfg, stack) {
			if lock, ok := project.NearestLockfile(cfg, dir, stack); ok {
				pm = lock.Manager
			}
		}
	}
	if len(locs) == 0 {
		return "", ""
	}
	line = stack + ": yes"
	if len(extras) > 0 {
		line += " (" + strings.Join(extras, ",") + ")"
	}
	if stack == "js" {
		pm = cmp.Or(pm, cfg.DefaultManager("js"))
	}
	if pm != "" {
		line += " [" + pm + "]"
	}
	if len(locs) != 1 || locs[0] != "." {
		line += " at " + strings.Join(locs, ", ")
	}
	return line, locs[0]
}

func hasEcosystem(cfg *config.Config, ecosystem string) bool {
	for _, pm := range cfg.PackageManagers {
		if pm.Ecosystem == ecosystem {
			return true
		}
	}
	return false
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
		_, ok := tools.JSSpecs(dir)[rule.Dep]
		return ok
	case rule.PyDep != "":
		return tools.PythonDeps(dir)[rule.PyDep]
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

// versions lists "<name> <version> (<label>)" for each kit.yml versions
// entry whose stack is in dir, from the first source that yields one.
func versions(cfg *config.Config, root, dir string) []string {
	var parts []string
	for _, stack := range cfg.VersionOrder {
		if !cfg.HasStack(dir, stack) {
			continue
		}
		for _, name := range cfg.Versions[stack] {
			for _, src := range cfg.VersionSources[stack] {
				file := strings.ReplaceAll(src.File, "{name}", name)
				path := filepath.Join(dir, file)
				if src.Up {
					path = fsx.FindUp(dir, root, file)
				}
				if path == "" || !fsx.IsFile(path) {
					continue
				}
				arg := strings.ReplaceAll(src.Arg, "{name}", name)
				// Package names ignore case (pip).
				if src.Extractor == "regex_lines" {
					arg = "(?i)" + arg
				}
				values, err := sources.Run(src.Extractor, path, arg)
				if err == nil && len(values) > 0 && values[0] != "" {
					parts = append(parts, name+" "+values[0]+" ("+src.Label+")")
					break
				}
			}
		}
	}
	return parts
}

// nvmrc is the file's content with every "v" and newline removed.
func nvmrc(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return strings.NewReplacer("v", "", "\n", "").Replace(string(data)), true
}
