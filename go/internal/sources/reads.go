package sources

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
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

// ChangedTasks names each task an edit of rel, a path relative to a project
// root, from before to after adds or gives another command: what the gates
// would run from it. A removed task runs nothing, so it isn't named. Each
// side is read alone, in a temp root holding only that file.
func ChangedTasks(cfg *config.Config, rel, before, after string) []string {
	rel = filepath.ToSlash(rel)
	if !Reads(rel) {
		return nil
	}
	old, now := fileTasks(cfg, rel, before), fileTasks(cfg, rel, after)
	var changed []string
	for key, command := range now {
		if prev, ok := old[key]; !ok || prev != command {
			changed = append(changed, key.name)
		}
	}
	slices.Sort(changed)
	return slices.Compact(changed)
}

// taskKey is a task as an edit can't change it without changing which task
// it is.
type taskKey struct{ source, stage, dir, name string }

// fileTasks is what Entries reads from rel holding content, each task's
// command and files by its key; none for empty content.
func fileTasks(cfg *config.Config, rel, content string) map[taskKey]string {
	out := map[taskKey]string{}
	if content == "" {
		return out
	}
	root, err := os.MkdirTemp("", "kit-tasks-")
	if err != nil {
		return out
	}
	// A temp dir left behind holds one copied file; the OS clears it.
	defer func() { _ = os.RemoveAll(root) }()
	file := filepath.Join(root, filepath.FromSlash(rel))
	if os.MkdirAll(filepath.Dir(file), 0o700) != nil || os.WriteFile(file, []byte(content), 0o600) != nil {
		return out
	}
	for _, e := range Entries(cfg, root, []string{path.Dir(rel)}) {
		if e.File == rel {
			out[taskKey{e.Source, e.Stage, e.Dir, e.Name}] = e.Command() + "\x00" + strings.Join(e.Files, ",")
		}
	}
	return out
}
