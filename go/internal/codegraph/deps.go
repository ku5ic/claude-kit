package codegraph

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/git"
)

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
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	g := newGraph(root, files)
	split = splitDir(split, rows)
	specs := g.specs(rows)
	g.rankRoots(specs)
	if err := build(specs, g, newModules(split, g.known)).write(outDir); err != nil {
		return err
	}
	mainRoot := "none"
	if len(g.roots) > 0 {
		mainRoot = cmp.Or(g.roots[0], "(repo root)")
	}
	for _, skipped := range g.skipped {
		fmt.Fprintln(w, "skipped manifest:", skipped)
	}
	fmt.Fprintf(w, "split dir: %s; main source root: %s; other candidates: %d\n", cmp.Or(split, "(repo root)"), mainRoot, max(0, len(g.roots)-1))
	return nil
}

// splitDir is split cleaned, or, when empty, the split found among the
// importing files of rows.
func splitDir(split string, rows [][]string) string {
	if split != "" {
		split = strings.Trim(path.Clean(split), "/") // src/ and ./src name the same dir
		if split == "." {
			split = ""
		}
		return split
	}
	var importing []string
	for _, r := range rows {
		importing = append(importing, r[0])
	}
	slices.Sort(importing)
	return findSplit(slices.Compact(importing))
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
}

// build resolves each spec and counts it: an edge between the modules of
// its two ends, or an external.
func build(specs []fileSpec, g *graph, mods modules) depsResult {
	r := depsResult{edges: map[pair]int{}, prodFiles: map[pair]map[string]int{}, externals: map[string]int{}, declared: map[string]bool{}}
	for _, fs := range specs {
		target := g.resolve(fs.file, fs.spec)
		if target == "" {
			name := externalName(fs.path, fs.kind == dotted)
			r.externals[name]++
			if g.isDeclared(fs.path, fs.kind == dotted, ecosystems[path.Ext(fs.file)]) {
				r.declared[name] = true
			}
			continue
		}
		r.resolved = append(r.resolved, pair{fs.file, target})
		source, dest := mods.of(fs.file), mods.of(target)
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

// write writes r's four TSVs to outDir.
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
	return cmp.Or(err, writeTSV(filepath.Join(outDir, "imports-resolved.tsv"), func(out io.Writer) {
		for _, p := range r.resolved {
			fmt.Fprintf(out, "%s\t%s\n", p.a, p.b)
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
