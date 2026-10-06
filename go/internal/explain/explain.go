// Package explain is `kit explain`: it shows why a guard or the Stop hook
// would decide what it decides, without logging, blocking, or running
// anything.
package explain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/bashguard"
	"github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/hooks"
	"github.com/ku5ic/claude-kit/go/internal/project"
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

// dryHook is a hook invocation that logs nothing, with its output captured.
func dryHook(paths config.Paths, cfg *config.Config, name string, payload map[string]any) (*hook.Hook, *bytes.Buffer, *bytes.Buffer) {
	raw, _ := json.Marshal(payload)
	var out, errs bytes.Buffer
	h := &hook.Hook{Name: name, Payload: hook.ParsePayload(raw), Paths: paths, Stdout: &out, Stderr: &errs, DryRun: true}
	h.SetConfig(cfg)
	return h, &out, &errs
}

// verdict prints a check's outcome: block with its rule, ask or allow with
// the reason, or pass.
func verdict(w io.Writer, name string, err error, out *bytes.Buffer) {
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
	var decision struct {
		HookSpecificOutput struct {
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if json.Unmarshal(out.Bytes(), &decision) == nil && decision.HookSpecificOutput.PermissionDecision != "" {
		fmt.Fprintf(w, "%s: %s\n  %s\n", name, decision.HookSpecificOutput.PermissionDecision, decision.HookSpecificOutput.PermissionDecisionReason)
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
	h, out, _ := dryHook(paths, cfg, "guard-bash.sh", map[string]any{
		"tool_name": "Bash", "cwd": cwd, "tool_input": map[string]any{"command": command},
	})
	verdict(w, "guard-bash", bashguard.Check(h), out)
	return 0
}

func edit(paths config.Paths, cfg *config.Config, cwd, path, tool string, w io.Writer) int {
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	payload := map[string]any{"tool_name": tool, "session_id": "explain", "cwd": cwd, "tool_input": map[string]any{"file_path": path}}
	h, out, _ := dryHook(paths, cfg, "guard-edit.sh", payload)
	verdict(w, "guard-edit", hooks.GuardEdit(h), out)
	h, out, _ = dryHook(paths, cfg, "guard-skills.sh", payload)
	name := "skills gate"
	if os.Getenv("CLAUDE_GUARD_SKILLS") != "1" {
		name += " (off: CLAUDE_GUARD_SKILLS isn't 1)"
	}
	verdict(w, name, hooks.GuardSkills(h), out)
	return 0
}

func stop(cfg *config.Config, cwd string, files []string, w, stderr io.Writer) int {
	top := project.Toplevel(cwd)
	if top == "" {
		fmt.Fprintln(stderr, "kit explain stop: not inside a git repository")
		return 1
	}
	root := project.PhysicalPath(top)
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
			fmt.Fprintf(w, "  file     %s\n", strings.TrimPrefix(f, root+"/"))
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
		if !claimed[project.PhysicalPath(abs)] {
			fmt.Fprintf(w, "unclaimed  %s (no check applies, or the file is ignored, deleted, or outside the repo)\n", f)
		}
	}
	return 0
}

// changedFiles is every modified or untracked file in the working tree.
func changedFiles(root string) []string {
	out, err := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=all").Output()
	if err != nil {
		return nil
	}
	var files []string
	for line := range strings.SplitSeq(string(out), "\n") {
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
