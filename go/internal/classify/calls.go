package classify

import (
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// builtins are the shell's own commands, which run no binary.
var builtins = []string{"[", "[[", "test", "echo", "printf", "exit", "return", "cd", "export", "set", "unset", "true", "false", ":",
	"read", "shift", "eval", "source", ".", "local", "declare", "trap", "wait", "pushd", "popd", "command", "type", "hash", "umask", "alias"}

// codeRunners are the builtins that run a command or a string of code.
var codeRunners = []string{"eval", "source", ".", "command", "trap"}

// interpreters run code handed to them in an argument after one of
// inlineCodeFlags: sh -c, node -e, python -c.
var (
	interpreters    = []string{"sh", "bash", "zsh", "dash", "ksh", "fish", "node", "deno", "bun", "ruby", "perl", "php"}
	inlineCodeFlags = []string{"-c", "-e", "--eval", "-p", "--print", "-r"}
)

// inlineCode is true when words, anywhere along them (pnpm exec node -e),
// hand an interpreter code to run.
func inlineCode(words []string) bool {
	for i, w := range words {
		name := w[strings.LastIndex(w, "/")+1:]
		if (slices.Contains(interpreters, name) || strings.HasPrefix(name, "python")) &&
			slices.ContainsFunc(words[i+1:], func(arg string) bool { return slices.Contains(inlineCodeFlags, arg) }) {
			return true
		}
	}
	return false
}

// Single is true when body runs exactly one binary, named literally, with
// builtins around it at most (test -z "$(deadcode ./...)"): no second
// command, no redirect, and no codeRunners or inlineCode to run one unseen.
func Single(body string) bool {
	calls, ok := Calls(body)
	if !ok || len(calls) != 1 || calls[0][0] == "$" || inlineCode(calls[0]) {
		return false
	}
	file, _ := parse(body)
	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.Redirect:
			ok = false
		case *syntax.CallExpr:
			if len(n.Args) > 0 {
				if name, _ := literal(n.Args[0]); slices.Contains(codeRunners, name) {
					ok = false
				}
			}
		}
		return ok
	})
	return ok
}

// Statements is body's top-level commands in order, each printed back as
// shell: statements a newline or && separates come apart, anything else (a
// pipe, ||, a subshell) stays whole. ok is false when body doesn't parse.
func Statements(body string) (stmts []string, ok bool) {
	file, err := parse(body)
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

// GitSubcommand splits git's words past its global options: the
// subcommand, its arguments, and the -C directory.
func GitSubcommand(words []string) (sub string, args []string, dir string) {
	for i := 0; i < len(words); {
		w := words[i]
		switch {
		case w == "-C":
			if i+1 < len(words) {
				dir = words[i+1]
			}
			i += 2
		case slices.Contains([]string{"-c", "--git-dir", "--work-tree", "--namespace", "--config-env", "--super-prefix"}, w):
			i += 2
		case strings.HasPrefix(w, "-"):
			i++
		default:
			return w, words[i+1:], dir
		}
	}
	return "", nil, dir
}

// Calls is every simple command body runs, as its literal words, in order:
// inside pipes, lists, subshells, and command substitutions too, with
// wrappers and the assignments after them dropped, and a find -exec or
// xargs target listed after its command. A command whose first word isn't
// literal is ["$"]; shell builtins aren't listed. ok is false when body
// doesn't parse.
func Calls(body string) (calls [][]string, ok bool) { return walkCalls(body, false) }

// Words is Calls with a word that isn't literal kept as "$" instead of
// ending its command's words: enough to read what a command does past a
// variable argument, never to resolve what it runs.
func Words(body string) (calls [][]string, ok bool) { return walkCalls(body, true) }

func walkCalls(body string, keepVariables bool) (calls [][]string, ok bool) {
	file, err := parse(body)
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
				// A command named by a variable stays ["$"].
				if !keepVariables || len(words) == 0 {
					break
				}
				v = "$"
			}
			words = append(words, v)
		}
		words, _ = unwrap(words)
		switch {
		case len(words) == 0:
			calls = append(calls, []string{"$"})
		case !slices.Contains(builtins, words[0]):
			calls = append(calls, words)
			if inner := execTarget(words); len(inner) > 0 {
				calls = append(calls, inner)
			}
		}
		return true
	})
	return calls, true
}
