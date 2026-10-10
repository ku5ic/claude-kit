package main

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/project"
	gaterun "github.com/ku5ic/claude-kit/go/internal/run"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// exitUnclassified is a run that left an entry with no verdict: apart from
// any count of failures a command exits with.
const exitUnclassified = 125

// runGates is run-checks on the enforcement plan: --plan prints it, else it
// runs, exiting with the failure count, or exitUnclassified when nothing
// failed but an entry has no verdict.
func runGates(e *env, cfg *config.Config, root string, a runChecksArgs) int {
	p := enforce.Build(cfg, enforce.Options{Root: root, CacheDir: e.paths.CacheDir(), Only: a.only, Ask: true, Timeout: classifyTimeout})
	if a.plan {
		p.Print(e.stdout)
		return 0
	}
	switch r := gaterun.Gates(p, e.paths.CacheDir(), e.stdout); {
	case r.Fail > 0:
		return min(r.Fail, exitUnclassified-1)
	case p.Unclassified > 0:
		return exitUnclassified
	}
	return 0
}

// cmdGates is kit gates reset [--restore]: it forgets the gates seen
// changing files, so they run again; --restore first puts those files back.
func cmdGates(e *env, args []string) int {
	restore := len(args) == 2 && args[1] == "--restore"
	if len(args) == 0 || args[0] != "reset" || len(args) > 1 && !restore {
		fmt.Fprintln(e.stderr, "usage: kit gates reset [--restore]")
		return 2
	}
	root := cmp.Or(git.Toplevel(e.cwd), e.cwd)
	marks := enforce.Marks(e.paths.CacheDir(), root)
	if len(marks) == 0 {
		fmt.Fprintln(e.stdout, "no gate was seen changing files")
		return 0
	}
	if restore {
		for _, m := range marks {
			restored, err := gaterun.Restore(root, m)
			if err != nil {
				fmt.Fprintf(e.stderr, "kit gates: restoring what %s changed: %v\n", m.Label, err)
				return 1
			}
			if len(restored) > 0 {
				fmt.Fprintf(e.stdout, "restored %s\n", strings.Join(restored, ", "))
			}
		}
	}
	if err := enforce.ResetMarks(e.paths.CacheDir(), root); err != nil {
		fmt.Fprintln(e.stderr, "kit gates:", err)
		return 1
	}
	fmt.Fprintf(e.stdout, "%d gate(s) will run again\n", len(marks))
	return 0
}

// classifyTimeout caps a foreground gap-fill run, waiting included.
const classifyTimeout = 60 * time.Second

func cmdEnforce(e *env, cfg *config.Config, args []string) int {
	if len(args) != 1 || args[0] != "--list" && args[0] != "classify" {
		fmt.Fprintln(e.stderr, "usage: kit enforce --list | classify")
		return 2
	}
	root, _ := project.Root(e.cwd)
	entries := sources.Entries(cfg, root, project.Subprojects(root))
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
