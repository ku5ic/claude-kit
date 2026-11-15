package codegraph

import (
	"cmp"
	"maps"
	"path"
	"slices"
	"strings"
)

// graph resolves import specs against a repo's tracked files.
type graph struct {
	known   map[string]string          // lowercased path, stem, or directory -> the real one
	parents map[string]map[string]bool // lowercased name -> its lowercased parent directories
	manifests
	rootRank map[string]int // source roots, best first
	roots    []string
	bare     map[bareKey]string
	// rootGo is the first non-test .go file at the repo root, where an import
	// of a root go.mod's own package lands.
	rootGo string
}

type bareKey struct {
	path      string
	kind      specKind
	ecosystem string
}

func newGraph(root string, files []string) *graph {
	g := &graph{known: map[string]string{}, parents: map[string]map[string]bool{}, bare: map[bareKey]string{}}
	addParent := func(name, parent string) {
		if g.parents[name] == nil {
			g.parents[name] = map[string]bool{}
		}
		g.parents[name][parent] = true
	}
	for _, file := range files {
		if g.rootGo == "" && !strings.Contains(file, "/") && strings.HasSuffix(file, ".go") && !strings.HasSuffix(file, "_test.go") {
			g.rootGo = file
		}
		stem := moduleStem(file)
		g.known[strings.ToLower(file)] = file
		g.known[strings.ToLower(stem)] = stem
		parts := strings.Split(file, "/")
		for depth := 1; depth < len(parts); depth++ {
			dir := strings.Join(parts[:depth], "/")
			g.known[strings.ToLower(dir)] = dir
			addParent(strings.ToLower(parts[depth-1]), strings.ToLower(strings.Join(parts[:depth-1], "/")))
		}
		addParent(strings.ToLower(path.Base(stem)), strings.ToLower(dirOf(stem)))
	}
	g.manifests = readManifests(root, files)
	return g
}

// longestExisting is the longest tracked path base/rel[:k], never shorter
// than one segment of rel; "" when none is.
func (g *graph) longestExisting(base, rel string) string {
	var segments []string
	for s := range strings.SplitSeq(rel, "/") {
		if s != "" {
			segments = append(segments, s)
		}
	}
	for k := len(segments); k > 0; k-- {
		if real, ok := g.known[strings.ToLower(path.Join(append([]string{base}, segments[:k]...)...))]; ok {
			return real
		}
	}
	return ""
}

// crateSrc is the src directory of the nearest Cargo crate above file.
func (g *graph) crateSrc(file string) string {
	for dir := dirOf(file); dir != ""; dir = dirOf(dir) {
		if _, ok := g.known[strings.ToLower(path.Join(dir, "Cargo.toml"))]; ok {
			return path.Join(dir, "src")
		}
	}
	return "src"
}

