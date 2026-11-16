package resolve

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// byName are the binaries code outside resolve may run by name, per ADR 0004.
var byName = []string{"git", "bash"}

// lookPathAllowed are the files outside resolve that may call
// exec.LookPath: run.go checks whether PATH already finds a resolved copy,
// and runs nothing it finds.
var lookPathAllowed = []string{"../run/run.go"}

// TestBinariesRunByNameOnlyWhereADR0004Says fails on exec.LookPath, or a
// literal binary name passed to proc.Command or exec.Command, outside
// resolve and its kittool, other than the byName ones. A new tool the kit
// runs goes through kittool.Path, a project's command through Bin.
func TestBinariesRunByNameOnlyWhereADR0004Says(t *testing.T) {
	// The name's position among each runner's arguments.
	runners := map[string]int{"proc.Command": 1, "exec.Command": 0, "exec.CommandContext": 1}
	for _, dir := range []string{"..", "../../cmd"} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && strings.HasPrefix(path+"/", "../resolve/") {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				fn := pkg.Name + "." + sel.Sel.Name
				if fn == "exec.LookPath" && !slices.Contains(lookPathAllowed, path) {
					t.Errorf("%s: exec.LookPath: use kittool.Path or Resolver.Bin", fset.Position(call.Pos()))
				}
				i, isRunner := runners[fn]
				if !isRunner || len(call.Args) <= i {
					return true
				}
				lit, ok := call.Args[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				if name, _ := strconv.Unquote(lit.Value); !slices.Contains(byName, name) {
					t.Errorf("%s: %s(%s): use kittool.Path or Resolver.Bin", fset.Position(lit.Pos()), fn, lit.Value)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
