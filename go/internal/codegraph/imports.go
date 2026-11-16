package codegraph

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/resolve/kittool"
)

// importRules are ast-grep rules, one per language, matching each import's
// source text.
var importRules = func() string {
	// Static import, export-from, require() and dynamic import() sources;
	// TypeScript adds import x = require(), a kind JavaScript lacks.
	jsSources := `{inside: {kind: import_statement, field: source}}, {inside: {kind: export_statement, field: source}}, {inside: {kind: arguments, inside: {kind: call_expression, has: {field: function, regex: "^(require|import)$"}}}}`
	rules := []struct{ languages, matcher string }{
		{"javascript", `{kind: string, any: [` + jsSources + `]}`},
		{"tsx typescript", `{kind: string, any: [` + jsSources + `, {inside: {kind: import_require_clause}}]}`},
		// Whole from-statements: deps joins the module with each imported name.
		{"python", `{any: [{kind: dotted_name, inside: {any: [{kind: import_statement}, {kind: aliased_import, inside: {kind: import_statement}}]}}, {kind: import_from_statement}]}`},
		{"go", `{kind: interpreted_string_literal, inside: {kind: import_spec}}`},
		// mod x; links a file to its child module file; a mod with a body is inline.
		{"rust", `{any: [{kind: use_declaration}, {kind: extern_crate_declaration}, {kind: mod_item, not: {has: {kind: declaration_list}}}]}`},
		{"java swift scala", `{kind: import_declaration}`},
		{"kotlin", `{kind: import_header}`},
		{"csharp", `{kind: using_directive}`},
		{"php", `{kind: namespace_use_declaration}`},
		{"c cpp", `{kind: preproc_include}`},
		{"ruby", `{kind: call, regex: "^require(_relative)?[ (]"}`},
		{"lua", `{kind: function_call, regex: "^require[ (]"}`},
		{"dart", `{kind: library_import}`},
		{"elixir", `{kind: call, regex: "^(alias|import|require|use) [A-Z]"}`},
	}
	var b strings.Builder
	for _, r := range rules {
		for language := range strings.FieldsSeq(r.languages) {
			fmt.Fprintf(&b, "---\n{id: %s, language: %s, rule: %s}\n", language, language, r.matcher)
		}
	}
	return b.String()
}()

// astGrep is ast-grep's names: its own, else sg, the name older installs
// and Homebrew's formula use.
var astGrep = []string{"ast-grep", "sg"}

// scanTimeout bounds one ast-grep scan.
var scanTimeout = 10 * time.Minute

// scanError is why a scan failed: the deadline, or the first lines of
// ast-grep's stderr, where a rule the installed version rejects is named.
func scanError(bin string, cmd *proc.Cmd, err error) error {
	if cmd.TimedOut() {
		return fmt.Errorf("%s scan timed out after %s", bin, scanTimeout)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		var lines []string
		for line := range strings.Lines(string(exit.Stderr)) {
			if line = strings.TrimSpace(line); line != "" && len(lines) < 3 {
				lines = append(lines, line)
			}
		}
		if len(lines) > 0 {
			return fmt.Errorf("%s scan: %w: %s", bin, err, strings.Join(lines, "; "))
		}
	}
	return fmt.Errorf("%s scan: %w", bin, err)
}

// Imports writes every import under root as path<TAB>import text, the
// path relative to root and the text on one line. A path under testdata
// or any of skipDirs is left out.
func Imports(root string, skipDirs []string, out, stderr io.Writer) error {
	found, err := scanImports(root, skipDirs, stderr)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(out)
	for _, imp := range found {
		fmt.Fprintln(w, imp.file+"\t"+imp.text)
	}
	return w.Flush()
}

// ErrNoASTGrep is the error Imports and Importers wrap when ast-grep isn't
// installed.
var ErrNoASTGrep = errors.New("ast-grep isn't installed")

