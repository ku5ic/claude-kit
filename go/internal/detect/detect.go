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
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/extract"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/tools"
)

// Report is the full text for root, "" when no stack is found.
func Report(cfg *config.Config, root string) string {
	subs := project.Subprojects(cfg, root)
	dirOf := func(sub string) string {
		if sub == "." {
			return root
		}
		return filepath.Join(root, sub)
	}

	var lines []string
	jsLoc := ""
	for _, stack := range cfg.StackOrder {
		var locs, extras []string
		pm := ""
		for _, sub := range subs {
			dir := dirOf(sub)
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
			continue
		}
		line := stack + ": yes"
		if len(extras) > 0 {
			line += " (" + strings.Join(extras, ",") + ")"
		}
		switch {
		case stack == "js":
			if pm == "" {
				pm = cfg.DefaultManager("js")
			}
			line += " [" + pm + "]"
			jsLoc = locs[0]
		case pm != "":
			line += " [" + pm + "]"
		}
		if len(locs) != 1 || locs[0] != "." {
			line += " at " + strings.Join(locs, ", ")
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}

	out := []string{"root: " + root}
	out = append(out, lines...)
	for _, sub := range subs {
		if parts := versions(cfg, root, dirOf(sub)); len(parts) > 0 {
			prefix := "versions"
			if sub != "." {
				prefix += " [" + sub + "]"
			}
			out = append(out, prefix+": "+strings.Join(parts, ", "))
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

func hasEcosystem(cfg *config.Config, ecosystem string) bool {
	for _, pm := range cfg.PackageManagers {
		if pm.Ecosystem == ecosystem {
			return true
		}
	}
	return false
}

// matchExtra evaluates one extra's first rule kind present, in the order
// dep, pydep, file, grep, any_of, and reports whether it matches.
func matchExtra(extra config.Extra, dir string) bool {
	switch {
	case extra.Dep != "":
		_, ok := tools.JSSpecs(dir)[extra.Dep]
		return ok
	case extra.PyDep != "":
		return tools.PythonDeps(dir)[extra.PyDep]
	case extra.File != "":
		return project.IsFile(filepath.Join(dir, extra.File))
	case extra.Grep != "":
		return grepAny(dir, extra.Grep, extra.In)
	}
	for _, rule := range extra.AnyOf {
		if rule.File != "" && project.IsFile(filepath.Join(dir, rule.File)) {
			return true
		}
		if rule.Grep != "" && grepAny(dir, rule.Grep, rule.In) {
			return true
		}
		if rule.PyDep != "" && tools.PythonDeps(dir)[rule.PyDep] {
			return true
		}
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
					path = project.FindUp(dir, root, file)
				}
				if path == "" || !project.IsFile(path) {
					continue
				}
				arg := strings.ReplaceAll(src.Arg, "{name}", name)
				// Package names ignore case (pip).
				if src.Extractor == "regex_lines" {
					arg = "(?i)" + arg
				}
				values, err := extract.Run(src.Extractor, path, arg)
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
