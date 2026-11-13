package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitcmd"
)

// Every command kitcmd lists must reach a handler: a case in run's
// switches, or a configCommands key. Read from the source, since running
// each one would run it (scratch-rotate prunes).
func TestEveryListedCommandIsDispatched(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	handled := map[string]bool{}
	for name := range configCommands {
		handled[name] = true
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "run" {
			ast.Inspect(fn, func(n ast.Node) bool {
				if cc, ok := n.(*ast.CaseClause); ok {
					for _, e := range cc.List {
						if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							if s, err := strconv.Unquote(lit.Value); err == nil {
								handled[s] = true
							}
						}
					}
				}
				return true
			})
		}
	}
	for _, c := range kitcmd.Commands {
		if !handled[c.Name] {
			t.Errorf("kit %s is listed in kitcmd.Commands, but run has no case for it", c.Name)
		}
	}
}
