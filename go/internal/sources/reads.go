package sources

import (
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Reads reports whether rel, a path relative to a project root, is a file
// Entries reads: a task runner's manifest in any subproject, or a CI,
// git-hook, or task-graph config at the root.
func Reads(rel string) bool {
	rel = filepath.ToSlash(rel)
	for _, tp := range TaskProviders {
		if slices.ContainsFunc(tp.Manifests, func(m string) bool { return rel == m || strings.HasSuffix(rel, "/"+m) }) {
			return true
		}
	}
	for _, names := range [][]string{lintStagedConfigs, lefthookConfigs, commitlintConfigs, preCommitConfigs, {turboConfig, nxConfig, gitlabCI}} {
		if slices.Contains(names, rel) {
			return true
		}
	}
	workflow, _ := path.Match(githubWorkflows, rel)
	return workflow || path.Dir(rel) == huskyDir
}
