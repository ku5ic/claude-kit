// Package blast is blast-radius: the files that import a given file
// (rules/change.md section 3).
//
// JS/TS: import, export-from, require() and import() specifiers. Relative
// specifiers resolve against the importing file; "@/" and "~/" aliases match
// on the path suffix (alias-match); a workspace package's name, or
// name/subpath, counts for every file in that package (workspace-match).
// Python: "import m" and "from m import x", absolute or relative; the module
// path follows the __init__.py chain, else it is root-relative minus src/.
// With a symbol, only consumers using it as a whole word are kept. Static
// only: a non-literal import() or require() prints "unresolvable imports
// present".
package blast

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/extract"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

var jsFiles = []string{"*.js", "*.jsx", "*.ts", "*.tsx", "*.mjs", "*.cjs", "*.mts", "*.cts", "*.vue", "*.svelte", "*.astro"}

// The matchers are the bash original's EREs, compiled POSIX so alternation
// stays leftmost-longest as grep's.
var (
	jsCandidate = regexp.MustCompilePOSIX(`(from|import|require)[[:space:]]*\(?[[:space:]]*['"]`)
	jsSpec      = regexp.MustCompilePOSIX(`(from|import|require[[:space:]]*\(|import[[:space:]]*\()[[:space:]]*['"]([^'"]+)['"]`)
	jsDynamic   = regexp.MustCompilePOSIX(`(^|[^A-Za-z0-9_$.])(import|require)[[:space:]]*\([[:space:]]*[^"'[:space:])]`)
	pyCandidate = regexp.MustCompilePOSIX(`^[[:space:]]*(from[[:space:]]+[.A-Za-z0-9_]+[[:space:]]+import[[:space:]]|import[[:space:]]+[A-Za-z_])`)
	pyFrom      = regexp.MustCompilePOSIX(`^[[:space:]]*from[[:space:]]+([.A-Za-z0-9_]+)[[:space:]]+import[[:space:]]+(.*)$`)
	pyImport    = regexp.MustCompilePOSIX(`^[[:space:]]*import[[:space:]]+(.*)$`)
)

type hit struct {
	file    string
	line    int
	content string
}

type scan struct {
	root, rel, symbol string
	testGlobs         []string
	seen              map[string]bool
	results           []string
	tests             int
}

// Run is blast-radius.sh <file> [symbol].
func Run(cfg *config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(stderr, "usage: blast-radius <file> [symbol]")
		return 2
	}
	target, symbol := args[0], ""
	if len(args) == 2 {
		symbol = args[1]
	}
	if info, err := os.Stat(target); err != nil || !info.Mode().IsRegular() {
		fmt.Fprintf(stderr, "blast-radius: no such file: %s\n", target)
		return 2
	}
	// Absolute before physical: a relative dir stays relative to cwd, not
	// the repo root the rel path below is cut from.
	dir, _ := filepath.Abs(filepath.Dir(target))
	dir, _ = filepath.EvalSymlinks(dir)
	abs := filepath.Join(dir, filepath.Base(target))
	root := project.Toplevel(dir)
	if root == "" {
		fmt.Fprintln(stderr, "blast-radius: not inside a git repository")
		return 2
	}
	s := &scan{root: root, rel: strings.TrimPrefix(abs, root+"/"), symbol: symbol, seen: map[string]bool{}}
	for _, rule := range cfg.SkillFileMap {
		if slices.Contains(rule.Skills, "test-patterns") {
			s.testGlobs = append(s.testGlobs, rule.Globs...)
		}
	}

	dynamic := false
	switch ext := path.Ext(s.rel); ext {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
		dynamic = s.javascript(cfg)
	case ".py":
		s.python()
	default:
		fmt.Fprintf(stderr, "blast-radius: no import scanner for %s\n", filepath.Base(s.rel))
		return 2
	}

	header := "blast-radius: " + s.rel
	if symbol != "" {
		header += " (symbol " + symbol + ")"
	}
	fmt.Fprintln(stdout, header)
	fmt.Fprintf(stdout, "consumers: %d (source %d, test %d)\n", len(s.results), len(s.results)-s.tests, s.tests)
	slices.Sort(s.results)
	for _, r := range s.results[:min(len(s.results), 50)] {
		fmt.Fprintln(stdout, r)
	}
	if len(s.results) > 50 {
		fmt.Fprintf(stdout, "... %d more\n", len(s.results)-50)
	}
	if dynamic {
		fmt.Fprintln(stdout, "unresolvable imports present")
	}
	return 0
}

