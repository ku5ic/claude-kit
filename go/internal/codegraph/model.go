package codegraph

import (
	"bufio"
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/git"
)

// ModelDir is where the architecture model of the repo holding target
// lives: base/<parent>/<project>, where project is the main checkout's
// folder name and parent the folder holding it. Two clones in one parent
// can't share a folder name, so they never share a model; a worktree
// shares its main checkout's. A target below the root gets
// base/<parent>/<project>/scopes/<subdir>, a model of its own.
func ModelDir(base, home, target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	root := git.Toplevel(abs)
	if root == "" {
		return "", fmt.Errorf("%s is not in a git work tree", target)
	}
	main := root
	if common, err := git.Line(root, "rev-parse", "--path-format=absolute", "--git-common-dir"); err == nil && filepath.Base(common) == ".git" {
		main = filepath.Dir(common)
	}
	if rest, ok := strings.CutPrefix(base, "~/"); ok {
		base = filepath.Join(home, rest)
	}
	// A relative base, from an unknown home or the config, would put the
	// model under whatever the cwd is.
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("architecture_dir %q isn't absolute: give a full path or ~/..., with a home directory set", base)
	}
	dir := filepath.Join(base, filepath.Base(filepath.Dir(main)), filepath.Base(main))
	if phys, err := filepath.EvalSymlinks(abs); err == nil {
		abs = phys
	}
	if rel, err := filepath.Rel(root, abs); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		dir = filepath.Join(dir, "scopes", rel)
	}
	return dir, nil
}

// Fingerprint is what a model was drawn from: every module, sorted, then
// every package externals.tsv marks declared, sorted. A standard-library
// or transitive import changes nothing; a new manifest dependency does.
func Fingerprint(rawDir string) ([]string, error) {
	edges, err := readTSV(filepath.Join(rawDir, "deps.tsv"), 3)
	if err != nil {
		return nil, err
	}
	modules, err := moduleNames(rawDir, edges)
	if err != nil {
		return nil, err
	}
	declared, err := declaredExternals(rawDir)
	if err != nil {
		return nil, err
	}
	return append(modules, declared...), nil
}

