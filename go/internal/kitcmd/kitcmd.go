// Package kitcmd is the table of kit's subcommands: the usage text,
// completion, and the bash guard's read-only allow list all read it. The
// commands' run functions live in cmd/kit.
package kitcmd

import (
	"strings"
)

// Command is one subcommand. Desc's lines are the usage text's lines.
type Command struct {
	Name, Args, Desc string
	Words            []string // completion words for its first argument
	Path             string   // its argument is a path: "file" or "dir"
	// ReadOnly: it only reads state or creates the scratch
	// directory, so a lone call needs no permission prompt. Never
	// run-checks: it runs project scripts.
	ReadOnly bool
}

var Commands = []Command{
	{Name: "config", Args: "[--check]", Desc: "print the effective kit.yml (base + overlay);\n--check prints only warnings, exits 1 on any", Words: []string{"--check"}},
	{Name: "subprojects", Args: "[root]", Desc: `".", then each subproject directory, sorted`, Path: "dir"},
	{Name: "tasks", Args: "[dir]", Desc: "provider, stack, task, command (tab-separated)", Path: "dir"},
	{Name: "project-root", Args: "[--check]", Desc: "the project root; --check: exit 1 if unanchored", Words: []string{"--check"}, ReadOnly: true},
	{Name: "scratch-dir", Args: "[kind [slug]]", Desc: "scratch directory, or a report path in it", ReadOnly: true},
	{Name: "detect-stack", Desc: "compact stack report", ReadOnly: true},
	{Name: "run-checks", Args: "[--plan] [--only sub...]", Desc: "every declared check, in every subproject;\nexits with the failure count. --plan lists\nthem, with commands, without running any", Words: []string{"--plan", "--only"}},
	{Name: "enforce", Args: "--list", Desc: "what the project's configs say to run: source,\nfile, stage, dir, name, files, command", Words: []string{"--list"}},
	{Name: "git-base", Args: "[--diff|--log] [base] [flags] [-- paths]", Words: []string{"--diff", "--log"}, ReadOnly: true},
	{Name: "explain", Args: "bash|edit|stop ...", Desc: "why a guard or the Stop hook decides what it\ndoes; logs, blocks, and runs nothing", Words: []string{"bash", "edit", "stop"}},
	{Name: "blast-radius", Args: "<file> [symbol]", Desc: "the files that import <file>", Path: "file", ReadOnly: true},
	{Name: "scratch-rotate", Args: "[days] [--dry-run]", Desc: "prune old scratch artifacts and caches", Words: []string{"--dry-run"}},
	{Name: "hook", Args: "<name>", Desc: "run a Claude Code hook; payload on stdin"},
	{Name: "statusline", Desc: "the statusLine rows; payload on stdin"},
	{Name: "subagent-statusline", Desc: "subagentStatusLine JSON lines; payload on stdin"},
	{Name: "version", Desc: "the plugin version this binary was built for"},
	{Name: "completion", Args: "bash|zsh", Desc: "a shell completion script for kit", Words: []string{"bash", "zsh"}},
}

// descColumn is where a description starts; a longer name and args put it
// on the lines below.
const descColumn = 29

// Usage is kit's usage text.
func Usage() string {
	var b strings.Builder
	b.WriteString("usage: kit <command> [args]\n\n")
	indent := strings.Repeat(" ", descColumn)
	for _, c := range Commands {
		spec := "  " + strings.TrimSpace(c.Name+" "+c.Args)
		lines := strings.Split(c.Desc, "\n")
		if c.Desc == "" {
			lines = nil
		}
		if len(spec)+2 <= descColumn && len(lines) > 0 {
			b.WriteString(spec + strings.Repeat(" ", descColumn-len(spec)) + lines[0] + "\n")
			lines = lines[1:]
		} else {
			b.WriteString(spec + "\n")
		}
		for _, l := range lines {
			b.WriteString(indent + l + "\n")
		}
	}
	return b.String()
}

// ReadOnly is true for a command a lone call of needs no prompt.
func ReadOnly(name string) bool {
	for _, c := range Commands {
		if c.Name == name {
			return c.ReadOnly
		}
	}
	return false
}
