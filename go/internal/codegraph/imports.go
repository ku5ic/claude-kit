package codegraph

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

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
	bin, err := kittool.Path(astGrep...)
	if err != nil {
		return err
	}
	// ast-grep prints ./src/a.py as src/a.py, and repo/ would make the
	// prefix repo//: either way the prefix would match nothing.
	root = filepath.Clean(root)
	if _, err := os.Stat(root); err != nil {
		return err
	}
	// The = form: a value starting with --- would parse as a flag.
	cmd := proc.Command(scanTimeout, bin, "scan", "--json=stream", "--inline-rules="+importRules, root)
	stream, err := cmd.Output()
	if err != nil {
		return scanError(bin, cmd, err)
	}
	// ast-grep scans in parallel: sorted, two runs print the same.
	var rows, odd []string
	flatten := strings.NewReplacer("\n", " ", "\t", " ")
	for line := range strings.SplitSeq(string(stream), "\n") {
		var match struct{ File, Text string }
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
		if !skipped(file, skipDirs) {
			rows = append(rows, file+"\t"+flatten.Replace(match.Text))
		}
	}
	slices.Sort(rows)
	w := bufio.NewWriter(out)
	for _, row := range rows {
		fmt.Fprintln(w, row)
	}
	return w.Flush()
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
