package hook

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// undocumented are the keys no example in testdata/payloads.json holds,
// each with why.
var undocumented = map[string]string{
	KeyInputEdits:        "MultiEdit has no tool_input table in the docs",
	KeyInputNotebookPath: "NotebookEdit has no tool_input table in the docs",
	KeyInputSkill:        "Skill has no tool_input table in the docs",
	KeyInputMessage:      "SendMessage has no tool_input table in the docs",
	KeyInputBefore:       "the kit's own, in bash-edits' synthetic payload",
}

// keys is every Key constant in keys.go, by value.
func keys(t *testing.T) map[string]string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "keys.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		if spec, ok := n.(*ast.ValueSpec); ok {
			for i, name := range spec.Names {
				if lit, ok := spec.Values[i].(*ast.BasicLit); ok {
					value, _ := strconv.Unquote(lit.Value)
					out[value] = name.Name
				}
			}
		}
		return true
	})
	if len(out) == 0 {
		t.Fatal("no Key constants in keys.go")
	}
	return out
}

// TestPayloadKeys checks each key against the docs' example payloads. One
// an example holds is documented, and only one none holds is undocumented.
func TestPayloadKeys(t *testing.T) {
	raw, err := os.ReadFile("testdata/payloads.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Examples []struct {
			Payload map[string]any `json:"payload"`
		} `json:"examples"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for value, name := range keys(t) {
		documented := false
		for _, e := range fixture.Examples {
			documented = documented || sources.GetPath(e.Payload, value) != nil
		}
		_, listed := undocumented[value]
		switch {
		case !documented && !listed:
			t.Errorf("%s (%q) is in no example payload: add the docs' example, or list it in undocumented with why", name, value)
		case documented && listed:
			t.Errorf("%s (%q) is in an example payload now: drop it from undocumented", name, value)
		}
	}
}

// TestChecksReadPayloadKeysByName fails on an accessor call that spells a
// key as a literal. A mistyped literal reads "", so its check fails open.
func TestChecksReadPayloadKeysByName(t *testing.T) {
	byValue := keys(t)
	accessors := []string{"String", "Bool", "Strings", "Object"}
	for _, dir := range []string{"..", "../../cmd"} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				sel, isSel := call.Fun.(*ast.SelectorExpr)
				lit, isLit := call.Args[0].(*ast.BasicLit)
				if !isSel || !isLit || lit.Kind != token.STRING || !slices.Contains(accessors, sel.Sel.Name) {
					return true
				}
				// A mistyped key matches no constant, so any literal fails.
				value, _ := strconv.Unquote(lit.Value)
				name, ok := byValue[value]
				if !ok {
					name = "Key... (none holds " + lit.Value + ")"
				}
				t.Errorf("%s: %s(%s): use hook.%s", fset.Position(lit.Pos()), sel.Sel.Name, lit.Value, name)
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
