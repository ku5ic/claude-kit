package tools

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Derived is what a project's own invocation of a tool adds to the
// file-scoped command: its env assignments and the allow-listed flags
// (Carry), with where they came from. The project's paths, globs, and any
// flag not on the list never carry: the check runs on the edited files, and
// never writes.
type Derived struct {
	Env    []string
	Flags  []string
	Source string
}

// carriedEnv is every env name a derived check may carry. Repo scripts are
// untrusted: GOFLAGS=-toolexec, NODE_OPTIONS=--require, or PYTHONPATH would
// run repo code under tools that otherwise don't.
var carriedEnv = map[string]bool{"CI": true, "TZ": true, "NODE_ENV": true, "DEBUG": true, "FORCE_COLOR": true, "NO_COLOR": true}

var notACheck = regexp.MustCompile(`(^|:)(fix|format|watch|dev|ui|coverage)($|:)|:fix$`)

// Derive finds the project's own invocation of the tool, from the run
// directory's tasks in task_providers order, then from the repo's CI steps
// that run in that directory. Tasks named for fixing, watching, or UIs are
// skipped: their flags aren't a check's.
func (a Adapter) Derive(cfg *config.Config, dir, root string) (Derived, bool) {
	tasks := project.Tasks(cfg, dir)
	// The classifier asks whether a task exists: pnpm eslint runs the
	// eslint binary only when there's no eslint script.
	lookup := func(provider, rel, name string) bool {
		return (rel == "" || rel == ".") && slices.ContainsFunc(tasks, func(t project.Task) bool { return t.Provider == provider && t.Name == name })
	}
	fromBody := func(body string) (Derived, bool) {
		for _, cmd := range classify.Body(cfg, body, lookup).Commands {
			if d, ok := a.match(cmd); ok {
				return d, true
			}
		}
		return Derived{}, false
	}
	for _, t := range tasks {
		if notACheck.MatchString(t.Name) {
			continue
		}
		if d, ok := fromBody(t.Body); ok {
			d.Source = t.Cmd
			return d, true
		}
	}
	// CI jobs are peers, so a bare `eslint .` in one doesn't hide another
	// job's --max-warnings: the first step carrying flags wins.
	var first Derived
	found := false
	for _, step := range sources.Steps(cfg, root) {
		if filepath.Join(root, step.Dir) != dir {
			continue
		}
		d, ok := fromBody(step.Run)
		if !ok {
			continue
		}
		d.Env = append(carried(step.Env), d.Env...)
		d.Source = step.File
		if len(d.Flags) > 0 {
			return d, true
		}
		if !found {
			first, found = d, true
		}
	}
	return first, found
}

// match reads one classified command as the adapter's tool: its name,
// then its required subcommand, with the env and allow-listed flags it
// carries. A command the classifier couldn't read whole never matches.
func (a Adapter) match(cmd classify.Command) (Derived, bool) {
	if cmd.Kind != classify.Gate && cmd.Kind != classify.Other || cmd.Expansion || len(cmd.Words) == 0 || filepath.Base(cmd.Words[0]) != a.Bin {
		return Derived{}, false
	}
	words := cmd.Words[1:]
	if len(a.Sub) > 0 {
		if len(words) == 0 || !slices.Contains(a.Sub, words[0]) {
			return Derived{}, false
		}
		words = words[1:]
	}
	d := Derived{Env: carried(cmd.Env)}
	for i := 0; i < len(words); i++ {
		flag, _, hasValue := strings.Cut(words[i], "=")
		takesValue, ok := a.Carry[flag]
		if !ok {
			continue
		}
		d.Flags = append(d.Flags, words[i])
		if takesValue && !hasValue && i+1 < len(words) {
			i++
			d.Flags = append(d.Flags, words[i])
		}
	}
	return d, true
}

// carried is env's K=V words whose name is on carriedEnv and whose value
// has nothing a shell would expand.
func carried(env []string) []string {
	var out []string
	for _, kv := range env {
		if name, _, _ := strings.Cut(kv, "="); carriedEnv[name] && !strings.ContainsAny(kv, "$`") {
			out = append(out, kv)
		}
	}
	return out
}
