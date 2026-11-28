package codegraph

import (
	"cmp"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/ku5ic/claude-kit/go/internal/sources"
)

var (
	packageKeyCut    = regexp.MustCompile(`[\s<>=!~;\[]`)
	requirementsFile = regexp.MustCompile(`^requirements.*\.txt$`)
	gemLine          = regexp.MustCompile(`(?m)^\s*gem\s+['"]([^'"]+)`)
	goRequire        = regexp.MustCompile(`(?m)^\s*(?:require\s+)?([^\s()]+)\s+v\d\S*`)
	goModule         = regexp.MustCompile(`(?m)^module\s+(\S+)`)
	pubspecName      = regexp.MustCompile(`(?m)^name:\s*['"]?([\w-]+)`)
	rustAutoTarget   = regexp.MustCompile(`^(src/bin|examples|tests|benches)/[^/]+\.rs$`)
)

// packageKey is a requirement's package name: "PyYAML[ext]>=6" is pyyaml,
// and serde-json and serde_json compare equal.
func packageKey(requirement string) string {
	return strings.ReplaceAll(strings.ToLower(requirementName(requirement)), "-", "_")
}

// pythonKey is packageKey by PEP 503, as sources.PyName normalizes: any run
// of -, _, and . is one separator, so zope.interface and Zope-Interface
// are one distribution.
func pythonKey(requirement string) string {
	return strings.ReplaceAll(sources.PyName(requirementName(requirement)), "-", "_")
}

// requirementName is a requirement's name, before any extra, version, or
// marker.
func requirementName(requirement string) string {
	return packageKeyCut.Split(strings.TrimSpace(requirement), 2)[0]
}

// keyOf is name's key in ecosystem: Go matches a module path as a prefix,
// so its dashes stay.
func keyOf(ecosystem, name string) string {
	switch ecosystem {
	case "go":
		return strings.ToLower(name)
	case "python":
		return pythonKey(name)
	}
	return packageKey(name)
}

// localPrefixes mark a dependency spec that points into the repo.
var localPrefixes = []string{"workspace:", "file:", "link:", "portal:"}

// registryNames is a dependency table's names, minus path and workspace
// entries: those live in this repo.
func registryNames(deps any) []string {
	entries, _ := deps.(map[string]any)
	var names []string
	for name, spec := range entries {
		switch s := spec.(type) {
		case map[string]any:
			if s["path"] != nil || s["workspace"] != nil {
				continue
			}
		case string:
			if slices.ContainsFunc(localPrefixes, func(p string) bool { return strings.HasPrefix(s, p) }) {
				continue
			}
		}
		names = append(names, name)
	}
	return names
}

// readManifest is a manifest's ecosystem and registry dependency names;
// "" for a file that is no manifest.
func readManifest(name, file string) (string, []string, error) {
	switch {
	case name == "package.json":
		names, err := npmNames(file)
		return "npm", names, err
	case name == "Cargo.toml":
		names, err := cargoNames(file)
		return "rust", names, err
	case name == "pyproject.toml":
		names, err := pyprojectNames(file)
		return "python", names, err
	case requirementsFile.MatchString(name):
		names, err := requirementNames(file)
		return "python", names, err
	case name == "Gemfile":
		return "ruby", submatches(file, gemLine), nil
	case name == "go.mod":
		return "go", submatches(file, goRequire), nil
	}
	return "", nil, nil
}

// npmNames is package.json's registry dependencies, every kind.
func npmNames(file string) ([]string, error) {
	m, err := sources.DecodeErr[map[string]any](file, json.Unmarshal)
	var names []string
	for _, field := range []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"} {
		names = append(names, registryNames(m[field])...)
	}
	return names, err
}

// cargoNames is Cargo.toml's registry dependencies, the workspace's too.
func cargoNames(file string) ([]string, error) {
	m, err := sources.DecodeErr[map[string]any](file, toml.Unmarshal)
	var names []string
	for _, t := range []any{m["dependencies"], m["dev-dependencies"], m["build-dependencies"], sources.GetPath(m, "workspace.dependencies")} {
		names = append(names, registryNames(t)...)
	}
	return names, err
}

// pyprojectNames is pyproject.toml's requirements, optional groups and
// Poetry's table included, less what [tool.uv.sources] takes from a local
// path.
func pyprojectNames(file string) ([]string, error) {
	m, err := sources.DecodeErr[map[string]any](file, toml.Unmarshal)
	if err != nil {
		return nil, err
	}
	requirements := sources.StringList(sources.GetPath(m, "project.dependencies"))
	optional, _ := sources.GetPath(m, "project.optional-dependencies").(map[string]any)
	for _, group := range optional {
		requirements = append(requirements, sources.StringList(group)...)
	}
	local := map[string]bool{}
	uvSources, _ := sources.GetPath(m, "tool.uv.sources").(map[string]any)
	for n := range uvSources {
		local[pythonKey(n)] = true
	}
	names := slices.DeleteFunc(requirements, func(r string) bool { return local[pythonKey(r)] })
	return append(names, registryNames(sources.GetPath(m, "tool.poetry.dependencies"))...), nil
}

// requirementNames is a requirements file's requirement lines, comments,
// options, and local paths left out.
func requirementNames(file string) ([]string, error) {
	data, err := os.ReadFile(file)
	var names []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if t := strings.TrimSpace(line); t != "" && !strings.ContainsAny(t[:1], "#-./") {
			names = append(names, t)
		}
	}
	return names, err
}

