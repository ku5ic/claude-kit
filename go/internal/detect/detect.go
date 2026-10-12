// Package detect writes the compact stack report `kit detect-stack` prints and
// the <repo-context> block carries:
//
//	root: <root>
//	<language>: yes (<dependencies>) at <subproject>, ...
//	package-manager [<lockfile dir>]: <manager> (<lockfile>)
//
// A language is one whose manifest a subproject holds (sources.Languages),
// listed with the dependencies it declares that dependency_skills names,
// in that map's order. Package managers are gap-fill's verified facts, read
// from its cache: none until a run has answered. Nothing is printed when no
// subproject holds a manifest.
package detect

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Report is the full text for root, "" when no language is found.
func Report(cfg *config.Config, root, cacheDir string) string {
	subs := project.Subprojects(root)
	out := []string{"root: " + root}
	for _, lang := range sources.Languages {
		if line := languageLine(cfg, root, subs, lang); line != "" {
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

// languageLine is lang's report line, "<language>: yes (deps) at subs";
// "" when no subproject holds its manifest.
func languageLine(cfg *config.Config, root string, subs []string, lang sources.Language) string {
	var locs, deps []string
	for _, sub := range subs {
		dir := filepath.Join(root, sub)
		if !slices.ContainsFunc(lang.Manifests, func(m string) bool { return fsx.IsFile(filepath.Join(dir, m)) }) {
			continue
		}
		locs = append(locs, sub)
		if lang.Deps == nil {
			continue
		}
		declared := lang.Deps(dir)
		for _, rule := range cfg.DependencySkills {
			for _, dep := range rule.Deps {
				if declared.Has(dep) && !slices.Contains(deps, dep) {
					deps = append(deps, dep)
				}
			}
		}
	}
	if len(locs) == 0 {
		return ""
	}
	line := lang.Name + ": yes"
	if len(deps) > 0 {
		line += " (" + strings.Join(deps, ",") + ")"
	}
	if len(locs) != 1 || locs[0] != "." {
		line += " at " + strings.Join(locs, ", ")
	}
	return line
}
