// Package tools knows the linters, type checkers, and test runners the Stop
// hook runs on edited files: how each is detected in a project, where it
// runs, how to call it without writing, and where its binary comes from.
package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/ku5ic/claude-kit/go/internal/extract"
)

// Deps are the packages a manifest declares, by normalized name.
type Deps map[string]bool

var (
	pep508Name = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*`)
	nameRuns   = regexp.MustCompile(`[-_.]+`)
)

// normalize is PEP 503's name normalization: case and -_. runs don't matter.
func normalize(name string) string {
	return strings.ToLower(nameRuns.ReplaceAllString(name, "-"))
}

// JSDeps are package.json's dependencies and devDependencies.
func JSDeps(dir string) Deps {
	deps := Deps{}
	for name := range JSSpecs(dir) {
		deps[name] = true
	}
	return deps
}

// JSSpecs maps package.json's dependencies and devDependencies to the
// version specs they declare; devDependencies win a clash.
func JSSpecs(dir string) map[string]string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Dependencies    map[string]any `json:"dependencies"`
		DevDependencies map[string]any `json:"devDependencies"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	specs := map[string]string{}
	for _, table := range []map[string]any{pkg.Dependencies, pkg.DevDependencies} {
		for name, spec := range table {
			s, _ := spec.(string)
			specs[name] = s
		}
	}
	return specs
}

// TestScript is package.json's scripts.test, "" when absent.
func TestScript(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return ""
	}
	return pkg.Scripts["test"]
}

// PythonDeps are every requirement pyproject.toml declares (PEP 621
// dependencies and optional-dependencies, PEP 735 dependency groups,
// Poetry's dependency tables, PDM's dev-dependencies) plus requirements
// files' entries, by normalized name.
func PythonDeps(dir string) Deps {
	deps := Deps{}
	addReq := func(req string) {
		if m := pep508Name.FindString(strings.TrimSpace(req)); m != "" {
			deps[normalize(m)] = true
		}
	}
	addList := func(v any) {
		items, _ := v.([]any)
		for _, item := range items {
			if s, ok := item.(string); ok {
				addReq(s)
			}
		}
	}
	addTables := func(v any) {
		groups, _ := v.(map[string]any)
		for _, g := range groups {
			addList(g)
		}
	}
	addKeys := func(v any) {
		table, _ := v.(map[string]any)
		for name := range table {
			if name != "python" {
				deps[normalize(name)] = true
			}
		}
	}

	if data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml")); err == nil {
		var doc map[string]any
		if toml.Unmarshal(data, &doc) == nil {
			project, _ := doc["project"].(map[string]any)
			addList(project["dependencies"])
			addTables(project["optional-dependencies"])
			addTables(doc["dependency-groups"])
			tool, _ := doc["tool"].(map[string]any)
			poetry, _ := tool["poetry"].(map[string]any)
			addKeys(poetry["dependencies"])
			addKeys(poetry["dev-dependencies"])
			groups, _ := poetry["group"].(map[string]any)
			for _, g := range groups {
				group, _ := g.(map[string]any)
				addKeys(group["dependencies"])
			}
			pdm, _ := tool["pdm"].(map[string]any)
			addTables(pdm["dev-dependencies"])
		}
	}
	reqs, _ := filepath.Glob(filepath.Join(dir, "requirements*.txt"))
	for _, file := range reqs {
		extract.EachLine(file, func(line string) {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "-") {
				addReq(line)
			}
		})
	}
	return deps
}

var gemSpec = regexp.MustCompile(`^    ([A-Za-z0-9_.-]+) \(`)

// RubyDeps are the gems a Gemfile.lock resolves.
func RubyDeps(dir string) Deps {
	deps := Deps{}
	extract.EachLine(filepath.Join(dir, "Gemfile.lock"), func(line string) {
		if m := gemSpec.FindStringSubmatch(line); m != nil {
			deps[m[1]] = true
		}
	})
	return deps
}