// normalize is s, from file, as a path, and how to resolve it.
func (g *graph) normalize(file, s string) spec {
	s, _, _ = strings.Cut(s, "?")
	lang := path.Ext(file)
	if m := pyRelative.FindStringSubmatch(s); m != nil && !strings.Contains(s, "/") {
		up := "./"
		if len(m[1]) > 1 {
			up = strings.Repeat("../", len(m[1])-1)
		}
		return spec{up + strings.ReplaceAll(m[2], ".", "/"), relative}
	}
	// A Go module path, or a workspace package by its package.json name.
	named := g.goModules
	if ecosystems[lang] == "npm" {
		named = g.npmNames
	}
	for _, mod := range named {
		if rest, ok := strings.CutPrefix(s, mod.a); ok && (rest == "" || strings.HasPrefix(rest, "/")) {
			return spec{path.Join(mod.b, rest), rooted}
		}
	}
	if lang == ".ex" || lang == ".exs" {
		s = snakeCase(s)
	}
	s = strings.TrimLeft(strings.NewReplacer("::", "/", `\`, "/").Replace(s), "/")
	head, tail, _ := strings.Cut(s, "/")
	if lib, ok := g.dartLibs[strings.ToLower(head)]; ok && lang == ".dart" {
		return spec{path.Join(lib, tail), rooted}
	}
	switch head {
	case "crate":
		return spec{path.Join(g.crateSrc(file), tail), rooted}
	case "self", "super":
		return g.rustRelative(file, head, tail)
	}
	// A dot in a JS spec is part of a package name (socket.io-client), never a separator.
	mayBeDotted := ecosystems[lang] != "npm" && (dottedLanguages[lang] || !sourceFile.MatchString(s))
	if mayBeDotted && !strings.Contains(s, "/") && strings.Contains(strings.Trim(s, "."), ".") {
		return spec{strings.ReplaceAll(s, ".", "/"), dotted}
	}
	return spec{s, plain}
}

// rustRelative resolves self:: and super:: from file. From src/net/client.rs,
// self is src/net/client; from a crate root or mod.rs, it's the directory.
func (g *graph) rustRelative(file, head, tail string) spec {
	base := strings.TrimSuffix(file, path.Ext(file))
	name := path.Base(base)
	crateRoot := name == "lib" || name == "main" || g.rustRoots[base]
	if crateRoot || name == "mod" {
		base = dirOf(base)
	}
	if crateRoot && head == "super" { // a root has no parent: its super is from an inline mod tests
		head = "self"
	}
	for head == "self" || head == "super" {
		if head == "super" {
			base = dirOf(base)
		}
		head, tail, _ = strings.Cut(tail, "/")
	}
	if head == "*" { // super::* names the module itself
		head = ""
	}
	if g.declared["rust"][packageKey(head)] { // extern crate clap puts clap at self::clap
		return spec{path.Join(head, tail), plain}
	}
	return spec{path.Join(base, head, tail), rooted}
}

// rankRoots finds the source roots: directories whose children are named
// by the first segment of bare specs. A real root (src, src/main/java)
// explains several distinct segments; a folder that merely shares a
// package's name explains one.
func (g *graph) rankRoots(specs []fileSpec) {
	votes, segments := map[string]int{}, map[string]map[string]bool{}
	for _, fs := range specs {
		// A Go spec never resolves through a root, so fmt mustn't vote for docs/fmt.md.
		if fs.kind != dotted && fs.kind != plain || ecosystems[path.Ext(fs.file)] == "go" {
			continue
		}
		head, _, _ := strings.Cut(fs.path, "/")
		head = strings.ToLower(head)
		for parent := range g.parents[head] {
			votes[parent]++
			if segments[parent] == nil {
				segments[parent] = map[string]bool{}
			}
			segments[parent][head] = true
		}
	}
	g.roots = slices.SortedFunc(maps.Keys(votes), func(a, b string) int {
		return cmp.Or(cmp.Compare(len(segments[b]), len(segments[a])), cmp.Compare(votes[b], votes[a]), strings.Compare(a, b))
	})
	g.rootRank = map[string]int{}
	for i, r := range g.roots {
		g.rootRank[r] = i
	}
}

// resolve is the repo path an import points at; "" for an external
// package. ok is false for a relative import nothing tracked matches: a
// generated file, or a path out of the repo.
func (g *graph) resolve(file string, s spec) (target string, ok bool) {
	dir := dirOf(file)
	switch {
	case strings.HasPrefix(s.path, "."):
		return g.resolveRelative(file, path.Join(dir, s.path))
	case s.kind == rooted && ecosystems[path.Ext(file)] == "go":
		// A Go import names its package's directory, never an item below it.
		dir := strings.ToLower(strings.Trim(s.path, "/"))
		if dir == "" {
			// A root go.mod's package is the repo root, which no tracked path names.
			return g.rootGo, g.rootGo != ""
		}
		real, ok := g.known[dir]
		return real, ok
	case s.kind == rooted:
		return cmp.Or(g.longestExisting("", s.path), s.path), true
	}
	ecosystem := ecosystems[path.Ext(file)]
	if ecosystem == "go" {
		return "", true // Go's own packages sit under a go.mod module path, matched as rooted
	}
	if sourceFile.MatchString(s.path) {
		if hit := g.longestExisting(dir, s.path); hit != "" {
			return hit, true
		}
	}
	return g.resolveBare(s, ecosystem), true
}

// resolveRelative is the tracked path joined names, from file. A .js spec
// may name its .ts source; a Python from x import name names a member of
// x, so x itself is tried too.
func (g *graph) resolveRelative(file, joined string) (string, bool) {
	lower := strings.ToLower(joined)
	if real, ok := g.known[lower]; ok {
		return real, true
	}
	if real, ok := g.known[strings.TrimSuffix(lower, path.Ext(lower))]; ok && jsEmitted[path.Ext(lower)] {
		return real, true
	}
	if real, ok := g.known[dirOf(lower)]; ok && ecosystems[path.Ext(file)] == "python" {
		return real, true
	}
	return "", false
}

// resolveBare resolves a bare spec against the source roots, cached: the
// same react or fmt repeats thousands of times.
func (g *graph) resolveBare(s spec, ecosystem string) string {
	key := bareKey{s.path, s.kind, ecosystem}
	if hit, ok := g.bare[key]; ok {
		return hit
	}
	head, _, _ := strings.Cut(s.path, "/")
	head = strings.ToLower(head)
	best := ""
	// A stdlib name is tried in the main root first: the repo's own types
	// package shadows the stdlib one, as it does on Python's path. Other
	// roots' folders sharing the name don't.
	stdlib := ecosystem == "python" && pythonStdlib[head]
	if !g.isDeclared(s.path, s.kind == dotted, ecosystem) {
		var candidates []string
		for parent := range g.parents[head] {
			if rank, ok := g.rootRank[parent]; ok && (!stdlib || rank == 0) {
				candidates = append(candidates, parent)
			}
		}
		slices.SortFunc(candidates, func(a, b string) int { return cmp.Compare(g.rootRank[a], g.rootRank[b]) })
		// The deepest match wins (lua/net/http over src/net for net.http); a
		// tie goes to the higher-ranked root.
		bestDepth := 0
		for _, sourceRoot := range candidates {
			base := cmp.Or(g.known[sourceRoot], sourceRoot)
			hit := g.longestExisting(base, s.path)
			if hit == "" {
				continue
			}
			depth := strings.Count(hit, "/") + 1
			if base != "" {
				depth -= strings.Count(base, "/") + 1
			}
			// Past the main root, a one-segment match is a folder sharing a
			// package's name (src/components/form/graphql vs graphql).
			if (g.rootRank[sourceRoot] == 0 || depth >= 2) && depth > bestDepth {
				best, bestDepth = hit, depth
			}
		}
	}
	g.bare[key] = best
	return best
}

// fileSpec is one spec and the file that imports it.
type fileSpec struct {
	file string
	spec
}

// specs is every import spec of rows, normalized against the file that
// imports it.
func (g *graph) specs(rows [][]string) []fileSpec {
	var specs []fileSpec
	for _, r := range rows {
		for _, raw := range rawSpecs(r[1]) {
			s := g.normalize(r[0], raw.path)
			if raw.kind == relative && !strings.HasPrefix(s.path, ".") {
				s.path = "./" + s.path
			}
			specs = append(specs, fileSpec{r[0], s})
		}
	}
	return specs
}
