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

// Owner is the file whose entries rel feeds, as Entry.File names it: rel
// when Entries reads it, .gitlab-ci.yml for a file it includes, else "".
func Owner(root, rel string) string {
	rel = filepath.ToSlash(rel)
	if Reads(rel) {
		return rel
	}
	doc := readYAML(filepath.Join(root, gitlabCI))
	if doc == nil {
		return ""
	}
	for _, inc := range gitlabLocalIncludes(doc["include"]) {
		if ok, _ := path.Match(path.Clean(strings.TrimPrefix(inc, "/")), rel); ok {
			return gitlabCI
		}
	}
	return ""
}
