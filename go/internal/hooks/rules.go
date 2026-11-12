package hooks

import (
	"os"
	"path/filepath"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// rulesDir holds the kit's rule files.
func rulesDir(paths config.Paths) string {
	return filepath.Join(paths.Root, "skills", "kit-rules", "reference")
}

// leftoverRulesLink is the directory under <home>/rules that still holds the
// kit's rules from an install-rules.sh install, "" when there is none: the
// kit's own rules dir, a copy holding every rule file it has (the
// marketplace clone a link points at is never the plugin cache the hooks
// run from), or, under the claude-kit name that script used, any of them.
// Left in place, the rules would load twice.
func leftoverRulesLink(paths config.Paths) string {
	kitRules, err := filepath.EvalSymlinks(rulesDir(paths))
	if err != nil {
		return ""
	}
	ruleFiles, _ := filepath.Glob(filepath.Join(kitRules, "*.md"))
	entries, _ := os.ReadDir(filepath.Join(paths.Home, "rules"))
	for _, entry := range entries {
		dir := filepath.Join(paths.Home, "rules", entry.Name())
		if !fsx.IsDir(dir) {
			continue
		}
		if physical, err := filepath.EvalSymlinks(dir); err == nil && physical == kitRules {
			return dir
		}
		present := 0
		for _, rule := range ruleFiles {
			if fsx.IsFile(filepath.Join(dir, filepath.Base(rule))) {
				present++
			}
		}
		if present > 0 && (present == len(ruleFiles) || entry.Name() == "claude-kit") {
			return dir
		}
	}
	return ""
}
