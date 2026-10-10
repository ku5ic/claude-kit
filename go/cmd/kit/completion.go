package main

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/kitcmd"
)

// argCommands are the commands with completion words or a path argument,
// sorted by name.
func argCommands() []kitcmd.Command {
	var out []kitcmd.Command
	for _, c := range kitcmd.Commands {
		if len(c.Words) > 0 || c.Path != "" {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b kitcmd.Command) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// hookNames is every name `kit hook` accepts.
func hookNames() []string {
	return slices.Sorted(maps.Keys(registry))
}

func cmdCompletion(args []string, stdout, stderr io.Writer) int {
	shell := ""
	if len(args) == 1 {
		shell = args[0]
	}
	switch shell {
	case "bash":
		fmt.Fprint(stdout, bashCompletion())
	case "zsh":
		fmt.Fprint(stdout, zshCompletion())
	default:
		fmt.Fprintln(stderr, "usage: kit completion bash|zsh")
		return 2
	}
	return 0
}

func bashCompletion() string {
	var names []string
	for _, c := range kitcmd.Commands {
		names = append(names, c.Name)
	}
	var b strings.Builder
	b.WriteString("# bash completion for kit; in ~/.bashrc: eval \"$(kit completion bash)\"\n_kit() {\n")
	b.WriteString("  local cur=${COMP_WORDS[COMP_CWORD]}\n")
	fmt.Fprintf(&b, "  if ((COMP_CWORD == 1)); then\n    COMPREPLY=($(compgen -W %q -- \"$cur\"))\n    return\n  fi\n", strings.Join(names, " "))
	b.WriteString("  ((COMP_CWORD == 2)) || return\n  case ${COMP_WORDS[1]} in\n")
	fmt.Fprintf(&b, "  hook) COMPREPLY=($(compgen -W %q -- \"$cur\")) ;;\n", strings.Join(hookNames(), " "))
	for _, c := range argCommands() {
		if len(c.Words) > 0 {
			fmt.Fprintf(&b, "  %s) COMPREPLY=($(compgen -W %q -- \"$cur\")) ;;\n", c.Name, strings.Join(c.Words, " "))
		}
	}
	for _, c := range argCommands() {
		if c.Path != "" {
			fmt.Fprintf(&b, "  %s) local IFS=$'\\n'; COMPREPLY=($(compgen -%c -- \"$cur\")) ;;\n", c.Name, c.Path[0])
		}
	}
	b.WriteString("  esac\n}\ncomplete -o filenames -F _kit kit\n")
	return b.String()
}

func zshCompletion() string {
	var b strings.Builder
	b.WriteString("#compdef kit\n# zsh completion for kit; in ~/.zshrc after compinit: eval \"$(kit completion zsh)\"\n_kit() {\n  local -a cmds\n  cmds=(\n")
	for _, c := range kitcmd.Commands {
		entry := c.Name
		if c.Desc != "" {
			entry += ":" + strings.ReplaceAll(c.Desc, "\n", " ")
		}
		fmt.Fprintf(&b, "    %s\n", zshQuote(entry))
	}
	b.WriteString("  )\n  if ((CURRENT == 2)); then\n    _describe 'kit command' cmds\n    return\n  fi\n")
	b.WriteString("  ((CURRENT == 3)) || return\n  case $words[2] in\n")
	fmt.Fprintf(&b, "  hook) compadd -- %s ;;\n", strings.Join(hookNames(), " "))
	for _, c := range argCommands() {
		if len(c.Words) > 0 {
			fmt.Fprintf(&b, "  %s) compadd -- %s ;;\n", c.Name, strings.Join(c.Words, " "))
		}
	}
	for _, c := range argCommands() {
		switch c.Path {
		case "file":
			fmt.Fprintf(&b, "  %s) _files ;;\n", c.Name)
		case "dir":
			fmt.Fprintf(&b, "  %s) _files -/ ;;\n", c.Name)
		}
	}
	b.WriteString("  esac\n}\ncompdef _kit kit\n")
	return b.String()
}

// zshQuote single-quotes s, escaping the colons _describe splits on except
// the first, which separates name from description.
func zshQuote(s string) string {
	name, desc, found := strings.Cut(s, ":")
	if found {
		s = name + ":" + strings.ReplaceAll(desc, ":", `\:`)
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
