package sources

import (
	"fmt"
	"slices"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

// LoadConfig is config.Load plus the warning only the task providers can
// give: a disabled_task_providers entry that names none, so a typo doesn't
// silently disable nothing.
func LoadConfig(p config.Paths) (*config.Config, []config.Warning, error) {
	cfg, warnings, err := config.Load(p)
	if err != nil {
		return cfg, warnings, err
	}
	from := p.Base
	if cfg.Tag == "merged" {
		from = p.Overlay
	}
	for _, d := range cfg.DisabledTaskProviders {
		if !slices.ContainsFunc(TaskProviders, func(tp TaskProvider) bool { return tp.Name == d }) {
			warnings = append(warnings, config.Warning{File: from, Err: fmt.Errorf("disabled_task_providers: %q names no task provider", d)})
		}
	}
	return cfg, warnings, nil
}

// TaskProvider is a task-runner format: the manifests it lives in (the first
// present is read), how to list its tasks (Run's Extractor and Arg), and the
// command that runs one ({task} the task, {pm} the directory's package
// manager). RunByPM replaces Run under a manager; Body wraps what the
// manifest holds into the shell the task runs ({body}). Knowledge of
// formats, not configuration: a project states its tasks in its own files,
// and these say how to read them.
type TaskProvider struct {
	Name      string
	Stack     string
	Manifests []string
	Extractor string
	Arg       string
	Run       string
	RunByPM   map[string]string
	Body      string
}

var TaskProviders = []TaskProvider{
	{Name: "package-scripts", Stack: "js", Manifests: []string{"package.json"}, Extractor: "json_keys", Arg: ".scripts", Run: "{pm} run {task}"},
	{Name: "make", Manifests: []string{"Makefile", "makefile", "GNUmakefile"}, Extractor: "make_targets", Run: "make {task}"},
	{Name: "just", Manifests: []string{"justfile", "Justfile", ".justfile"}, Extractor: "just_recipes", Run: "just {task}"},
	{Name: "pdm", Stack: "python", Manifests: []string{"pyproject.toml"}, Extractor: "toml_keys", Arg: ".tool.pdm.scripts", Run: "pdm run {task}"},
	{Name: "poe", Stack: "python", Manifests: []string{"pyproject.toml"}, Extractor: "toml_keys", Arg: ".tool.poe.tasks", Run: "poe {task}", RunByPM: map[string]string{"poetry": "poetry run poe {task}"}},
	{Name: "rake", Stack: "ruby", Manifests: []string{"Rakefile"}, Extractor: "regex_lines", Arg: `^[[:space:]]*task[[:space:]]+:?"?([A-Za-z0-9_:]+)`, Run: "bundle exec rake {task}"},
	{Name: "cargo-alias", Stack: "rust", Manifests: []string{".cargo/config.toml", ".cargo/config"}, Extractor: "toml_keys", Arg: ".alias", Run: "cargo {task}", Body: "cargo {body}"},
	{Name: "composer", Stack: "php", Manifests: []string{"composer.json"}, Extractor: "json_keys", Arg: ".scripts", Run: "composer run {task}"},
	{Name: "taskfile", Manifests: []string{"Taskfile.yml", "taskfile.yml", "Taskfile.yaml", "taskfile.yaml", "Taskfile.dist.yml", "Taskfile.dist.yaml"}, Extractor: "taskfile_tasks", Run: "task {task}"},
	{Name: "pre-commit", Manifests: []string{".pre-commit-config.yaml", ".pre-commit-config.yml"}, Extractor: "precommit_hooks", Run: "pre-commit run {task} --all-files"},
}
