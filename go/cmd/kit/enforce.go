package main

import (
	"fmt"
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
	entries := sources.Entries(cfg, root, project.Subprojects(cfg, root))
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
			answer = v.String()
		}
		fmt.Fprintln(e.stdout, entry.String()+"\t"+answer)
	}
	for _, m := range r.Managers {
		fmt.Fprintf(e.stdout, "manager\t%s\t%s\t%s\tcites %s\n", m.Dir, m.Manager, m.RunPrefix, m.Cites)
	}
	for _, p := range r.Proposals {
		fmt.Fprintf(e.stdout, "proposal\t%s\t%s\t%s\tevidence %s\n", gapfill.RoleKind(p.Role, p.Kind), p.Dir, p.Command, p.Evidence)
	}
	for _, d := range r.Dropped {
		fmt.Fprintln(e.stdout, "dropped\t"+d)
	}
	if r.Unclassified() > 0 {
		return exitUnclassified
	}
	return 0
}
