package sources

import (
	"cmp"
	"os"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// yamlRoot is file's top-level mapping node, nil when unreadable.
func yamlRoot(file string) *yaml.Node {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	return doc.Content[0]
}

// field is the value under key in mapping m, nil when absent.
func field(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// value is field's scalar value, "" when absent or not a scalar.
func value(m *yaml.Node, key string) string {
	if n := field(m, key); n != nil && n.Kind == yaml.ScalarNode {
		return n.Value
	}
	return ""
}

// items is a sequence's items, nil for anything else.
func items(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

// taskfileVar is go-task's {{.VAR}} interpolation: a value the kit can't know.
var taskfileVar = regexp.MustCompile(`\{\{[^}]*\}\}`)

// TaskfileTasks lists a go-task Taskfile's tasks in file order, without the
// internal ones `task` refuses to run directly.
func TaskfileTasks(file string) []string {
	var names []string
	eachTaskfileTask(file, func(name string, _ Body) { names = append(names, name) })
	return names
}

// taskfileBodies maps each task to its deps (as "task <dep>" lines) and its
// commands, one shell per command; a `task:` command reads as "task <name>".
func taskfileBodies(file string) map[string]Body {
	out := map[string]Body{}
	eachTaskfileTask(file, func(name string, body Body) { out[name] = body })
	return out
}

func eachTaskfileTask(file string, fn func(string, Body)) {
	tasks := field(yamlRoot(file), "tasks")
	if tasks == nil || tasks.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(tasks.Content); i += 2 {
		name, task := tasks.Content[i].Value, tasks.Content[i+1]
		if value(task, "internal") == "true" {
			continue
		}
		var lines []string
		switch task.Kind {
		case yaml.ScalarNode:
			lines = []string{task.Value}
		case yaml.SequenceNode:
			for _, cmd := range task.Content {
				lines = append(lines, cmd.Value)
			}
		case yaml.MappingNode:
			for _, dep := range items(field(task, "deps")) {
				if dep := cmp.Or(dep.Value, value(dep, "task")); dep != "" {
					lines = append(lines, "task "+dep)
				}
			}
			if cmd := value(task, "cmd"); cmd != "" {
				lines = append(lines, cmd)
			}
			for _, cmd := range items(field(task, "cmds")) {
				switch {
				case cmd.Kind == yaml.ScalarNode:
					lines = append(lines, cmd.Value)
				case value(cmd, "cmd") != "":
					lines = append(lines, value(cmd, "cmd"))
				case value(cmd, "task") != "":
					lines = append(lines, "task "+value(cmd, "task"))
				}
			}
		}
		for j, line := range lines {
			lines[j] = taskfileVar.ReplaceAllString(line, "$$TASK_VAR")
		}
		fn(name, Body{Text: strings.Join(lines, "\n"), PerLine: true})
	}
}

// PreCommitHooks lists the hook ids a .pre-commit-config.yaml runs, in file
// order, each once.
func PreCommitHooks(file string) []string {
	var ids []string
	eachPreCommitHook(file, func(id string, _ Body) {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	})
	return ids
}

// preCommitBodies maps each hook id to the command it runs with its args: a
// local hook's entry, else the id, which names the tool in a hook repo's
// own definition.
func preCommitBodies(file string) map[string]Body {
	out := map[string]Body{}
	eachPreCommitHook(file, func(id string, body Body) {
		if _, ok := out[id]; !ok {
			out[id] = body
		}
	})
	return out
}

func eachPreCommitHook(file string, fn func(string, Body)) {
	for _, repo := range items(field(yamlRoot(file), "repos")) {
		local := value(repo, "repo") == "local"
		for _, hook := range items(field(repo, "hooks")) {
			id := value(hook, "id")
			if id == "" {
				continue
			}
			cmd := id
			if entry := value(hook, "entry"); local && entry != "" {
				cmd = entry
			}
			var args []string
			for _, arg := range items(field(hook, "args")) {
				args = append(args, arg.Value)
			}
			fn(id, Body{Text: strings.Join(append([]string{cmd}, args...), " ")})
		}
	}
}
