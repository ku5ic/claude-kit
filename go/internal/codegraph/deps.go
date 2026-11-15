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
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/git"
)

// Deps reads imports (path<TAB>import text, as Imports writes) under root
// and writes deps.tsv, externals.tsv, upward.tsv, imports-resolved.tsv,
// and modules.txt to outDir. split overrides the found split directory.
// It prints the split and the main source root to w. A path under any of
// skipDirs is no module.
func Deps(root, imports, outDir, split string, skipDirs []string, w io.Writer) error {
	files, err := git.Paths(root, "ls-files", "-z")
	if err != nil {
		return fmt.Errorf("git ls-files: %w", err)
	}
	rows, err := readTSV(imports, 2)
	if err != nil {
		return err
	}
	sources := sourceFiles(files, rows)
	dropped := skippedDirs(sources, skipDirs)
	files = slices.DeleteFunc(files, func(f string) bool { return skipped(f, skipDirs) })
	sources = slices.DeleteFunc(sources, func(f string) bool { return skipped(f, skipDirs) })
	rows = slices.DeleteFunc(rows, func(r []string) bool { return skipped(r[0], skipDirs) })
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	g := newGraph(root, files)
	split = splitDir(split, sources)
	specs := g.specs(rows)
	g.rankRoots(specs)
	if err := build(specs, g, newModules(split, sources), skipDirs).write(outDir); err != nil {
		return err
	}
	mainRoot := "none"
	if len(g.roots) > 0 {
		mainRoot = cmp.Or(g.roots[0], "(repo root)")
	}
	for _, skipped := range g.skipped {
		fmt.Fprintln(w, "skipped manifest:", skipped)
	}
	for _, dir := range dropped {
		fmt.Fprintf(w, "skipped dir: %s (skip_dirs)\n", dir)
	}
	fmt.Fprintf(w, "split dir: %s; main source root: %s; other candidates: %d\n", cmp.Or(split, "(repo root)"), mainRoot, max(0, len(g.roots)-1))
	return nil
}

// splitDir is split cleaned, or, when empty, the split found among
// sources.
func splitDir(split string, sources []string) string {
	if split != "" {
		split = strings.Trim(path.Clean(split), "/") // src/ and ./src name the same dir
		if split == "." {
			split = ""
		}
		return split
	}
	return findSplit(sources)
}

// sourceFiles is each file in files, sorted, with the extension of a file
// rows import from. A leaf that imports nothing counts toward the split
// like the files that do.
func sourceFiles(files []string, rows [][]string) []string {
	exts := map[string]bool{}
	for _, r := range rows {
		if ext := path.Ext(r[0]); ext != "" {
			exts[ext] = true
		}
	}
	var sources []string
	for _, f := range files {
		if exts[path.Ext(f)] {
			sources = append(sources, f)
		}
	}
	slices.Sort(sources)
	return slices.Compact(sources)
}

// skippedDirs is each directory a skipDirs entry drops that holds one of
// sources, sorted. build/ or out/ can be real code, so the drop is named.
func skippedDirs(sources, skipDirs []string) []string {
	var dirs []string
	for _, f := range sources {
		parts := strings.Split(f, "/")
		for i, dir := range parts[:len(parts)-1] {
			if slices.Contains(skipDirs, dir) {
				dirs = append(dirs, strings.Join(parts[:i+1], "/"))
				break
			}
		}
	}
	slices.Sort(dirs)
	return slices.Compact(dirs)
}

// depsResult is what the specs resolved to: module edges with their
// import counts and production files, externals by name, and each
// resolved file pair.
type depsResult struct {
	edges     map[pair]int
	prodFiles map[pair]map[string]int
	externals map[string]int
	declared  map[string]bool
	resolved  []pair
	modules   map[string]bool
}

// build resolves each spec and counts it: an edge between the modules of
// its two ends, or an external. A spec resolving under skipDirs counts as
// neither.
func build(specs []fileSpec, g *graph, mods modules, skipDirs []string) depsResult {
	r := depsResult{edges: map[pair]int{}, prodFiles: map[pair]map[string]int{}, externals: map[string]int{}, declared: map[string]bool{}, modules: map[string]bool{}}
	for _, fs := range specs {
		r.addModule(mods.of(fs.file))
		target, ok := g.resolve(fs.file, fs.spec)
		if !ok {
			continue
		}
		if target == "" {
			name := externalName(fs.path, fs.kind == dotted)
			r.externals[name]++
			if g.isDeclared(fs.path, fs.kind == dotted, ecosystems[path.Ext(fs.file)]) {
				r.declared[name] = true
			}
			continue
		}
		if skipped(target, skipDirs) {
			continue
		}
		r.resolved = append(r.resolved, pair{fs.file, target})
		source, dest := mods.of(fs.file), mods.of(target)
		r.addModule(dest)
		if source == "" || dest == "" || source == dest {
			continue
		}
		e := pair{source, dest}
		r.edges[e]++
		if !testOrStory.MatchString(fs.file) {
			if r.prodFiles[e] == nil {
				r.prodFiles[e] = map[string]int{}
			}
			r.prodFiles[e][fs.file]++
		}
	}
	return r
}

// addModule records m, unless it's "", the split itself.
func (r depsResult) addModule(m string) {
	if m != "" {
		r.modules[m] = true
	}
}

// write writes r's four TSVs and modules.txt to outDir.
func (r depsResult) write(outDir string) error {
	ordered := byCount(r.edges, pair.compare)
	err := writeTSV(filepath.Join(outDir, "deps.tsv"), func(out io.Writer) {
		for _, e := range ordered {
			fmt.Fprintf(out, "%d\t%s\t%s\n", r.edges[e], e.a, e.b)
		}
	})
	err = cmp.Or(err, writeTSV(filepath.Join(outDir, "externals.tsv"), func(out io.Writer) {
		for _, name := range byCount(r.externals, strings.Compare) {
			state := "undeclared"
			if r.declared[name] {
				state = "declared"
			}
			fmt.Fprintf(out, "%d\t%s\t%s\n", r.externals[name], name, state)
		}
	}))
	// An edge is upward when the reverse direction carries more production
	// imports. Tests and stories don't vote: they often import upward legitimately.
	err = cmp.Or(err, writeTSV(filepath.Join(outDir, "upward.tsv"), func(out io.Writer) {
		fmt.Fprintln(out, "imports\tprod_imports\tprod_files\tsource\ttarget\ttop_prod_files")
		for _, e := range ordered {
			files := r.prodFiles[e]
			prod := sum(files)
			if prod == 0 || sum(r.prodFiles[pair{e.b, e.a}]) <= prod {
				continue
			}
			top := byCount(files, strings.Compare)
			fmt.Fprintf(out, "%d\t%d\t%d\t%s\t%s\t%s\n", r.edges[e], prod, len(files), e.a, e.b, strings.Join(top[:min(5, len(top))], ","))
		}
	}))
	err = cmp.Or(err, writeTSV(filepath.Join(outDir, "imports-resolved.tsv"), func(out io.Writer) {
		for _, p := range r.resolved {
			fmt.Fprintf(out, "%s\t%s\n", p.a, p.b)
		}
	}))
	return cmp.Or(err, writeTSV(filepath.Join(outDir, "modules.txt"), func(out io.Writer) {
		for _, m := range slices.Sorted(maps.Keys(r.modules)) {
			fmt.Fprintln(out, m)
		}
	}))
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