// componentFiles are single-file component formats ast-grep can't parse:
// their imports sit in a <script> block, so a text match finds them.
var componentFiles = set(".vue", ".svelte", ".astro")

// jsImport is the literal specifier of an import, export-from, require(),
// or import().
var jsImport = regexp.MustCompile(`(?:\bfrom\s*|\bimport\s*\(?\s*|\brequire\s*\(\s*)['"]([^'"\n]+)['"]`)

// Ecosystem is the package ecosystem file's extension belongs to (npm,
// python, go, rust, ruby), "" for none.
func Ecosystem(file string) string { return ecosystems[path.Ext(file)] }

// JSSpecs is every literal module specifier src imports, in order.
func JSSpecs(src string) []string {
	var out []string
	for _, m := range jsImport.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}

// componentImports is each import in root's component files, its text
// quoted as ast-grep prints a JS string. Outside a git repo there are none.
func componentImports(root string, skipDirs []string) []found {
	files, _ := git.Files(root)
	var rows []found
	for _, file := range files {
		if !componentFiles[path.Ext(file)] || skipped(file, skipDirs) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			continue
		}
		for n, line := range strings.Split(string(data), "\n") {
			for _, spec := range JSSpecs(line) {
				rows = append(rows, found{file, `"` + spec + `"`, n + 1})
			}
		}
	}
	return rows
}

// found is one import: its file, relative to the scanned root, its text
// on one line, and its 1-based line.
type found struct {
	file, text string
	line       int
}

// scanImports is every import under root, sorted by file and text. A path
// under testdata or any of skipDirs is left out.
func scanImports(root string, skipDirs []string, stderr io.Writer) ([]found, error) {
	bin, err := kittool.Path(astGrep...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoASTGrep, err)
	}
	// ast-grep prints ./src/a.py as src/a.py, and repo/ would make the
	// prefix repo//: either way the prefix would match nothing.
	root = filepath.Clean(root)
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	// The = form: a value starting with --- would parse as a flag. Hidden
	// directories (.storybook, .github) hold real imports; .git is dropped below.
	cmd := proc.Command(scanTimeout, bin, "scan", "--json=stream", "--no-ignore=hidden", "--inline-rules="+importRules, root)
	stream, err := cmd.Output()
	if err != nil {
		return nil, scanError(bin, cmd, err)
	}
	// ast-grep scans in parallel: sorted, two runs print the same.
	var rows []found
	var odd []string
	flatten := strings.NewReplacer("\n", " ", "\t", " ")
	for line := range strings.SplitSeq(string(stream), "\n") {
		var match struct {
			File, Text string
			Range      struct{ Start struct{ Line int } }
		}
		if json.Unmarshal([]byte(line), &match) != nil {
			continue
		}
		file := strings.TrimPrefix(match.File, root+"/")
		if strings.ContainsAny(file, "\t\n") {
			if !slices.Contains(odd, file) {
				odd = append(odd, file)
				fmt.Fprintf(stderr, "skipped: %q: a tab or newline in the path would split its row\n", file)
			}
			continue
		}
		if !skipped(file, skipDirs) && !slices.Contains(strings.Split(file, "/"), ".git") {
			rows = append(rows, found{file, flatten.Replace(match.Text), match.Range.Start.Line + 1})
		}
	}
	rows = append(rows, componentImports(root, skipDirs)...)
	slices.SortFunc(rows, func(a, b found) int {
		return cmp.Or(strings.Compare(a.file, b.file), strings.Compare(a.text, b.text), cmp.Compare(a.line, b.line))
	})
	return rows, nil
}

// skipped reports whether file sits under testdata or a skipDirs entry.
// testdata holds fixtures, not the repo's code, and the go tool skips it
// too.
func skipped(file string, skipDirs []string) bool {
	for _, dir := range strings.Split(path.Dir(file), "/") {
		if dir == "testdata" || slices.Contains(skipDirs, dir) {
			return true
		}
	}
	return false
}