// moduleNames is every module modules.txt lists or a deps.tsv edge names,
// sorted. A raw dir from before modules.txt reads as its edges alone.
func moduleNames(rawDir string, edges [][]string) ([]string, error) {
	modules, err := readLines(filepath.Join(rawDir, "modules.txt"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, e := range edges {
		if len(e) == 3 {
			modules = append(modules, e[1], e[2])
		}
	}
	modules = slices.DeleteFunc(modules, func(m string) bool { return m == "" })
	slices.Sort(modules)
	return slices.Compact(modules), nil
}

// declaredExternals is every package externals.tsv marks declared, sorted.
func declaredExternals(rawDir string) ([]string, error) {
	rows, err := readTSV(filepath.Join(rawDir, "externals.tsv"), 3)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, r := range rows {
		if len(r) == 3 && r[2] == "declared" {
			out = append(out, r[1])
		}
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// C4 writes a container view skeleton to w, built from rawDir's deps.tsv,
// modules.txt, and externals.tsv: each module a container inside one
// system boundary, with its %% path:, each declared package an external
// with its %% package:, and an edge per module pair that imports. The classDef
// lines, people, and anything the imports can't show are left to whoever
// finishes it.
func C4(rawDir, system string, w io.Writer) error {
	edges, err := readTSV(filepath.Join(rawDir, "deps.tsv"), 3)
	if err != nil {
		return err
	}
	declared, err := declaredExternals(rawDir)
	if err != nil {
		return err
	}
	modules, err := moduleNames(rawDir, edges)
	if err != nil {
		return err
	}
	nodeID := nodeIDs()
	fmt.Fprintf(w, "flowchart TB\n  subgraph system[%q]\n", system)
	for _, m := range modules {
		fmt.Fprintf(w, "    %s[%q]\n    %%%% path: %s\n", nodeID("m", m), m, m)
	}
	fmt.Fprintln(w, "  end\n  class system boundary")
	for _, p := range declared {
		fmt.Fprintf(w, "  %s[%q]:::ext\n  %%%% package: %s\n", nodeID("ext", p), p, p)
	}
	for _, e := range edges {
		if len(e) == 3 && e[1] != e[2] {
			fmt.Fprintf(w, "  %s --> %s\n", nodeID("m", e[1]), nodeID("m", e[2]))
		}
	}
	return nil
}

var notIDChar = regexp.MustCompile(`[^A-Za-z0-9]+`)

// nodeIDs gives each name a Mermaid node id: prefix, then name's letters
// and digits joined by underscores, so no id is a keyword like end. A name
// whose id another took (foo-bar, foo/bar) gets _2, _3.
func nodeIDs() func(prefix, name string) string {
	byName, taken := map[string]string{}, map[string]bool{}
	return func(prefix, name string) string {
		key := prefix + "\x00" + name
		if id, ok := byName[key]; ok {
			return id
		}
		base := prefix + "_" + strings.Trim(notIDChar.ReplaceAllString(name, "_"), "_")
		id := base
		for n := 2; taken[id]; n++ {
			id = fmt.Sprintf("%s_%d", base, n)
		}
		byName[key], taken[id] = id, true
		return id
	}
}

// node is one node of a model file, with the comments under it.
type node struct {
	id, label, file string
	path, pkg       string
	judgment, isExt bool
}

var (
	nodeLine = regexp.MustCompile(`^\s*([A-Za-z_][\w-]*)\s*[\[({]+"?([^"\])}]*)"?[\])}]+(:::(\w+))?\s*$`)
	// endpoint is one end of an edge: an id, maybe with its shape and
	// label inline (ui["UI"]), maybe with a class (:::ext).
	endpoint  = `([A-Za-z_][\w-]*)(?:\s*[\[({]+"?([^"\])}]*)"?[\])}]+)?(?::::(\w+))?`
	edgeStart = regexp.MustCompile(`^\s*` + endpoint)
	edgeNext  = regexp.MustCompile(`^\s*(?:-->|-\.->|==>|---)(?:\|[^|]*\|)?\s*` + endpoint)
	comment   = regexp.MustCompile(`^\s*%%\s*(path|package|source):\s*(.+?)\s*$`)
)

// readModel is every node and edge in the c4-*.mmd files of modelDir.
func readModel(modelDir string) (nodes map[string]*node, edges [][2]string, files []string, err error) {
	files, err = filepath.Glob(filepath.Join(modelDir, "c4-*.mmd"))
	if err != nil {
		return nil, nil, nil, err
	}
	r := modelReader{nodes: map[string]*node{}}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, nil, nil, err
		}
		r.last = nil
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			r.line(scanner.Text(), file)
		}
		_ = f.Close()
		if err := scanner.Err(); err != nil {
			return nil, nil, nil, err
		}
	}
	return r.nodes, r.edges, files, nil
}

// modelReader collects a model's nodes and edges, one line at a time.
type modelReader struct {
	nodes map[string]*node
	edges [][2]string
	last  *node // the node a %% comment under it describes
}

// mermaidKeywords open a line that holds no node or edge.
var mermaidKeywords = []string{"flowchart", "graph", "subgraph", "end", "class", "classDef", "linkStyle", "style", "direction"}

// line reads one line of file: a %% comment on the last node, a node, or
// a chain of edges.
func (r *modelReader) line(line, file string) {
	trimmed := strings.TrimSpace(line)
	if m := comment.FindStringSubmatch(line); m != nil && r.last != nil {
		switch m[1] {
		case "path":
			r.last.path = m[2]
		case "package":
			r.last.pkg = m[2]
		case "source":
			r.last.judgment = m[2] == "judgment"
		}
		return
	}
	if first, _, _ := strings.Cut(trimmed, " "); strings.HasPrefix(trimmed, "%%") || trimmed == "" || slices.Contains(mermaidKeywords, first) {
		return
	}
	if m := nodeLine.FindStringSubmatch(line); m != nil {
		r.last = addNode(r.nodes, m[1], m[2], m[4], file)
		return
	}
	ends := chain(line)
	for i, end := range ends {
		if len(ends) < 2 {
			break
		}
		addNode(r.nodes, end[1], end[2], end[3], file)
		if i > 0 {
			r.edges = append(r.edges, [2]string{ends[i-1][1], end[1]})
		}
	}
}

// addNode is nodes' node id, added with label from file when it's new;
// a class of ext marks it external.
func addNode(nodes map[string]*node, id, label, class, file string) *node {
	n := nodes[id]
	if n == nil {
		n = &node{id: id, label: label, file: filepath.Base(file)}
		nodes[id] = n
	}
	n.isExt = n.isExt || class == "ext"
	return n
}

// chain is the endpoints of the edges on line, in order, each as its
// match groups (whole, id, label, class): a --> b --> c gives three.
// Fewer than two means line holds no edge. Both patterns start at ^, so
// a match's length is where the rest of the line starts.
func chain(line string) [][]string {
	m := edgeStart.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	ends := [][]string{m}
	for rest := line[len(m[0]):]; ; {
		n := edgeNext.FindStringSubmatch(rest)
		if n == nil {
			return ends
		}
		ends = append(ends, n)
		rest = rest[len(n[0]):]
	}
}

// CheckC4 checks the model in modelDir against rawDir, collected from the
// repo at root, and writes each problem to w: a fingerprint that differs,
// a %% path: that's gone, an edge between two containers with no import
// between their modules, and an external whose %% package: no manifest
// declares, or that names none. It lists the nodes marked
// %% source: judgment, the ones a person reviews. ok is false on any
// problem, or when modelDir has no c4-*.mmd file.
func CheckC4(modelDir, rawDir, root string, w io.Writer) (ok bool, err error) {
	nodes, edges, files, err := readModel(modelDir)
	if err != nil {
		return false, err
	}
	if len(files) == 0 {
		fmt.Fprintf(w, "no model: %s holds no c4-*.mmd file\n", modelDir)
		return false, nil
	}
	now, err := Fingerprint(rawDir)
	if err != nil {
		return false, err
	}
	saved, err := readLines(filepath.Join(modelDir, "fingerprint.txt"))
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	declared, err := declaredExternals(rawDir)
	if err != nil {
		return false, err
	}
	deps, err := readTSV(filepath.Join(rawDir, "deps.tsv"), 3)
	if err != nil {
		return false, err
	}
	ids := slices.Sorted(maps.Keys(nodes))
	problems := diffFingerprint(now, saved)
	problems = append(problems, checkNodes(nodes, ids, declared, root)...)
	problems = append(problems, checkEdges(nodes, edges, deps)...)
	for _, p := range problems {
		fmt.Fprintln(w, p)
	}
	for _, id := range ids {
		if n := nodes[id]; n.judgment {
			fmt.Fprintf(w, "judgment: %s %q (%s)\n", n.id, cmp.Or(n.label, n.id), n.file)
		}
	}
	return len(problems) == 0, nil
}

// diffFingerprint is a stale line for each module now has that saved
// lacks, and each saved has that now lacks.
func diffFingerprint(now, saved []string) []string {
	var problems []string
	for _, s := range now {
		if !slices.Contains(saved, s) {
			problems = append(problems, fmt.Sprintf("stale: %s is new since the model was drawn", s))
		}
	}
	for _, s := range saved {
		if s != "" && !slices.Contains(now, s) {
			problems = append(problems, fmt.Sprintf("stale: %s is gone since the model was drawn", s))
		}
	}
	return problems
}

// checkNodes is a line for each node, in ids order, whose %% path: is
// outside root or gone, or whose external package no manifest declares,
// or that names none.
func checkNodes(nodes map[string]*node, ids, declared []string, root string) []string {
	var problems []string
	for _, id := range ids {
		n := nodes[id]
		switch {
		case n.path != "" && !filepath.IsLocal(n.path):
			problems = append(problems, fmt.Sprintf("gone: %s (%s) cites %s, which is outside the repo", n.id, n.file, n.path))
		case n.path != "":
			if _, err := os.Stat(filepath.Join(root, n.path)); err != nil {
				problems = append(problems, fmt.Sprintf("gone: %s (%s) cites %s, which no longer exists", n.id, n.file, n.path))
			}
		case n.isExt && n.pkg != "" && !slices.Contains(declared, n.pkg):
			problems = append(problems, fmt.Sprintf("undeclared: %s (%s) names package %s, which no manifest declares", n.id, n.file, n.pkg))
		case n.isExt && n.pkg == "" && !n.judgment:
			problems = append(problems, fmt.Sprintf("unbacked: %s (%s) is an external with no %%%% package: and no %%%% source: judgment", n.id, n.file))
		}
	}
	return problems
}

// checkEdges is a line for each edge between two containers with paths
// that no import in deps backs.
func checkEdges(nodes map[string]*node, edges [][2]string, deps [][]string) []string {
	var problems []string
	for _, e := range edges {
		from, to := nodes[e[0]], nodes[e[1]]
		if from == nil || to == nil || from.path == "" || to.path == "" || from.path == to.path {
			continue
		}
		if !slices.ContainsFunc(deps, func(d []string) bool {
			return len(d) == 3 && overlaps(d[1], from.path) && overlaps(d[2], to.path)
		}) {
			problems = append(problems, fmt.Sprintf("unbacked: %s --> %s has no import from %s into %s", e[0], e[1], from.path, to.path))
		}
	}
	return problems
}

// overlaps is true when module and a container's path name the same
// directory, or one holds the other: a container can be coarser or finer
// than a module.
func overlaps(module, p string) bool {
	module, p = strings.Trim(module, "/"), strings.Trim(path.Clean(p), "/")
	return module == p || strings.HasPrefix(module, p+"/") || strings.HasPrefix(p, module+"/") || p == "."
}