// grepFiles is every line matching re in the repo's tracked and
// untracked-but-not-ignored files matching the pathspecs.
func (s *scan) grepFiles(re *regexp.Regexp, pathspecs ...string) []hit {
	out, _ := exec.Command("git", append([]string{"-C", s.root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"}, pathspecs...)...).Output()
	var hits []hit
	for _, file := range strings.Split(string(out), "\x00") {
		if file == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, file))
		if err != nil || bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
			continue
		}
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
		for n := 1; sc.Scan(); n++ {
			if re.MatchString(sc.Text()) {
				hits = append(hits, hit{file, n, sc.Text()})
			}
		}
	}
	return hits
}

// candidates narrows grepFiles to lines holding one of the needles (as
// grep -F over "path:line:content"); no needles means no narrowing.
func (s *scan) candidates(re *regexp.Regexp, needles []string, pathspecs ...string) []hit {
	var keep []string
	for _, n := range needles {
		if n != "" {
			keep = append(keep, n)
		}
	}
	hits := s.grepFiles(re, pathspecs...)
	if len(keep) == 0 {
		return hits
	}
	var out []hit
	for _, h := range hits {
		line := fmt.Sprintf("%s:%d:%s", h.file, h.line, h.content)
		if slices.ContainsFunc(keep, func(n string) bool { return strings.Contains(line, n) }) {
			out = append(out, h)
		}
	}
	return out
}

var wordChars = regexp.MustCompile(`[A-Za-z0-9_]`)

func (s *scan) add(file string, line int, label string) {
	if file == s.rel || s.seen[file] {
		return
	}
	s.seen[file] = true
	if s.symbol != "" && !containsWord(filepath.Join(s.root, file), s.symbol) {
		return
	}
	kind := "source"
	base := filepath.Base(file)
	if slices.ContainsFunc(s.testGlobs, func(g string) bool { return guard.Glob(g, base) }) {
		kind = "test"
		s.tests++
	}
	result := fmt.Sprintf("%s:%d %s", file, line, kind)
	if label != "" {
		result += " " + label
	}
	s.results = append(s.results, result)
}

// containsWord is grep -qw: word as a whole word, word characters being
// letters, digits, and underscore.
func containsWord(file, word string) bool {
	data, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	text := string(data)
	for i := 0; ; {
		j := strings.Index(text[i:], word)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(word)
		before := start == 0 || !wordChars.MatchString(text[start-1:start])
		after := end == len(text) || !wordChars.MatchString(text[end:end+1])
		if before && after {
			return true
		}
		i = start + 1
	}
}

func stripJSExt(p string) string {
	switch path.Ext(p) {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
		return strings.TrimSuffix(p, path.Ext(p))
	}
	return p
}

