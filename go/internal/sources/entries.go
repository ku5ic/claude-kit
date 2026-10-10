package sources

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// Entry is one command a project's configs say to run. Source names the
// format; File is the config it came from and Dir where it runs, both
// relative to the root. Files are the globs it applies to, in the source's
// syntax: nil for the whole tree, or when the source filters files itself
// (pre-commit). PassFiles appends the matched files to Body; a {files} in
// Body takes them in place. Stage is the git hook it runs at, "" for CI and
// tasks.
type Entry struct {
	Source, File, Name, Dir string
	Env                     []string
	Body                    Body
	Files                   []string
	PassFiles               bool
	Stage                   string
}

// String is the entry as `kit enforce --list` prints it, tab-separated:
// source, file, stage, dir, name, files, and the command, its env first,
// {files} last when files are appended, newlines as \n.
func (e Entry) String() string {
	command := e.Command()
	if e.PassFiles {
		command += " {files}"
	}
	return strings.Join([]string{e.Source, e.File, cmp.Or(e.Stage, "-"), e.Dir, e.Name, cmp.Or(strings.Join(e.Files, ","), "-"), strings.ReplaceAll(command, "\n", `\n`)}, "\t")
}

// Command is the body with its env in front, as a shell line.
func (e Entry) Command() string {
	return strings.Join(append(slices.Clone(e.Env), e.Body.Text), " ")
}

// Entries lists what root's configs say to run: CI steps, the git-hook
// managers' commands, turbo's and nx's tasks, and the task runners' tasks
// in each of dirs, the subprojects relative to root.
func Entries(cfg *config.Config, root string, dirs []string) []Entry {
	var out []Entry
	for _, s := range Steps(cfg, root) {
		source := "github-actions"
		if s.File == gitlabCI {
			source = "gitlab-ci"
		}
		out = append(out, Entry{Source: source, File: s.File, Name: s.Name, Dir: s.Dir, Env: s.Env, Body: Body{Text: s.Run}})
	}
	for _, read := range []func(string) []Entry{preCommitEntries, lintStagedEntries, lefthookEntries, huskyEntries, commitlintEntries, turboEntries, nxEntries} {
		out = append(out, read(root)...)
	}
	for _, dir := range dirs {
		out = append(out, taskEntries(cfg, root, dir)...)
	}
	return out
}

// taskEntries are the tasks of each task runner with a manifest in dir.
// pre-commit's hooks are preCommitEntries, which keep their stages.
func taskEntries(cfg *config.Config, root, dir string) []Entry {
	var out []Entry
	abs := filepath.Join(root, dir)
	for _, tp := range TaskProviders {
		manifest := fsx.FindUp(abs, abs, tp.Manifests...)
		if manifest == "" || tp.Name == "pre-commit" || slices.Contains(cfg.DisabledTaskProviders, tp.Name) {
			continue
		}
		names, _ := Run(tp.Extractor, manifest, tp.Arg)
		bodies := Bodies(tp.Extractor, manifest, tp.Arg)
		for _, name := range names {
			if name != "" {
				out = append(out, Entry{Source: tp.Name, File: fsx.Rel(root, manifest), Name: name, Dir: dir, Body: tp.Wrap(bodies[name])})
			}
		}
	}
	return out
}

// turboEntries run each task turbo.json configures, through turbo. A
// package's task (web#build) or a root task (//#lint) runs as its task.
func turboEntries(root string) []Entry {
	file := filepath.Join(root, "turbo.json")
	var out []Entry
	seen := map[string]bool{}
	for _, key := range append(JSONKeys(file, ".tasks"), JSONKeys(file, ".pipeline")...) {
		task := key[strings.LastIndex(key, "#")+1:]
		if !seen[task] {
			seen[task] = true
			out = append(out, Entry{Source: "turbo", File: "turbo.json", Name: task, Dir: ".", Body: Body{Text: "turbo run " + task}})
		}
	}
	return out
}

// nxEntries run each target nx.json has defaults for, on the affected
// projects: nx's own form, so a verified form only ever adds flags to it.
// Defaults keyed by an executor (@nx/jest:jest) name no target.
func nxEntries(root string) []Entry {
	var out []Entry
	for _, target := range JSONKeys(filepath.Join(root, "nx.json"), ".targetDefaults") {
		if !strings.Contains(target, ":") {
			out = append(out, Entry{Source: "nx", File: "nx.json", Name: target, Dir: ".", Body: Body{Text: "nx affected -t " + target}})
		}
	}
	return out
}
