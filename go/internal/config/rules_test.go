package config

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

// sourceRules is every slug the kit's code passes as a rule: the last
// argument of a Block, Deny, block, or RuleDisabled call when it's a string
// literal, the same of an Ask or ask call, and each string literal in the
// code, for the reverse check.
func sourceRules(t *testing.T) (passed, asked []string, literals map[string]bool) {
	t.Helper()
	literals = map[string]bool{}
	fset := token.NewFileSet()
	for _, dir := range []string{"..", "../../cmd"} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.BasicLit:
					if s, err := strconv.Unquote(n.Value); err == nil && n.Kind == token.STRING {
						literals[s] = true
					}
				case *ast.CallExpr:
					sel, ok := n.Fun.(*ast.SelectorExpr)
					if !ok || len(n.Args) == 0 {
						return true
					}
					lit, ok := n.Args[len(n.Args)-1].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return true
					}
					s, _ := strconv.Unquote(lit.Value)
					switch sel.Sel.Name {
					case "Block", "Deny", "block", "RuleDisabled":
						passed = append(passed, s)
					case "Ask", "ask":
						asked = append(asked, s)
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return passed, asked, literals
}

// Rules lists every slug the code blocks with, and nothing the code lost.
func TestRulesMatchTheCode(t *testing.T) {
	t.Parallel()
	passed, asked, literals := sourceRules(t)
	for _, slug := range passed {
		if !slices.Contains(Rules, slug) {
			t.Errorf("the code blocks with %q, which Rules lacks", slug)
		}
	}
	for _, slug := range asked {
		if !slices.Contains(Rules, slug) && !slices.Contains(AskRules, slug) {
			t.Errorf("the code asks with %q, which AskRules lacks", slug)
		}
	}
	for _, slug := range Rules {
		if !literals[slug] {
			t.Errorf("Rules has %q, which no code uses", slug)
		}
	}
	for _, slug := range AskRules {
		if !slices.Contains(asked, slug) || slices.Contains(Rules, slug) {
			t.Errorf("AskRules has %q, which no code asks with or Rules already has", slug)
		}
	}
	if !slices.IsSorted(Rules) || !slices.IsSorted(AskRules) {
		t.Error("Rules or AskRules isn't sorted")
	}
}

func TestAnUnknownRuleNameWarns(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		cfg  Config
		want []string // each warning's prefix, in order
	}{
		{"an unknown disabled rule",
			Config{DisabledRules: []string{"plan-critic", "slop-copula", "slop-vocabulary", "plan-critc", "slop-nope"}, Slop: Slop{Rules: []SlopRule{{ID: "copula"}}}},
			[]string{`disabled_rules: "plan-critc"`, `disabled_rules: "slop-nope"`}},
		{"an unknown shadow rule",
			Config{ShadowRules: []string{"weaken", "weakn"}},
			[]string{`shadow_rules: "weakn"`}},
		// The slop guards deny under one rule, slop, so a shadowed slop-<id>
		// would shadow nothing.
		{"a shadowed slop rule",
			Config{ShadowRules: []string{"slop", "slop-vocabulary"}, Slop: Slop{Rules: []SlopRule{{ID: "vocabulary"}}}},
			[]string{`shadow_rules: "slop-vocabulary"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, err := range tc.cfg.unknownDisables() {
				got = append(got, err.Error())
			}
			if len(got) != len(tc.want) {
				t.Fatalf("warnings: %q, want %d", got, len(tc.want))
			}
			for i, prefix := range tc.want {
				if !strings.HasPrefix(got[i], prefix) {
					t.Errorf("warning %d = %q, want prefix %q", i, got[i], prefix)
				}
			}
		})
	}
}