// parent is the path's directory, "" for a top-level name.
func parent(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

// normPath resolves . and .. segments; "" for the root.
func normPath(p string) string {
	var out []string
	for _, part := range strings.Split(p, "/") {
		switch part {
		case "", ".":
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		default:
			out = append(out, part)
		}
	}
	return strings.Join(out, "/")
}

func (s *scan) javascript(cfg *config.Config) (dynamic bool) {
	stem := stripJSExt(s.rel)
	isIndex := path.Base(stem) == "index"
	indexDir := ""
	if isIndex {
		indexDir = parent(stem)
	}
	var wsNames []string
	for _, dir := range project.Subprojects(cfg, s.root) {
		if dir != "." && strings.HasPrefix(s.rel, dir+"/") {
			if name := extract.JSONValue(filepath.Join(s.root, dir, "package.json"), ".name"); name != "" {
				wsNames = append(wsNames, name)
			}
		}
	}
	needles := append([]string{path.Base(stem), path.Base(indexDir)}, wsNames...)
	if indexDir == "" {
		needles[1] = ""
	}
	if isIndex {
		// An index is also reached by a bare '.', '..', or './'.
		needles = append(needles, ".'", `."`, "/'", `/"`)
	}
	for _, h := range s.candidates(jsCandidate, needles, jsFiles...) {
		content := h.content
		for {
			m := jsSpec.FindStringSubmatchIndex(content)
			if m == nil {
				break
			}
			spec := content[m[4]:m[5]]
			content = content[m[1]:]
			switch {
			case strings.HasPrefix(spec, "."):
				resolved := stripJSExt(normPath(parent(h.file) + "/" + spec))
				if resolved == stem || (isIndex && resolved == indexDir) {
					s.add(h.file, h.line, "")
				}
			case strings.HasPrefix(spec, "@/") || strings.HasPrefix(spec, "~/"):
				suffix := stripJSExt(spec[2:])
				if stem == suffix || strings.HasSuffix(stem, "/"+suffix) ||
					(isIndex && (indexDir == suffix || strings.HasSuffix(indexDir, "/"+suffix))) {
					s.add(h.file, h.line, "alias-match")
				}
			default:
				for _, name := range wsNames {
					if spec == name || strings.HasPrefix(spec, name+"/") {
						s.add(h.file, h.line, "workspace-match")
					}
				}
			}
		}
	}
	return len(s.grepFiles(jsDynamic, jsFiles...)) > 0
}

// pyModule is the dotted module of a root-relative .py path: the
// __init__.py chain decides the package root, else src/ is dropped.
func (s *scan) pyModule(p string) string {
	dir := parent(p)
	mod := strings.TrimSuffix(strings.TrimSuffix(p, ".py"), "/__init__")
	if dir != "" && isFile(filepath.Join(s.root, dir, "__init__.py")) {
		pkgRoot := dir
		for pkgRoot != "" && isFile(filepath.Join(s.root, pkgRoot, "__init__.py")) {
			pkgRoot = parent(pkgRoot)
		}
		if pkgRoot != "" {
			mod = strings.TrimPrefix(mod, pkgRoot+"/")
		}
	} else {
		mod = strings.TrimPrefix(mod, "src/")
	}
	return strings.ReplaceAll(mod, "/", ".")
}

func dottedParent(m string) string {
	if i := strings.LastIndexByte(m, '.'); i >= 0 {
		return m[:i]
	}
	return ""
}

// pyResolve is the absolute module a `from <spec> import` in file names.
func (s *scan) pyResolve(file, spec string) string {
	if !strings.HasPrefix(spec, ".") {
		return spec
	}
	rest := strings.TrimLeft(spec, ".")
	dots := len(spec) - len(rest)
	pkg := s.pyModule(file)
	if path.Base(file) != "__init__.py" {
		pkg = dottedParent(pkg)
	}
	for i := 1; i < dots; i++ {
		pkg = dottedParent(pkg)
	}
	if pkg != "" && rest != "" {
		return pkg + "." + rest
	}
	return pkg + rest
}

func (s *scan) python() {
	mod := s.pyModule(s.rel)
	modParent := dottedParent(mod)
	leaf := mod[strings.LastIndexByte(mod, '.')+1:]
	for _, h := range s.candidates(pyCandidate, []string{leaf}, "*.py") {
		content, _, _ := strings.Cut(h.content, "#")
		if m := pyFrom.FindStringSubmatch(content); m != nil {
			names := " " + strings.NewReplacer("(", " ", ")", " ", ",", " ").Replace(m[2]) + " "
			from := s.pyResolve(h.file, m[1])
			if from != "" && (from == mod || (from == modParent && containsSpaced(names, leaf))) {
				s.add(h.file, h.line, "")
			}
		} else if m := pyImport.FindStringSubmatch(content); m != nil {
			for _, item := range strings.Split(m[1], ",") {
				fields := strings.Fields(item)
				if len(fields) > 0 && (fields[0] == mod || strings.HasPrefix(fields[0], mod+".")) {
					s.add(h.file, h.line, "")
				}
			}
		}
	}
}

// containsSpaced is bash's [[ $names == *[[:space:]]$leaf[[:space:]]* ]].
func containsSpaced(names, leaf string) bool {
	for _, sep := range []string{" ", "\t"} {
		for _, sep2 := range []string{" ", "\t"} {
			if strings.Contains(names, sep+leaf+sep2) {
				return true
			}
		}
	}
	return false
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}
