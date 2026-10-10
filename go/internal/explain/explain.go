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
	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/hooks"
	"github.com/ku5ic/claude-kit/go/internal/stackctx"
)

const usage = `usage: kit explain <what> ...

  bash '<command>'          how guard-bash parses the command, and its decision
  edit <path> [tool]        guard-edit's (and the skills gate's) decision on a
                            Read/Edit/Write of path; tool defaults to Write
  stop [file...]            which checks claim each file, where they run, the
                            exact command, and what blocks; defaults to the
                            files the working tree has changed
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
		return stop(paths, cfg, cwd, args[1:], stdout, stderr)
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

func stop(paths config.Paths, cfg *config.Config, cwd string, files []string, w, stderr io.Writer) int {
	root := git.Toplevel(cwd)
	if root == "" {
		fmt.Fprintln(stderr, "kit explain stop: not inside a git repository")
		return 1
	}
	if len(files) == 0 {
		// The hook is silent on a pass: its last report says what ran and what was skipped.
		if report, err := os.ReadFile(cache.StopReport(paths.CacheDir(), root)); err == nil {
			fmt.Fprintf(w, "last stop-checks run:\n%s\n\n", strings.TrimRight(string(report), "\n"))
		}
		files = changedFiles(root)
		if len(files) == 0 {
			fmt.Fprintln(w, "no changed files; name some: kit explain stop <file>...")
			return 0
		}
	}
	abs := make([]string, len(files))
	for i, f := range files {
		abs[i] = fsx.Abs(cwd, f)
	}
	p := enforce.Stop(cfg, enforce.Options{Root: root, CacheDir: paths.CacheDir()}, abs)
	claimed := map[string]bool{}
	for _, g := range p.Gates {
		fmt.Fprintln(w, g.Label)
		if g.Skip != "" {
			fmt.Fprintf(w, "  skip     %s\n", g.Skip)
			continue
		}
		fmt.Fprintf(w, "  runs in  %s\n", fsx.Rel(root, g.Dir))
		for _, f := range g.Files {
			claimed[f] = true
			fmt.Fprintf(w, "  file     %s\n", fsx.Rel(root, f))
		}
		fmt.Fprintf(w, "  command  %s\n", g.Command())
		for _, bin := range g.Bins {
			fmt.Fprintf(w, "  source   %s (%s)\n", bin.Path, bin.Source)
		}
		blocks := "any failure"
		if g.LintLike() {
			blocks = "findings on changed lines"
		}
		fmt.Fprintf(w, "  blocks   %s\n", blocks)
		fmt.Fprintf(w, "  verdict  %s\n", g.Verdict)
	}
	for i, f := range files {
		if !claimed[fsx.PhysicalPath(abs[i])] {
			fmt.Fprintf(w, "unclaimed  %s (no check applies, or the file is ignored, deleted, or outside the repo)\n", f)
		}
	}
	return 0
}

// changedFiles is every modified or untracked file in the working tree.
func changedFiles(root string) []string {
	changes, err := git.Status(root, true)
	if err != nil {
		return nil
	}
	var files []string
	for _, c := range changes {
		if !strings.Contains(c.XY, "D") {
			files = append(files, filepath.Join(root, c.Path))
		}
	}
	return files
}
