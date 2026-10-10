package classify

import (
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// builtins are the shell's own commands, which run no binary.
var builtins = []string{"[", "[[", "test", "echo", "printf", "exit", "return", "cd", "export", "set", "unset", "true", "false", ":",
	"read", "shift", "eval", "source", ".", "local", "declare", "trap", "wait", "pushd", "popd", "command", "type", "hash", "umask", "alias"}

// Statements is body's top-level commands in order, each printed back as
// shell: statements a newline or && separates come apart, anything else (a
// pipe, ||, a subshell) stays whole. ok is false when body doesn't parse.
func Statements(body string) (stmts []string, ok bool) {
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(body), "")
	if err != nil {
		return nil, false
	}
	printer := syntax.NewPrinter()
	var add func(s *syntax.Stmt)
	add = func(s *syntax.Stmt) {
		if and, isAnd := s.Cmd.(*syntax.BinaryCmd); isAnd && and.Op == syntax.AndStmt && !s.Negated && !s.Background && len(s.Redirs) == 0 {
			add(and.X)
			add(and.Y)
			return
		}
		var b strings.Builder
		_ = printer.Print(&b, s)
		stmts = append(stmts, strings.TrimSpace(b.String()))
	}
	for _, s := range file.Stmts {
		add(s)
	}
	return stmts, true
}

// Calls is every simple command body runs, as its literal words, in order:
// inside pipes, lists, subshells, and command substitutions too, with
// wrappers and the assignments after them dropped, and a find -exec or
// xargs target listed after its command. A command whose first word isn't
// literal is ["$"]; shell builtins aren't listed. ok is false when body
// doesn't parse.
func Calls(body string) (calls [][]string, ok bool) {
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(body), "")
	if err != nil {
		return nil, false
	}
	syntax.Walk(file, func(node syntax.Node) bool {
		call, isCall := node.(*syntax.CallExpr)
		if !isCall || len(call.Args) == 0 {
			return true
		}
		var words []string
		for _, w := range call.Args {
			v, literal := literal(w)
			if !literal {
				break
			}
			words = append(words, v)
		}
		if prefix, wrapped := matchPrefix(words, Grammar.Wrappers); wrapped {
			words = words[prefix:]
			for len(words) > 0 && strings.Contains(words[0], "=") && !strings.HasPrefix(words[0], "-") {
				words = words[1:]
			}
		}
		switch {
		case len(words) == 0:
			calls = append(calls, []string{"$"})
		case !slices.Contains(builtins, words[0]):
			calls = append(calls, words)
			if _, inner := execTarget(words); len(inner) > 0 {
				calls = append(calls, inner)
			}
		}
		return true
	})
	return calls, true
}
