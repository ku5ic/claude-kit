package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// exitUnclassified is a run that left an entry with no verdict: apart from
// any count of failures a command exits with.
const exitUnclassified = 125

// classifyTimeout caps a foreground gap-fill run, waiting included.
const classifyTimeout = 60 * time.Second

func cmdEnforce(e *env, cfg *config.Config, args []string) int {
	if len(args) != 1 || args[0] != "--list" && args[0] != "classify" {
		fmt.Fprintln(e.stderr, "usage: kit enforce --list | classify")
		return 2
	}
	root, _ := project.Root(cfg, e.cwd)
	entries := sources.Entries(cfg, root)
	if args[0] == "--list" {
		for _, entry := range entries {
			fmt.Fprintln(e.stdout, entry)
		}
		return 0
	}
	r := gapfill.Run(cfg, gapfill.Options{Root: root, CacheDir: e.paths.CacheDir(), Entries: entries, Ask: true, Timeout: classifyTimeout})
	for _, entry := range entries {
		key := gapfill.Key(entry)
		answer := r.Skipped[key]
		if v, ok := r.Verdicts[key]; ok {
			answer = verdictLine(v)
		}
		fmt.Fprintln(e.stdout, entry.String()+"\t"+answer)
	}
	for _, m := range r.Managers {
		fmt.Fprintf(e.stdout, "manager\t%s\t%s\t%s\tcites %s\n", m.Dir, m.Manager, m.RunPrefix, m.Cites)
	}
	for _, p := range r.Proposals {
		fmt.Fprintf(e.stdout, "proposal\t%s\t%s\t%s\tevidence %s\n", roleKind(p.Role, p.Kind), p.Dir, p.Command, p.Evidence)
	}
	for _, d := range r.Dropped {
		fmt.Fprintln(e.stdout, "dropped\t"+d)
	}
	if r.Unclassified() > 0 {
		return exitUnclassified
	}
	return 0
}

// verdictLine is v as one column: role:kind, then what else it says.
func verdictLine(v gapfill.Verdict) string {
	parts := []string{roleKind(v.Role, v.Kind)}
	if v.Mutates {
		parts = append(parts, "mutates")
	}
	for _, s := range v.Segments {
		parts = append(parts, fmt.Sprintf("segment %q %s", s.Text, roleKind(s.Role, s.Kind)))
	}
	if v.FileForm != "" {
		parts = append(parts, "files: "+v.FileForm)
	}
	if v.Affected != "" {
		parts = append(parts, "affected: "+v.Affected)
	}
	return strings.Join(parts, "; ")
}

func roleKind(role, kind string) string {
	if kind == "" {
		return role
	}
	return role + ":" + kind
}
