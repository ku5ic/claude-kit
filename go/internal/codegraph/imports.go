package codegraph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/proc"
)

// importRules are ast-grep rules, one per language, matching each import's
// source text.
var importRules = func() string {
	// Static import, export-from, require() and dynamic import() sources.
	js := `{kind: string, any: [{inside: {kind: import_statement, field: source}}, {inside: {kind: export_statement, field: source}}, {inside: {kind: arguments, inside: {kind: call_expression, has: {field: function, regex: "^(require|import)$"}}}}]}`
	rules := []struct{ languages, matcher string }{
		{"tsx typescript javascript", js},
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

// astGrep is the ast-grep binary: its own name, else sg, the name older
// installs and Homebrew's formula use.
func astGrep() (string, error) {
	for _, name := range []string{"ast-grep", "sg"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("ast-grep (or sg) isn't installed")
}

// Imports writes every import under root as path<TAB>import text, the
// path relative to root and the text on one line.
func Imports(root string, out io.Writer) error {
	bin, err := astGrep()
	if err != nil {
		return err
	}
	root = strings.TrimRight(root, "/") // repo/ would make the prefix repo// and match nothing
	// The = form: a value starting with --- would parse as a flag.
	stream, err := proc.Command(10*time.Minute, bin, "scan", "--json=stream", "--inline-rules="+importRules, root).Output()
	if err != nil {
		return fmt.Errorf("%s scan: %w", bin, err)
	}
	// ast-grep scans in parallel: sorted, two runs print the same.
	var rows []string
	flatten := strings.NewReplacer("\n", " ", "\t", " ")
	for line := range strings.SplitSeq(string(stream), "\n") {
		var match struct{ File, Text string }
		if json.Unmarshal([]byte(line), &match) != nil {
			continue
		}
		file := strings.TrimPrefix(match.File, root+"/")
		// testdata holds fixtures, not the repo's code: the go tool skips it too.
		if !slices.Contains(strings.Split(path.Dir(file), "/"), "testdata") {
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
