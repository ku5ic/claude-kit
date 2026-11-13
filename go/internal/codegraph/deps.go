package codegraph

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/ku5ic/claude-kit/go/internal/git"
)

// An import's text is a bare spec ("react", "./x") or a whole statement
// ("use crate::a::b;", "#include <x.h>", "import static a.B.C;").
var (
	keyword = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?` +
		`(#\s*include|import|using|use|alias|require_relative|require|from|mod|extern\s+crate)(\s+|[("'<])`)
	staticWord  = regexp.MustCompile(`^static\s+`)
	specEnd     = regexp.MustCompile(`\s+as\s+|[;,\s()]`)
	csharpAlias = regexp.MustCompile(`^\w+\s*=`)
	pyRelative  = regexp.MustCompile(`^(\.+)([^./].*)?$`)
	sourceFile  = regexp.MustCompile(`\.(h|hh|hpp|hxx|inc|c|cc|cpp|js|jsx|mjs|cjs|ts|tsx|json|css|scss|sass|less|vue|svelte` +
		`|rb|py|go|rs|php|dart|lua|graphql|gql|html|svg|png|md|ya?ml|wasm)$`)
)

var (
	domainRoots   = set("com", "org", "net", "io", "dev", "edu", "gov", "co")
	platformRoots = set("java", "javax", "jakarta", "kotlin", "kotlinx", "android", "androidx", "scala", "System", "Microsoft")
	// Languages whose specs are always dotted names, so a.b.json is a module, not a file.
	dottedLanguages = set(".py", ".java", ".kt", ".kts", ".cs", ".scala", ".groovy", ".lua", ".ex", ".exs")
	// NodeNext imports name the emitted file: ./a.js is a.ts.
	jsEmitted  = set(".js", ".jsx", ".mjs", ".cjs")
	ecosystems = map[string]string{
		".js": "npm", ".jsx": "npm", ".mjs": "npm", ".cjs": "npm", ".ts": "npm", ".tsx": "npm", ".mts": "npm", ".cts": "npm",
		".vue": "npm", ".svelte": "npm", ".py": "python", ".rs": "rust", ".rb": "ruby", ".go": "go",
	}
)

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}

// spec is one import target, path-like, and how it resolves.
type spec struct {
	path string
	kind specKind
}

type specKind int

const (
	plain    specKind = iota // a bare name: a package, or a path under a source root
	dotted                   // a.b.c, a module path in a dotted language
	rooted                   // a path from the repo root
	relative                 // a path from the importing file
)

// rawSpecs is the specs in an import's text, each marked when the
// language makes it file-relative (require_relative, #include "x").
func rawSpecs(text string) []spec {
	text = strings.TrimSpace(text)
	kw := ""
	if m := keyword.FindStringSubmatchIndex(text); m != nil {
		kw = text[m[2]:m[3]]
		end := m[1]
		if !unicode.IsSpace(rune(text[m[4]])) {
			end = m[4] // the quote or bracket opens the spec
		}
		rest := staticWord.ReplaceAllString(text[end:], "")
		// The ( of require("x") would otherwise cut the spec down to nothing.
		text = strings.TrimLeft(rest, "( ")
	}
	if kw == "mod" { // Rust mod x; names its file the way self::x does
		text = "self::" + text
	}
	if csharpAlias.MatchString(text) { // using Json = Newtonsoft.Json;
		_, text, _ = strings.Cut(text, "=")
		text = strings.TrimSpace(text)
	}
	quoted := text != "" && strings.ContainsAny(text[:1], `"'`)
	isRelative := kw == "require_relative" || strings.HasPrefix(kw, "#") && quoted
	var raw []string
	if kw == "from" && strings.Contains(text, " import ") {
		raw = pythonFrom(text)
	} else {
		raw = expandGroups(text)
	}
	kind := plain
	if isRelative {
		kind = relative
	}
	var out []spec
	for _, r := range raw {
		if loc := specEnd.FindStringIndex(r); loc != nil {
			r = r[:loc[0]]
		}
		r = strings.TrimPrefix(strings.Trim(r, `"'<>`), "package:")
		if r != "" {
			out = append(out, spec{r, kind})
		}
	}
	return out
}

// expandGroups is a::{b, c::{d, e}} as a::b, a::c::d, a::c::e; text with
// no group as is.
func expandGroups(text string) []string {
	start := strings.Index(text, "{")
	if start < 0 {
		return []string{text}
	}
	var items []string
	depth, itemStart := 0, start+1
scan:
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case ',':
			if depth == 1 {
				items = append(items, text[itemStart:i])
				itemStart = i + 1
			}
		case '}':
			depth--
			if depth == 0 {
				items = append(items, text[itemStart:i])
				break scan
			}
		}
	}
	var out []string
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, expandGroups(text[:start]+item)...)
		}
	}
	return out
}

