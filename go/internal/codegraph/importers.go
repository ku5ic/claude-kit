package codegraph

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/git"
)

// JSExtensions and TSExtensions are each language's file extensions.
var (
	JSExtensions = []string{"js", "jsx", "mjs", "cjs"}
	TSExtensions = []string{"ts", "tsx", "mts", "cts"}
)

// scanned is the extensions ast-grep reads, by its defaults, as one of
// importRules' languages.
var scanned = set(".js", ".jsx", ".mjs", ".cjs", ".ts", ".mts", ".cts", ".tsx", ".vue", ".svelte", ".astro", ".py", ".go", ".rs",
	".java", ".swift", ".scala", ".sc", ".kt", ".kts", ".cs", ".php", ".c", ".h", ".cc", ".cpp", ".cxx",
	".hpp", ".hh", ".hxx", ".rb", ".lua", ".dart", ".ex", ".exs")

// families groups the extensions whose files import each other, past
// ecosystems: a .java file imports a .kt one, a .cpp file a .h one.
var families = map[string]string{
	".java": "jvm", ".kt": "jvm", ".kts": "jvm", ".scala": "jvm", ".sc": "jvm", ".groovy": "jvm",
	".c": "c", ".h": "c", ".cc": "c", ".cpp": "c", ".cxx": "c", ".hpp": "c", ".hh": "c", ".hxx": "c",
	".ex": "elixir", ".exs": "elixir",
}

// family is the import family of p's extension: its ecosystem, else its
// families group, else the extension itself.
func family(p string) string {
	ext := path.Ext(p)
	if e := ecosystems[ext]; e != "" {
		return e
	}
	if f := families[ext]; f != "" {
		return f
	}
	return ext
}

// Scans reports whether Importers reads file's language.
func Scans(file string) bool { return scanned[path.Ext(file)] }

// Importer is a file that uses the file asked about, the line where it
// does, and how it was matched when that isn't a resolved import:
// alias-match, workspace-match, or same-package.
type Importer struct {
	File  string
	Line  int
	Label string
}

// Uses is who uses a file: its importers, and whether a JS file anywhere
// imports through a non-literal import() or require(), which no scan can
// resolve.
type Uses struct {
	Importers    []Importer
	Unresolvable bool
}

// jsDynamic is an import() or require() whose argument isn't a string literal.
var jsDynamic = regexp.MustCompilePOSIX(`(^|[^A-Za-z0-9_$.])(import|require)[[:space:]]*\([[:space:]]*[^"'[:space:])]`)

// Importers is who uses file (relative to root), by the resolution Deps
// uses: each import that names a Go file's package or any other file's
// module. Also matched, as Deps can't:
//   - an "@/" or "~/" alias whose path ends in the file's module
//     (alias-match);
//   - a workspace package's name, or name/subpath, for every file in that
//     package (workspace-match);
//   - a Go file's package's other files, which use it without an import
//     (same-package), at their package clause.
//
// Untracked files that git doesn't ignore count, as they would in a blast
// radius.
func Importers(root, file string, skipDirs []string, stderr io.Writer) (Uses, error) {
	imports, err := scanImports(root, skipDirs, stderr)
	if err != nil {
		return Uses{}, err
	}
	files, err := git.Files(root)
	if err != nil {
		return Uses{}, fmt.Errorf("git ls-files: %w", err)
	}
	files = slices.DeleteFunc(files, func(f string) bool { return skipped(f, skipDirs) })
	tracked := map[string]bool{}
	for _, f := range files {
		tracked[f] = true
	}
	g := newGraph(root, files)
	specs := g.specsOf(imports)
	g.rankRoots(specs)
	// By line, so a file importing the target twice is listed at its first.
	slices.SortStableFunc(specs, func(a, b fileSpec) int {
		return cmp.Or(strings.Compare(a.file, b.file), cmp.Compare(a.line, b.line))
	})
	t := newTarget(g, file)
	var uses Uses
	for _, fs := range specs {
		n := len(uses.Importers)
		if fs.file == file || n > 0 && uses.Importers[n-1].File == fs.file {
			continue
		}
		if label, ok := t.match(g, fs, tracked, skipDirs); ok {
			uses.Importers = append(uses.Importers, Importer{fs.file, fs.line, label})
		}
	}
	if path.Ext(file) == ".go" {
		uses.Importers = append(uses.Importers, samePackage(root, file, files)...)
	}
	if ecosystems[path.Ext(file)] == "npm" {
		uses.Unresolvable = anyLine(root, files, jsDynamic, func(f string) bool { return ecosystems[path.Ext(f)] == "npm" })
	}
	return uses, nil
}

