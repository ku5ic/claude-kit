package sources

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// Toolchains are the language manifests and the binaries each one's own
// toolchain provides: a project with the manifest states them.
var Toolchains = map[string][]string{
	"go.mod":         {"go", "gofmt"},
	"Cargo.toml":     {"cargo", "rustc", "rustfmt"},
	"package.json":   {"node", "npm", "npx"},
	"pyproject.toml": {"python", "python3"},
	"Gemfile":        {"ruby", "bundle"},
	"composer.json":  {"php", "composer"},
}

// Anchors are the language manifests, sorted: a directory holding one is a
// project root, or a subproject below one.
func Anchors() []string { return slices.Sorted(maps.Keys(Toolchains)) }

// Language is a language the manifests in a directory state, and the
// dependencies they declare.
type Language struct {
	Name      string
	Manifests []string
	Deps      func(dir string) Deps
}

// Languages are in the order a stack report lists them.
var Languages = []Language{
	{"js", []string{"package.json"}, JSDeps},
	{"python", []string{"pyproject.toml", "Pipfile", "requirements.txt"}, PythonDeps},
	{"go", []string{"go.mod"}, nil},
	{"rust", []string{"Cargo.toml"}, nil},
	{"ruby", []string{"Gemfile"}, RubyDeps},
	{"php", []string{"composer.json"}, nil},
}

// Declared are the dependencies every language's manifests in dir declare.
func Declared(dir string) Deps {
	deps := Deps{}
	for _, l := range Languages {
		if l.Deps != nil {
			maps.Copy(deps, l.Deps(dir))
		}
	}
	return deps
}

// Has is true when deps holds name as written or, for Python, normalized.
func (deps Deps) Has(name string) bool { return deps[name] || deps[PyName(name)] }

// HasManifest is true when dir holds a language manifest or a task
// runner's: something there states how it is built and checked.
func HasManifest(dir string) bool {
	for manifest := range Toolchains {
		if fsx.IsFile(filepath.Join(dir, manifest)) {
			return true
		}
	}
	return slices.ContainsFunc(TaskProviders, func(tp TaskProvider) bool { return fsx.FindUp(dir, dir, tp.Manifests...) != "" })
}

// Deps are the packages a manifest declares, by normalized name.
type Deps map[string]bool

var (
	pep508Name = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*`)
	nameRuns   = regexp.MustCompile(`[-_.]+`)
)

// PyName is PEP 503's name normalization: case and -_. runs don't matter.
func PyName(name string) string {
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
	if doc := readTOMLTable(filepath.Join(dir, "pyproject.toml")); doc != nil {
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
	if doc := readTOMLTable(filepath.Join(dir, "Pipfile")); doc != nil {
		deps.addKeys(doc["packages"])
		deps.addKeys(doc["dev-packages"])
	}
	reqs, _ := filepath.Glob(filepath.Join(dir, "requirements*.txt"))
	for _, file := range reqs {
		EachLine(file, func(line string) {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "-") {
				deps.addReq(line)
			}
		})
	}
	return deps
}

func readTOMLTable(file string) map[string]any {
	return Decode[map[string]any](file, toml.Unmarshal)
}

// addReq adds a PEP 508 requirement's name.
func (d Deps) addReq(req string) {
	if m := pep508Name.FindString(strings.TrimSpace(req)); m != "" {
		d[PyName(m)] = true
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
			d[PyName(name)] = true
		}
	}
}

var gemSpec = regexp.MustCompile(`^    ([A-Za-z0-9_.-]+) \(`)

// RubyDeps are the gems a Gemfile.lock resolves.
func RubyDeps(dir string) Deps {
	deps := Deps{}
	EachLine(filepath.Join(dir, "Gemfile.lock"), func(line string) {
		if m := gemSpec.FindStringSubmatch(line); m != nil {
			deps[m[1]] = true
		}
	})
	return deps
}