// pythonFrom is "pkg import a, b as c" as pkg.a and pkg.b, and ". import x"
// as .x. A name that is no module falls back to its package later.
func pythonFrom(text string) []string {
	pkg, names, _ := strings.Cut(text, " import ")
	pkg = strings.TrimSpace(pkg)
	joiner := "."
	if strings.HasSuffix(pkg, ".") {
		joiner = ""
	}
	var out []string
	for name := range strings.SplitSeq(strings.Trim(strings.TrimSpace(names), "() "), ",") {
		name, _, _ = strings.Cut(name, " as ")
		switch name = strings.TrimSpace(name); name {
		case "":
		case "*":
			out = append(out, pkg)
		default:
			out = append(out, pkg+joiner+name)
		}
	}
	return out
}

// snakeCase is an Elixir module as its file name: MyApp.HTTPClient is
// my_app.http_client.
func snakeCase(name string) string {
	var b strings.Builder
	for i, r := range name {
		if i > 0 && unicode.IsUpper(r) {
			prev := rune(name[i-1])
			next := rune(0)
			if i+1 < len(name) {
				next = rune(name[i+1])
			}
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || unicode.IsUpper(prev) && unicode.IsLower(next) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// externalName is the package an unresolved spec names: @scope/name, a
// platform's two segments, a domain-rooted three, else its first.
func externalName(p string, isDotted bool) string {
	var parts []string
	for part := range strings.SplitSeq(p, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return p
	}
	n := 1
	switch {
	case strings.HasPrefix(p, "@") || isDotted && platformRoots[parts[0]]:
		n = 2
	case isDotted && domainRoots[parts[0]] || !isDotted && strings.Contains(parts[0], "."):
		n = 3
	}
	sep := "/"
	if isDotted {
		sep = "."
	}
	return strings.Join(parts[:min(n, len(parts))], sep)
}

// graph resolves import specs against a repo's tracked files.
type graph struct {
	known   map[string]string          // lowercased path, stem, or directory -> the real one
	parents map[string]map[string]bool // lowercased name -> its lowercased parent directories
	manifests
	rootRank map[string]int // source roots, best first
	roots    []string
	bare     map[bareKey]string
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
	for _, mod := range g.goModules {
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
		if fs.kind != dotted && fs.kind != plain {
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

// resolve is the repo path an import points at; "" for an external package.
func (g *graph) resolve(file string, s spec) string {
	dir := dirOf(file)
	switch {
	case strings.HasPrefix(s.path, "."):
		joined := path.Join(dir, s.path)
		lower := strings.ToLower(joined)
		stem := strings.TrimSuffix(lower, path.Ext(lower))
		if _, ok := g.known[lower]; !ok && jsEmitted[path.Ext(lower)] {
			if real, ok := g.known[stem]; ok {
				return real
			}
		}
		return cmp.Or(g.longestExisting("", joined), joined)
	case s.kind == rooted:
		return cmp.Or(g.longestExisting("", s.path), s.path)
	}
	ecosystem := ecosystems[path.Ext(file)]
	if ecosystem == "go" {
		return "" // Go's own packages sit under a go.mod module path, matched as rooted
	}
	if sourceFile.MatchString(s.path) {
		if hit := g.longestExisting(dir, s.path); hit != "" {
			return hit
		}
	}
	return g.resolveBare(s, ecosystem)
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
	external := g.isDeclared(s.path, s.kind == dotted, ecosystem) || ecosystem == "python" && pythonStdlib[head]
	if !external {
		var candidates []string
		for parent := range g.parents[head] {
			if _, ok := g.rootRank[parent]; ok {
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

// findSplit descends while one directory holds over 80% of files.
func findSplit(files []string) string {
	prefix := ""
	for {
		children, inside := map[string]int{}, 0
		for _, f := range files {
			rest, ok := strings.CutPrefix(f, prefix)
			if !ok {
				continue
			}
			inside++
			if child, _, nested := strings.Cut(rest, "/"); nested {
				children[child]++
			}
		}
		if len(children) == 0 {
			return strings.TrimSuffix(prefix, "/")
		}
		top := byCount(children, strings.Compare)[0]
		if float64(children[top])/float64(inside) <= 0.8 {
			return strings.TrimSuffix(prefix, "/")
		}
		prefix += top + "/"
	}
}

// modules maps a path to its module: a child of the split directory, or
// a top-level directory outside it; each file when the split holds only
// files.
type modules struct {
	split    string
	children map[string]bool
}

func newModules(split string, known map[string]string) modules {
	m := modules{split: split, children: map[string]bool{}}
	for _, p := range known {
		if rest, ok := strings.CutPrefix(p, split+"/"); ok && split != "" {
			if child, _, nested := strings.Cut(rest, "/"); nested {
				m.children[child] = true
			}
		}
	}
	return m
}

func (m modules) of(p string) string {
	inSplit := m.split != "" && strings.HasPrefix(p, m.split+"/")
	top, _, _ := strings.Cut(p, "/")
	switch {
	case m.split != "" && len(m.children) == 0:
		if p == m.split {
			return "" // the split itself is none of its file modules
		}
		if inSplit {
			return moduleStem(p)
		}
		return top
	case inSplit:
		child, _, _ := strings.Cut(strings.TrimPrefix(p, m.split+"/"), "/")
		if m.children[child] {
			return m.split + "/" + child
		}
		return m.split
	}
	return top
}

// Deps reads imports (path<TAB>import text, as Imports writes) under root
// and writes deps.tsv, externals.tsv, upward.tsv, and imports-resolved.tsv
// to outDir. split overrides the found split directory. It prints the
// split and the main source root to w.
func Deps(root, imports, outDir, split string, w io.Writer) error {
	files, err := git.Paths(root, "ls-files", "-z")
	if err != nil {
		return fmt.Errorf("git ls-files: %w", err)
	}
	rows, err := readTSV(imports, 2)
	if err != nil {
		return err
	}
	g := newGraph(root, files)
	if split != "" {
		split = strings.Trim(path.Clean(split), "/") // src/ and ./src name the same dir
		if split == "." {
			split = ""
		}
	} else {
		var importing []string
		for _, r := range rows {
			importing = append(importing, r[0])
		}
		slices.Sort(importing)
		split = findSplit(slices.Compact(importing))
	}
	mods := newModules(split, g.known)

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
	g.rankRoots(specs)

	edges, prodFiles := map[pair]int{}, map[pair]map[string]int{}
	externals, declaredExternal := map[string]int{}, map[string]bool{}
	var resolved []pair
	for _, fs := range specs {
		target := g.resolve(fs.file, fs.spec)
		if target == "" {
			name := externalName(fs.path, fs.kind == dotted)
			externals[name]++
			if g.isDeclared(fs.path, fs.kind == dotted, ecosystems[path.Ext(fs.file)]) {
				declaredExternal[name] = true
			}
			continue
		}
		resolved = append(resolved, pair{fs.file, target})
		source, dest := mods.of(fs.file), mods.of(target)
		if source == "" || dest == "" || source == dest {
			continue
		}
		e := pair{source, dest}
		edges[e]++
		if !testOrStory.MatchString(fs.file) {
			if prodFiles[e] == nil {
				prodFiles[e] = map[string]int{}
			}
			prodFiles[e][fs.file]++
		}
	}

	ordered := byCount(edges, pair.compare)
	err = writeTSV(filepath.Join(outDir, "deps.tsv"), func(out io.Writer) {
		for _, e := range ordered {
			fmt.Fprintf(out, "%d\t%s\t%s\n", edges[e], e.a, e.b)
		}
	})
	err = cmp.Or(err, writeTSV(filepath.Join(outDir, "externals.tsv"), func(out io.Writer) {
		for _, name := range byCount(externals, strings.Compare) {
			state := "undeclared"
			if declaredExternal[name] {
				state = "declared"
			}
			fmt.Fprintf(out, "%d\t%s\t%s\n", externals[name], name, state)
		}
	}))
	// An edge is upward when the reverse direction carries more production
	// imports. Tests and stories don't vote: they often import upward legitimately.
	err = cmp.Or(err, writeTSV(filepath.Join(outDir, "upward.tsv"), func(out io.Writer) {
		fmt.Fprintln(out, "imports\tprod_imports\tprod_files\tsource\ttarget\ttop_prod_files")
		for _, e := range ordered {
			files := prodFiles[e]
			prod := sum(files)
			if prod == 0 || sum(prodFiles[pair{e.b, e.a}]) <= prod {
				continue
			}
			top := byCount(files, strings.Compare)
			fmt.Fprintf(out, "%d\t%d\t%d\t%s\t%s\t%s\n", edges[e], prod, len(files), e.a, e.b, strings.Join(top[:min(5, len(top))], ","))
		}
	}))
	err = cmp.Or(err, writeTSV(filepath.Join(outDir, "imports-resolved.tsv"), func(out io.Writer) {
		for _, r := range resolved {
			fmt.Fprintf(out, "%s\t%s\n", r.a, r.b)
		}
	}))
	if err != nil {
		return err
	}
	mainRoot := "none"
	if len(g.roots) > 0 {
		mainRoot = cmp.Or(g.roots[0], "(repo root)")
	}
	fmt.Fprintf(w, "split dir: %s; main source root: %s; other candidates: %d\n", cmp.Or(split, "(repo root)"), mainRoot, max(0, len(g.roots)-1))
	return nil
}

func sum(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}

// writeTSV writes file with fill.
func writeTSV(file string, fill func(io.Writer)) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fill(w)
	return cmp.Or(w.Flush(), f.Close())
}