// submatches is re's first group at each match in file; none when it
// can't be read.
func submatches(file string, re *regexp.Regexp) []string {
	data, _ := os.ReadFile(file)
	var out []string
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		out = append(out, m[1])
	}
	return out
}

// manifests is what the tracked manifests say: the registry packages per
// ecosystem, so src/graphql never shadows the graphql package; Go module
// paths, the longest first so nested modules win; Dart package lib dirs;
// and the stems of every Rust crate target root.
type manifests struct {
	declared  map[string]map[string]bool
	goModules []pair // module path, its directory
	npmNames  []pair // a package.json's name, its directory
	dartLibs  map[string]string
	rustRoots map[string]bool
	skipped   []string // file: why, for each manifest that didn't read
}

func readManifests(root string, files []string) manifests {
	m := manifests{declared: map[string]map[string]bool{}, dartLibs: map[string]string{}, rustRoots: map[string]bool{}}
	for _, file := range files {
		abs := filepath.Join(root, file)
		name := path.Base(file)
		// An unreadable or oddly shaped manifest declares nothing, and is
		// named, so its packages don't turn undeclared unexplained.
		ecosystem, names, err := readManifest(name, abs)
		switch {
		case err != nil:
			m.skipped = append(m.skipped, file+": "+err.Error())
		case ecosystem != "":
			m.declare(ecosystem, names)
		}
		m.addLayout(name, abs, file, files)
	}
	longestFirst := func(a, b pair) int { return cmp.Or(cmp.Compare(len(b.a), len(a.a)), strings.Compare(a.a, b.a)) }
	slices.SortStableFunc(m.goModules, longestFirst)
	slices.SortStableFunc(m.npmNames, longestFirst)
	return m
}

// declare records names as ecosystem's registry dependencies, by keyOf.
func (m manifests) declare(ecosystem string, names []string) {
	if m.declared[ecosystem] == nil {
		m.declared[ecosystem] = map[string]bool{}
	}
	for _, n := range names {
		m.declared[ecosystem][keyOf(ecosystem, n)] = true
	}
}

// addLayout records what the manifest file, abs on disk, says about the
// repo's layout: an npm package's name, a Go module path, a Dart lib dir,
// or a crate's target roots.
func (m *manifests) addLayout(name, abs, file string, files []string) {
	switch name {
	case "package.json":
		if pkg := sources.Decode[struct{ Name string }](abs, json.Unmarshal); pkg.Name != "" {
			m.npmNames = append(m.npmNames, pair{pkg.Name, dirOf(file)})
		}
	case "go.mod":
		if mods := submatches(abs, goModule); len(mods) > 0 {
			m.goModules = append(m.goModules, pair{mods[0], dirOf(file)})
		}
	case "pubspec.yaml":
		if names := submatches(abs, pubspecName); len(names) > 0 {
			m.dartLibs[strings.ToLower(names[0])] = path.Join(dirOf(file), "lib")
		}
	case "Cargo.toml":
		m.addRustRoots(abs, dirOf(file), files)
	}
}

// addRustRoots adds a crate's target roots: [lib] and [[bin]]-style paths,
// and the auto-discovered target files.
func (m manifests) addRustRoots(manifest, crate string, files []string) {
	cargo := sources.Decode[map[string]any](manifest, toml.Unmarshal)
	targets := []any{cargo["lib"]}
	for _, kind := range []string{"bin", "example", "test", "bench"} {
		list, _ := cargo[kind].([]any)
		targets = append(targets, list...)
	}
	for _, t := range targets {
		if target, ok := t.(map[string]any); ok {
			if p, ok := target["path"].(string); ok {
				full := path.Join(crate, p)
				m.rustRoots[strings.TrimSuffix(full, path.Ext(full))] = true
			}
		}
	}
	for _, f := range files {
		rest := f
		if crate != "" {
			var ok bool
			if rest, ok = strings.CutPrefix(f, crate+"/"); !ok {
				continue
			}
		}
		if rustAutoTarget.MatchString(rest) {
			m.rustRoots[strings.TrimSuffix(f, ".rs")] = true
		}
	}
}

// isDeclared is whether spec, by its external name, is a registry
// dependency of ecosystem; for Go, whether it sits in a required module.
func (m manifests) isDeclared(spec string, dotted bool, ecosystem string) bool {
	if ecosystem == "go" {
		lower := strings.ToLower(spec)
		for mod := range m.declared["go"] {
			if lower == mod || strings.HasPrefix(lower, mod+"/") {
				return true
			}
		}
		return false
	}
	key := keyOf(ecosystem, externalName(spec, dotted))
	if ecosystem == "python" {
		return m.declared["python"][key] || slices.ContainsFunc(pythonDistributions[key], func(d string) bool { return m.declared["python"][d] })
	}
	return m.declared[ecosystem][key]
}

// pythonDistributions maps an import name to the distributions that
// install it, both as pythonKey writes them. PyYAML installs yaml.
var pythonDistributions = map[string][]string{
	"yaml":     {"pyyaml"},
	"pil":      {"pillow"},
	"dateutil": {"python_dateutil"},
	"bs4":      {"beautifulsoup4"},
	"sklearn":  {"scikit_learn"},
	"cv2":      {"opencv_python", "opencv_python_headless", "opencv_contrib_python", "opencv_contrib_python_headless"},
}
