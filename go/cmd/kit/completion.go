package main

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

// argWords are the words offered after a command; commands taking a file
// or directory fall through to the shell's own path completion.
var argWords = map[string][]string{
	"completion":     {"bash", "zsh"},
	"config":         {"--check"},
	"explain":        {"bash", "edit", "stop"},
	"git-base":       {"--diff", "--log"},
	"project-root":   {"--check"},
	"run-checks":     {"--only"},
	"scratch-rotate": {"--dry-run"},
}

type command struct{ name, desc string }

// usageCommands reads the command list off usage, so completion can't
// drift from it. A description is the text from column 29 on, across the
// command's line and its continuation lines.
func usageCommands() []command {
	const descCol = 29
	var cmds []command
	for _, line := range strings.Split(usage, "\n") {
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		if line[2] != ' ' {
			cmds = append(cmds, command{name: strings.Fields(line)[0]})
		}
		last := &cmds[len(cmds)-1]
		if len(line) > descCol && line[descCol-1] == ' ' {
			last.desc = strings.TrimSpace(last.desc + " " + line[descCol:])
		}
	}
	return cmds
}

// hookNames is every name `kit hook` accepts.
func hookNames() []string {
	names := append(slices.Collect(maps.Keys(singleChecks)), "guard-dispatch")
	slices.Sort(names)
	return names
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
	for _, c := range usageCommands() {
		names = append(names, c.name)
	}
	var b strings.Builder
	b.WriteString("# bash completion for kit; in ~/.bashrc: eval \"$(kit completion bash)\"\n_kit() {\n")
	b.WriteString("  local cur=${COMP_WORDS[COMP_CWORD]}\n")
	fmt.Fprintf(&b, "  if ((COMP_CWORD == 1)); then\n    COMPREPLY=($(compgen -W %q -- \"$cur\"))\n    return\n  fi\n", strings.Join(names, " "))
	b.WriteString("  ((COMP_CWORD == 2)) || return\n  case ${COMP_WORDS[1]} in\n")
	fmt.Fprintf(&b, "  hook) COMPREPLY=($(compgen -W %q -- \"$cur\")) ;;\n", strings.Join(hookNames(), " "))
	for _, name := range slices.Sorted(maps.Keys(argWords)) {
		fmt.Fprintf(&b, "  %s) COMPREPLY=($(compgen -W %q -- \"$cur\")) ;;\n", name, strings.Join(argWords[name], " "))
	}
	b.WriteString("  esac\n}\ncomplete -o bashdefault -o default -F _kit kit\n")
	return b.String()
}

func zshCompletion() string {
	var b strings.Builder
	b.WriteString("#compdef kit\n# zsh completion for kit; in ~/.zshrc after compinit: eval \"$(kit completion zsh)\"\n_kit() {\n  local -a cmds\n  cmds=(\n")
	for _, c := range usageCommands() {
		entry := c.name
		if c.desc != "" {
			entry += ":" + c.desc
		}
		fmt.Fprintf(&b, "    %s\n", zshQuote(entry))
	}
	b.WriteString("  )\n  if ((CURRENT == 2)); then\n    _describe 'kit command' cmds\n    return\n  fi\n")
	b.WriteString("  ((CURRENT == 3)) || return\n  case $words[2] in\n")
	fmt.Fprintf(&b, "  hook) compadd -- %s ;;\n", strings.Join(hookNames(), " "))
	for _, name := range slices.Sorted(maps.Keys(argWords)) {
		fmt.Fprintf(&b, "  %s) compadd -- %s ;;\n", name, strings.Join(argWords[name], " "))
	}
	b.WriteString("  blast-radius) _files ;;\n  subprojects|tasks) _files -/ ;;\n  esac\n}\ncompdef _kit kit\n")
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
