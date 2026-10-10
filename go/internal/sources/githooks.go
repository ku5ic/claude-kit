package sources

import (
	"cmp"
	"encoding/json"
	"maps"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// readConfig is a config file as a map: YAML or TOML by its extension,
// else JSON, else YAML (a .lintstagedrc may be either). nil when it can't
// be read or parsed.
func readConfig(file string) map[string]any {
	switch filepath.Ext(file) {
	case ".yaml", ".yml":
		return readYAML(file)
	case ".toml":
		return Decode[map[string]any](file, toml.Unmarshal)
	}
	if m := Decode[map[string]any](file, json.Unmarshal); m != nil {
		return m
	}
	return readYAML(file)
}

// oneOrMany is a config value that is a string or a list of them.
func oneOrMany(v any) []string {
	if s, ok := v.(string); ok {
		return []string{s}
	}
	return strings_(v)
}

// lintStagedConfigs are where lint-staged looks, in its order; package.json
// counts only with a lint-staged key. A JS config is code, never run to be
// read.
var lintStagedConfigs = []string{"package.json", ".lintstagedrc", ".lintstagedrc.json", ".lintstagedrc.yaml", ".lintstagedrc.yml"}

// lintStagedEntries are root's lint-staged commands: each glob's commands,
// run at pre-commit with the staged files it matches appended.
func lintStagedEntries(root string) []Entry {
	for _, name := range lintStagedConfigs {
		config := readConfig(filepath.Join(root, name))
		if name == "package.json" {
			config, _ = config["lint-staged"].(map[string]any)
		}
		if config == nil {
			continue
		}
		var out []Entry
		for _, glob := range slices.Sorted(maps.Keys(config)) {
			for _, cmd := range oneOrMany(config[glob]) {
				out = append(out, Entry{Source: "lint-staged", File: name, Name: glob, Dir: ".", Body: Body{Text: cmd}, Files: []string{glob}, PassFiles: true, Stage: "pre-commit"})
			}
		}
		return out
	}
	return nil
}

var lefthookConfigs = []string{
	"lefthook.yml", "lefthook.yaml", "lefthook.json", "lefthook.toml",
	".lefthook.yml", ".lefthook.yaml", ".lefthook.json", ".lefthook.toml",
	".config/lefthook.yml", ".config/lefthook.yaml", ".config/lefthook.json", ".config/lefthook.toml",
}

// lefthookFiles maps lefthook's file-list templates to the kit's {files}.
var lefthookFiles = strings.NewReplacer("{staged_files}", "{files}", "{push_files}", "{files}", "{all_files}", "{files}")

// lefthookEntries are what each hook in root's lefthook config runs: its
// commands and scripts, by name, then its jobs, in order. Keys that aren't
// hooks hold none of the three.
func lefthookEntries(root string) []Entry {
	file := fsx.FindUp(root, root, lefthookConfigs...)
	config := readConfig(file)
	h := lefthookHook{file: fsx.Rel(root, file), sourceDir: cmp.Or(str(config["source_dir"]), ".lefthook")}
	var out []Entry
	for _, stage := range slices.Sorted(maps.Keys(config)) {
		hook, _ := config[stage].(map[string]any)
		h.stage = stage
		commands, _ := hook["commands"].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(commands)) {
			job, _ := commands[name].(map[string]any)
			out = h.add(out, name, "", job, nil)
		}
		scripts, _ := hook["scripts"].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(scripts)) {
			job, _ := scripts[name].(map[string]any)
			out = h.add(out, name, name, job, nil)
		}
		out = append(out, h.jobs(hook["jobs"], nil)...)
	}
	return out
}

// lefthookHook is one hook of a lefthook config, as its commands, jobs,
// and scripts need it.
type lefthookHook struct{ file, sourceDir, stage string }

// jobs are a jobs list's entries, a group's jobs in its place. A group's
// root, glob, and env hold for its jobs that don't set their own.
func (h lefthookHook) jobs(list any, parent map[string]any) []Entry {
	jobs, _ := list.([]any)
	var out []Entry
	for i, item := range jobs {
		job, _ := item.(map[string]any)
		if group, ok := job["group"].(map[string]any); ok {
			inherited := map[string]any{}
			maps.Copy(inherited, parent)
			maps.Copy(inherited, job)
			out = append(out, h.jobs(group["jobs"], inherited)...)
			continue
		}
		out = h.add(out, cmp.Or(str(job["name"]), strconv.Itoa(i+1)), str(job["script"]), job, parent)
	}
	return out
}

// add appends a command, job, or script: its run line, else its runner on
// the script under source_dir/<hook>. One that runs nothing or is skipped
// outright adds nothing.
func (h lefthookHook) add(out []Entry, name, script string, job, parent map[string]any) []Entry {
	own := func(key string) any {
		if v := job[key]; v != nil {
			return v
		}
		return parent[key]
	}
	body := lefthookFiles.Replace(str(job["run"]))
	if body == "" && script != "" {
		body = strings.TrimSpace(str(job["runner"]) + " " + path.Join(h.sourceDir, h.stage, script))
	}
	if body == "" || job["skip"] == true {
		return out
	}
	env, _ := literalEnv(own("env"))
	return append(out, Entry{Source: "lefthook", File: h.file, Name: name, Dir: filepath.Clean(str(own("root"))), Env: env, Body: Body{Text: body}, Files: oneOrMany(own("glob")), Stage: h.stage})
}

// huskyBootstrap is the line husky 5 to 8 put first in a hook to load
// itself.
var huskyBootstrap = regexp.MustCompile(`^\.\s.*husky\.sh"?$`)

// huskyEntries are root's husky hooks: each .husky file named for a git
// hook, as the lines it runs.
func huskyEntries(root string) []Entry {
	files, _ := filepath.Glob(filepath.Join(root, ".husky", "*"))
	var out []Entry
	for _, file := range files {
		stage := filepath.Base(file)
		// _ holds husky's own scripts, and a name with a dot is a helper.
		if strings.ContainsAny(stage, "._") || !fsx.IsFile(file) {
			continue
		}
		var lines []string
		EachLine(file, func(line string) {
			if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") && !huskyBootstrap.MatchString(trimmed) {
				lines = append(lines, line)
			}
		})
		if len(lines) > 0 {
			out = append(out, Entry{Source: "husky", File: fsx.Rel(root, file), Name: stage, Dir: ".", Body: Body{Text: strings.Join(lines, "\n")}, Stage: stage})
		}
	}
	return out
}

// commitlintConfigs are where commitlint looks after package.json's
// commitlint key. Any of them, JS included, says messages are checked.
var commitlintConfigs = []string{
	".commitlintrc", ".commitlintrc.json", ".commitlintrc.yaml", ".commitlintrc.yml",
	".commitlintrc.js", ".commitlintrc.cjs", ".commitlintrc.mjs", ".commitlintrc.ts", ".commitlintrc.cts",
	"commitlint.config.js", "commitlint.config.cjs", "commitlint.config.mjs", "commitlint.config.ts", "commitlint.config.cts",
}

// commitlintEntries is commitlint checking the commit message, at
// commit-msg with the message file appended, when root configures it.
func commitlintEntries(root string) []Entry {
	file := "package.json"
	if GetPath(readJSON(filepath.Join(root, file)), ".commitlint") == nil {
		file = fsx.Rel(root, fsx.FindUp(root, root, commitlintConfigs...))
	}
	if file == "" {
		return nil
	}
	return []Entry{{Source: "commitlint", File: file, Name: "commitlint", Dir: ".", Body: Body{Text: "commitlint --edit"}, PassFiles: true, Stage: "commit-msg"}}
}
