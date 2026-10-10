// Package explain is `kit explain`: it shows why a guard or the Stop hook
// would decide what it decides, without logging, blocking, or running
// anything.
package explain

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/bashguard"
	"github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/hooks"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/stackctx"
)

const usage = `usage: kit explain <what> ...

  bash '<command>'          how guard-bash parses the command, and its decision
  edit <path> [tool]        guard-edit's (and the skills gate's) decision on a
                            Read/Edit/Write of path; tool defaults to Write
  stop [file...]            which file checks claim each file, where they run,
                            and the exact command; defaults to the files the
                            working tree has changed
`

// Run dispatches kit explain.
func Run(paths config.Paths, cfg *config.Config, cwd string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "bash":
		if len(args) != 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		return bash(paths, cfg, cwd, args[1], stdout)
	case "edit":
		if len(args) < 2 || len(args) > 3 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		tool := "Write"
		if len(args) == 3 {
			tool = args[2]
		}
		return edit(paths, cfg, cwd, args[1], tool, stdout)
	case "stop":
		return stop(cfg, cwd, args[1:], stdout, stderr)
	}
	fmt.Fprint(stderr, usage)
	return 2
}

// dryHook is a hook invocation that logs nothing and prints nothing.
func dryHook(paths config.Paths, cfg *config.Config, name string, payload map[string]any) *hook.Hook {
	raw, _ := json.Marshal(payload)
	h := &hook.Hook{Name: name, Payload: hook.ParsePayload(raw), Paths: paths, Stdout: io.Discard, Stderr: io.Discard, Home: os.Getenv("HOME"), DryRun: true}
	h.SetConfig(cfg)
	return h
}

// verdict prints a check's outcome: block with its rule, ask or allow with
// the reason, or pass.
func verdict(w io.Writer, name string, err error, h *hook.Hook) {
	if b, ok := err.(*hook.Blocked); ok {
		rule := b.Rule
		if rule == "" {
			rule = "(no slug)"
		}
		fmt.Fprintf(w, "%s: block  %s\n", name, rule)
		for line := range strings.SplitSeq(b.Reason, "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
		return
	}
	if err != nil {
		fmt.Fprintf(w, "%s: fails open (%v)\n", name, err)
		return
	}
	if decision, reason := h.Decision(); decision != "" {
		fmt.Fprintf(w, "%s: %s\n  %s\n", name, decision, reason)
		return
	}
	fmt.Fprintf(w, "%s: pass\n", name)
}

func bash(paths config.Paths, cfg *config.Config, cwd, command string, w io.Writer) int {
	fmt.Fprintln(w, "parsed (inner substitutions first, as they run):")
	for i, seg := range bashguard.Segments(command) {
		var calls []string
		for _, c := range seg.Calls {
			var words []string
			for _, word := range c.Words {
				words = append(words, fmt.Sprintf("%q", word.Value))
			}
			for _, r := range c.Redirs {
				words = append(words, r.Op+fmt.Sprintf("%q", r.Target.Value))
			}
			calls = append(calls, strings.Join(words, " "))
		}
		fmt.Fprintf(w, "  %d  %s\n", i+1, strings.Join(calls, "  |  "))
	}
	h := dryHook(paths, cfg, "guard-bash", map[string]any{
		"tool_name": "Bash", "cwd": cwd, "tool_input": map[string]any{"command": command},
	})
	verdict(w, "guard-bash", bashguard.Check(h), h)
	return 0
}

func edit(paths config.Paths, cfg *config.Config, cwd, path, tool string, w io.Writer) int {
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	payload := map[string]any{"tool_name": tool, "session_id": "explain", "cwd": cwd, "tool_input": map[string]any{"file_path": path}}
	h := dryHook(paths, cfg, "guard-edit", payload)
	verdict(w, "guard-edit", hooks.GuardEdit(h), h)
	h = dryHook(paths, cfg, "guard-skills", payload)
	name := "skills gate"
	if !stackctx.SkillsEnforced() {
		name += " (off: CLAUDE_GUARD_SKILLS isn't 1)"
	}
	verdict(w, name, hooks.GuardSkills(h), h)
	return 0
}

func stop(cfg *config.Config, cwd string, files []string, w, stderr io.Writer) int {
	root := project.Toplevel(cwd)
	if root == "" {
		fmt.Fprintln(stderr, "kit explain stop: not inside a git repository")
		return 1
	}
	if len(files) == 0 {
		files = changedFiles(root)
		if len(files) == 0 {
			fmt.Fprintln(w, "no changed files; name some: kit explain stop <file>...")
			return 0
		}
	}
	groups := checks.Plan(cfg, root, cwd, files)
	claimed := map[string]bool{}
	for _, g := range groups {
		fmt.Fprintf(w, "%s  (%s)\n", g.Adapter.Name, g.Why)
		fmt.Fprintf(w, "  runs in  %s\n", g.Dir)
		for _, f := range g.Files {
			claimed[f] = true
			fmt.Fprintf(w, "  file     %s\n", project.Rel(root, f))
		}
		if g.Skip != "" {
			fmt.Fprintf(w, "  skip     %s\n", g.Skip)
			continue
		}
		if g.Derived.Source != "" {
			carried := append(append([]string{}, g.Derived.Env...), g.Derived.Flags...)
			what := "no flags on the carry list"
			if len(carried) > 0 {
				what = "carries " + strings.Join(carried, " ")
			}
			fmt.Fprintf(w, "  from     %s (%s)\n", g.Derived.Source, what)
		}
		fmt.Fprintf(w, "  command  %s\n", strings.Join(g.Words, " "))
		fmt.Fprintf(w, "  source   %s\n", g.Bin.Origin())
		blocks := "any failure (whole file)"
		if g.Adapter.Findings != nil {
			blocks = "findings on changed lines"
		}
		fmt.Fprintf(w, "  blocks   %s\n", blocks)
	}
	for _, f := range files {
		abs := f
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(cwd, f)
		}
		if !claimed[fsx.PhysicalPath(abs)] {
			fmt.Fprintf(w, "unclaimed  %s (no check applies, or the file is ignored, deleted, or outside the repo)\n", f)
		}
	}
	return 0
}

// changedFiles is every modified or untracked file in the working tree.
func changedFiles(root string) []string {
	out, err := git.Output(root, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil
	}
	var files []string
	for line := range strings.SplitSeq(out, "\n") {
		if len(line) < 4 || strings.Contains(line[:2], "D") {
			continue
		}
		path := line[3:]
		if _, after, ok := strings.Cut(path, " -> "); ok {
			path = after
		}
		files = append(files, filepath.Join(root, path))
	}
	return files
}
