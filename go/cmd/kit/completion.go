package main

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

// argWords are the words offered after a command.
var argWords = map[string][]string{
	"completion":     {"bash", "zsh"},
	"config":         {"--check"},
	"explain":        {"bash", "edit", "stop"},
	"git-base":       {"--diff", "--log"},
	"project-root":   {"--check"},
	"run-checks":     {"--plan", "--only"},
	"scratch-rotate": {"--dry-run"},
}

// pathArgs are the commands whose argument is a path, true when only a
// directory fits; every other argument gets no path completion.
var pathArgs = map[string]bool{
	"blast-radius": false,
	"subprojects":  true,
	"tasks":        true,
}

type command struct{ name, desc string }

// usageCommands reads the command list off usage, so completion can't
// drift from it.
func usageCommands() []command { return parseUsage(usage) }

// parseUsage reads commands from text: a command line is indented two
// spaces, its description follows the first run of two or more spaces, and
// a deeper-indented line continues the previous command's description.
func parseUsage(text string) []command {
	var cmds []command
	for line := range strings.SplitSeq(text, "\n") {
		rest, ok := strings.CutPrefix(line, "  ")
		if !ok || strings.TrimSpace(rest) == "" {
			continue
		}
		var desc string
		if rest[0] == ' ' {
			if len(cmds) == 0 {
				continue
			}
			desc = rest
		} else {
			spec := rest
			if i := strings.Index(rest, "  "); i >= 0 {
				spec, desc = rest[:i], rest[i:]
			}
			cmds = append(cmds, command{name: strings.Fields(spec)[0]})
		}
		last := &cmds[len(cmds)-1]
		last.desc = strings.TrimSpace(last.desc + " " + strings.TrimSpace(desc))
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
	for _, name := range slices.Sorted(maps.Keys(pathArgs)) {
		kind := "-f"
		if pathArgs[name] {
			kind = "-d"
		}
		fmt.Fprintf(&b, "  %s) local IFS=$'\\n'; COMPREPLY=($(compgen %s -- \"$cur\")) ;;\n", name, kind)
	}
	b.WriteString("  esac\n}\ncomplete -o filenames -F _kit kit\n")
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
	for _, name := range slices.Sorted(maps.Keys(pathArgs)) {
		files := "_files"
		if pathArgs[name] {
			files = "_files -/"
		}
		fmt.Fprintf(&b, "  %s) %s ;;\n", name, files)
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
