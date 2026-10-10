package sources

import (
	"fmt"
	"slices"
	"strings"

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
// present is read), how to list its tasks (Tasks) and what each runs
// (Bodies, nil for a format whose manifest holds no commands; recipes list
// their dependencies first, as "make <dep>" lines), and the command that
// runs one ({task} the task, {pm} the directory's package manager: PM, or a
// verified one that rivals it). RunByPM replaces Run under a manager; Body
// wraps what the manifest holds into the
// shell the task runs ({body}). Prefixes are the command lines that run
// one of its tasks. Knowledge of formats, not configuration: a project
// states its tasks in its own files, and these say how to read them.
type TaskProvider struct {
	Name      string
	Stack     string
	Manifests []string
	Tasks     func(file string) []string
	Bodies    func(file string) map[string]Body
	Run       string
	PM        string
	RunByPM   map[string]string
	Body      string
	Prefixes  []Prefix
}

// Prefix is a command prefix that runs one of a provider's tasks: the word
// after it, or Task when set. With Shorthand, a word that names no such
// task is a tool instead (pnpm eslint).
type Prefix struct {
	Words     string
	Task      string
	Shorthand bool
}

// at binds arg, a path or a pattern, to an extractor.
func at[T any](extract func(file, arg string) T, arg string) func(string) T {
	return func(file string) T { return extract(file, arg) }
}

// Wrap is body as the shell its task runs: inside tp.Body, when tp has one.
func (tp TaskProvider) Wrap(body Body) Body {
	if tp.Body != "" && body.Text != "" {
		body.Text = strings.ReplaceAll(tp.Body, "{body}", body.Text)
	}
	return body
}

var TaskProviders = []TaskProvider{
	{Name: "package-scripts", Stack: "js", Manifests: []string{"package.json"}, Tasks: at(JSONKeys, ".scripts"), Bodies: at(scripts(readJSON), ".scripts"), Run: "{pm} run {task}", PM: "npm",
		Prefixes: []Prefix{{Words: "npm run"}, {Words: "npm run-script"}, {Words: "npm test", Task: "test"}, {Words: "npm t", Task: "test"},
			{Words: "pnpm run"}, {Words: "pnpm", Shorthand: true}, {Words: "yarn run"}, {Words: "yarn", Shorthand: true}, {Words: "bun run"}, {Words: "node --run"}}},
	{Name: "make", Manifests: []string{"Makefile", "makefile", "GNUmakefile"}, Tasks: MakeTargets, Bodies: makeBodies, Run: "make {task}", Prefixes: []Prefix{{Words: "make"}}},
	{Name: "just", Manifests: []string{"justfile", "Justfile", ".justfile"}, Tasks: JustRecipes, Bodies: justBodies, Run: "just {task}", Prefixes: []Prefix{{Words: "just"}}},
	{Name: "pdm", Stack: "python", Manifests: []string{"pyproject.toml"}, Tasks: at(TOMLKeys, ".tool.pdm.scripts"), Bodies: at(scripts(readTOML), ".tool.pdm.scripts"), Run: "pdm run {task}", Prefixes: []Prefix{{Words: "pdm run", Shorthand: true}}},
	{Name: "poe", Stack: "python", Manifests: []string{"pyproject.toml"}, Tasks: at(TOMLKeys, ".tool.poe.tasks"), Bodies: at(scripts(readTOML), ".tool.poe.tasks"), Run: "poe {task}", RunByPM: map[string]string{"poetry": "poetry run poe {task}"}, Prefixes: []Prefix{{Words: "poe"}}},
	{Name: "rake", Stack: "ruby", Manifests: []string{"Rakefile"}, Tasks: at(RegexLines, `^[[:space:]]*task[[:space:]]+:?"?([A-Za-z0-9_:]+)`), Run: "bundle exec rake {task}", Prefixes: []Prefix{{Words: "rake"}, {Words: "bin/rake"}}},
	{Name: "cargo-alias", Stack: "rust", Manifests: []string{".cargo/config.toml", ".cargo/config"}, Tasks: at(TOMLKeys, ".alias"), Bodies: at(scripts(readTOML), ".alias"), Run: "cargo {task}", Body: "cargo {body}"},
	{Name: "composer", Stack: "php", Manifests: []string{"composer.json"}, Tasks: at(JSONKeys, ".scripts"), Bodies: at(scripts(readJSON), ".scripts"), Run: "composer run {task}", Prefixes: []Prefix{{Words: "composer run"}, {Words: "composer run-script"}}},
	{Name: "taskfile", Manifests: []string{"Taskfile.yml", "taskfile.yml", "Taskfile.yaml", "taskfile.yaml", "Taskfile.dist.yml", "Taskfile.dist.yaml"}, Tasks: TaskfileTasks, Bodies: taskfileBodies, Run: "task {task}", Prefixes: []Prefix{{Words: "task"}}},
	// Its hooks are preCommitEntries, which keep their stages; no Tasks.
	{Name: "pre-commit", Manifests: []string{".pre-commit-config.yaml", ".pre-commit-config.yml"}, Run: "pre-commit run {task} --all-files", Prefixes: []Prefix{{Words: "pre-commit run"}}},
}