// target is the file asked about, as each kind of import names it.
type target struct {
	file      string
	id        string   // as importTarget names it
	stem      string   // the path without its extension
	workspace []string // the workspace package dirs holding it
}

func newTarget(g *graph, file string) target {
	t := target{file: file, id: importTarget(file), stem: strings.TrimSuffix(file, path.Ext(file))}
	for _, mod := range g.npmNames {
		if mod.b != "" && strings.HasPrefix(file, mod.b+"/") {
			t.workspace = append(t.workspace, mod.b)
		}
	}
	return t
}

// match is whether fs reaches t, and the label saying how. Only a file
// of t's import family can, so a TS import of "lib" never reaches lib/f.go.
func (t target) match(g *graph, fs fileSpec, tracked map[string]bool, skipDirs []string) (label string, ok bool) {
	if family(fs.file) != family(t.file) {
		return "", false
	}
	npm := ecosystems[path.Ext(fs.file)] == "npm"
	if rest, alias := cutAlias(fs.path); npm && alias {
		if jsEmitted[path.Ext(rest)] || slices.Contains(TSExtensions, strings.TrimPrefix(path.Ext(rest), ".")) {
			rest = strings.TrimSuffix(rest, path.Ext(rest))
		}
		return "alias-match", endsInPath(t.id, rest) || endsInPath(t.stem, rest)
	}
	if npm && fs.kind == rooted {
		for _, dir := range t.workspace {
			if fs.path == dir || strings.HasPrefix(fs.path, dir+"/") {
				return "workspace-match", true
			}
		}
	}
	resolved, ok := g.resolve(fs.file, fs.spec)
	// A tracked file of another family (./a.css beside a.ts) shares the
	// target's stem but isn't it.
	if !ok || resolved == "" || skipped(resolved, skipDirs) || tracked[resolved] && family(resolved) != family(t.file) {
		return "", false
	}
	id := targetID(fs.file, resolved, tracked)
	// Importing app.models.user runs app/models/__init__.py first.
	if family(fs.file) == "python" && strings.HasPrefix(id, t.id+"/") {
		return "", true
	}
	return "", id == t.id
}

// cutAlias is p without an "@/" or "~/" prefix, and whether it had one.
func cutAlias(p string) (string, bool) {
	for _, prefix := range []string{"@/", "~/"} {
		if rest, ok := strings.CutPrefix(p, prefix); ok {
			return rest, true
		}
	}
	return p, false
}

// endsInPath is true when p is suffix or ends in "/"+suffix.
func endsInPath(p, suffix string) bool {
	return p == suffix || strings.HasSuffix(p, "/"+suffix)
}

// targetID is how importTarget names target, a resolved import of
// importer's: a Go import's package directory, the root package's being
// "", and any other import's module.
func targetID(importer, target string, tracked map[string]bool) string {
	switch {
	case path.Ext(importer) != ".go":
		return moduleID(target, tracked)
	case path.Ext(target) == ".go":
		return dirOf(target)
	}
	return target
}

var goPackageClause = regexp.MustCompile(`^package[[:space:]]`)

// samePackage is the other .go files in file's directory, each at its
// package clause.
func samePackage(root, file string, files []string) []Importer {
	var out []Importer
	for _, f := range files {
		if f == file || path.Ext(f) != ".go" || dirOf(f) != dirOf(file) {
			continue
		}
		if n := firstLine(filepath.Join(root, f), goPackageClause); n > 0 {
			out = append(out, Importer{f, n, "same-package"})
		}
	}
	return out
}

// anyLine reports whether a line of any of files that keep holds re.
func anyLine(root string, files []string, re *regexp.Regexp, keep func(string) bool) bool {
	return slices.ContainsFunc(files, func(f string) bool {
		return keep(f) && firstLine(filepath.Join(root, f), re) > 0
	})
}

// firstLine is the 1-based number of file's first line matching re, 0 for none.
func firstLine(file string, re *regexp.Regexp) int {
	data, err := os.ReadFile(file)
	if err != nil || git.IsBinary(data) {
		return 0
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for n := 1; sc.Scan(); n++ {
		if re.MatchString(sc.Text()) {
			return n
		}
	}
	return 0
}
