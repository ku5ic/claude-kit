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
	name := packageKeyCut.Split(strings.TrimSpace(requirement), 2)[0]
	return strings.ReplaceAll(strings.ToLower(name), "-", "_")
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

// lookup is m's value at keys, nil when one is missing or not a table.
func lookup(m map[string]any, keys ...string) any {
	var v any = m
	for _, k := range keys {
		t, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = t[k]
	}
	return v
}

// stringList is v's string items, when v is a list.
func stringList(v any) []string {
	items, _ := v.([]any)
	var out []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// readManifest is a manifest's ecosystem and registry dependency names;
// "" for a file that is no manifest.
func readManifest(name, file string) (string, []string, error) {
	switch {
	case name == "package.json":
		var m map[string]any
		if err := readJSON(file, &m); err != nil {
			return "", nil, err
		}
		var names []string
		for _, field := range []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"} {
			names = append(names, registryNames(m[field])...)
		}
		return "npm", names, nil
	case name == "Cargo.toml":
		m, err := readTOML(file)
		if err != nil {
			return "", nil, err
		}
		var names []string
		for _, t := range []any{m["dependencies"], m["dev-dependencies"], m["build-dependencies"], lookup(m, "workspace", "dependencies")} {
			names = append(names, registryNames(t)...)
		}
		return "rust", names, nil
	case name == "pyproject.toml":
		m, err := readTOML(file)
		if err != nil {
			return "", nil, err
		}
		requirements := stringList(lookup(m, "project", "dependencies"))
		optional, _ := lookup(m, "project", "optional-dependencies").(map[string]any)
		for _, group := range optional {
			requirements = append(requirements, stringList(group)...)
		}
		local := map[string]bool{}
		sources, _ := lookup(m, "tool", "uv", "sources").(map[string]any)
		for n := range sources {
			local[packageKey(n)] = true
		}
		var names []string
		for _, r := range requirements {
			if !local[packageKey(r)] {
				names = append(names, r)
			}
		}
		return "python", append(names, registryNames(lookup(m, "tool", "poetry", "dependencies"))...), nil
	case requirementsFile.MatchString(name):
		data, err := os.ReadFile(file)
		var names []string
		for line := range strings.SplitSeq(string(data), "\n") {
			if t := strings.TrimSpace(line); t != "" && !strings.ContainsAny(t[:1], "#-./") {
				names = append(names, t)
			}
		}
		return "python", names, err
	case name == "Gemfile":
		return "ruby", submatches(file, gemLine), nil
	case name == "go.mod":
		return "go", submatches(file, goRequire), nil
	}
	return "", nil, nil
}

func readJSON(file string, v any) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func readTOML(file string) (map[string]any, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, toml.Unmarshal(data, &m)
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
		if err != nil {
			m.skipped = append(m.skipped, file+": "+err.Error())
		}
		if err == nil && ecosystem != "" {
			if m.declared[ecosystem] == nil {
				m.declared[ecosystem] = map[string]bool{}
			}
			for _, n := range names {
				// Go matches a module path as a prefix, so its dashes stay.
				if ecosystem == "go" {
					m.declared[ecosystem][strings.ToLower(n)] = true
				} else {
					m.declared[ecosystem][packageKey(n)] = true
				}
			}
		}
		switch name {
		case "package.json":
			var pkg struct{ Name string }
			if readJSON(abs, &pkg) == nil && pkg.Name != "" {
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
	longestFirst := func(a, b pair) int { return cmp.Or(cmp.Compare(len(b.a), len(a.a)), strings.Compare(a.a, b.a)) }
	slices.SortStableFunc(m.goModules, longestFirst)
	slices.SortStableFunc(m.npmNames, longestFirst)
	return m
}

// addRustRoots adds a crate's target roots: [lib] and [[bin]]-style paths,
// and the auto-discovered target files.
func (m manifests) addRustRoots(manifest, crate string, files []string) {
	cargo, _ := readTOML(manifest)
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
	key := packageKey(externalName(spec, dotted))
	if ecosystem == "python" {
		return m.declared["python"][key] || slices.ContainsFunc(pythonDistributions[key], func(d string) bool { return m.declared["python"][d] })
	}
	return m.declared[ecosystem][key]
}

// pythonDistributions maps an import name to the distributions that
// install it, both as packageKey writes them. PyYAML installs yaml.
var pythonDistributions = map[string][]string{
	"yaml":     {"pyyaml"},
	"pil":      {"pillow"},
	"dateutil": {"python_dateutil"},
	"bs4":      {"beautifulsoup4"},
	"sklearn":  {"scikit_learn"},
	"cv2":      {"opencv_python", "opencv_python_headless", "opencv_contrib_python", "opencv_contrib_python_headless"},
}
