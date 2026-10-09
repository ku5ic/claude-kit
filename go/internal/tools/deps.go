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

// PythonDeps are every requirement pyproject.toml declares (PEP 621
// dependencies and optional-dependencies, PEP 735 dependency groups,
// Poetry's dependency tables, PDM's dev-dependencies) plus requirements
// files' entries and Pipfile's packages, by normalized name.
func PythonDeps(dir string) Deps {
	deps := Deps{}
	if doc, ok := readTOML(filepath.Join(dir, "pyproject.toml")); ok {
		project, _ := doc["project"].(map[string]any)
		deps.addList(project["dependencies"])
		deps.addTables(project["optional-dependencies"])
		deps.addTables(doc["dependency-groups"])
		tool, _ := doc["tool"].(map[string]any)
		poetry, _ := tool["poetry"].(map[string]any)
		deps.addKeys(poetry["dependencies"])
		deps.addKeys(poetry["dev-dependencies"])
		groups, _ := poetry["group"].(map[string]any)
		for _, g := range groups {
			group, _ := g.(map[string]any)
			deps.addKeys(group["dependencies"])
		}
		pdm, _ := tool["pdm"].(map[string]any)
		deps.addTables(pdm["dev-dependencies"])
	}
	if doc, ok := readTOML(filepath.Join(dir, "Pipfile")); ok {
		deps.addKeys(doc["packages"])
		deps.addKeys(doc["dev-packages"])
	}
	reqs, _ := filepath.Glob(filepath.Join(dir, "requirements*.txt"))
	for _, file := range reqs {
		extract.EachLine(file, func(line string) {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "-") {
				deps.addReq(line)
			}
		})
	}
	return deps
}

func readTOML(file string) (map[string]any, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, false
	}
	var doc map[string]any
	return doc, toml.Unmarshal(data, &doc) == nil
}

// addReq adds a PEP 508 requirement's name.
func (d Deps) addReq(req string) {
	if m := pep508Name.FindString(strings.TrimSpace(req)); m != "" {
		d[normalize(m)] = true
	}
}

// addList adds a list of requirement strings.
func (d Deps) addList(v any) {
	items, _ := v.([]any)
	for _, item := range items {
		if s, ok := item.(string); ok {
			d.addReq(s)
		}
	}
}

// addTables adds each requirement list of a table of them.
func (d Deps) addTables(v any) {
	groups, _ := v.(map[string]any)
	for _, g := range groups {
		d.addList(g)
	}
}

// addKeys adds a table's keys as names (Poetry, Pipfile), minus python.
func (d Deps) addKeys(v any) {
	table, _ := v.(map[string]any)
	for name := range table {
		if name != "python" {
			d[normalize(name)] = true
		}
	}
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
