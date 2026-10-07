package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/stackctx"
	"github.com/ku5ic/claude-kit/go/internal/tools"
)

// cwdOf is the payload's cwd, which Claude Code always sends, else the
// process's own.
func cwdOf(h *hook.Hook) string {
	if cwd := h.Payload.String("cwd"); cwd != "" {
		return cwd
	}
	cwd, _ := os.Getwd()
	return cwd
}

// projectOf resolves the project for cwd; ok is false for a non-project
// context (home, /, or a name that sanitizes to nothing).
func projectOf(cfg *config.Config, cwd string) (name, root string, ok bool) {
	root, _ = project.Root(cfg, cwd)
	name = project.Name(root)
	switch name {
	case "home", "root", "unknown":
		return name, root, false
	}
	return name, root, true
}

// InjectContext is the SessionStart hook: <repo-context>,
// <required-skills>, <suggested-skills>, and <tooling>. The rules come from
// InjectRules.
// Plain stdout on SessionStart becomes context. Install problems switch it to
// one JSON object, since a hook's stdout is either JSON or text, never both:
// systemMessage shows them to the user, additionalContext carries them and
// the context to Claude.
func InjectContext(h *hook.Hook) error {
	if missing := prerequisites(h.Paths); missing != "" {
		hook.WriteJSON(h.Stdout, map[string]string{
			"systemMessage": "claude-kit guards fail open until this is fixed. Missing: " + missing,
		})
		return nil
	}
	var out strings.Builder
	cfg := h.Config()
	if cfg != nil {
		writeContext(h, cfg, &out)
	}
	var lines []string
	for _, w := range h.Warnings() {
		lines = append(lines, w.String())
	}
	if dir := leftoverRulesLink(h.Paths); dir != "" {
		lines = append(lines, dir+" holds the claude-kit rules, which now ship with the plugin: remove it, or they load twice")
	}
	if len(lines) == 0 {
		fmt.Fprint(h.Stdout, out.String())
		return nil
	}
	notice := "claude-kit install problems:\n" + strings.Join(lines, "\n")
	hook.AddContext(h.Stdout, "SessionStart", notice, notice+"\n"+out.String())
	return nil
}

func writeContext(h *hook.Hook, cfg *config.Config, out *strings.Builder) {
	cwd := cwdOf(h)
	name, root, ok := projectOf(cfg, cwd)
	if !ok {
		return
	}
	cache := stackctx.CacheFile(h.Paths, cfg, name, root)
	stackctx.Refresh(h.Paths, cfg, root, cache)
	report, _ := os.ReadFile(cache)

	if len(report) > 0 {
		scratch, _ := project.Dir(cfg, h.Paths, root, "scratch", false)
		fmt.Fprintf(out, "\n<repo-context>\n%sbranch (at session start): %s\ndirty-files (at session start): %s\nscratch: %s\n</repo-context>\n",
			report, branch(root), dirtyCount(root), scratch)
	}

	required := stackctx.Required(cfg)
	out.WriteString(stackctx.RequiredBlock(required))
	for _, skill := range required {
		h.Log("skills", "required-skill", "cwd", h.Payload.String("cwd"), "skill_file", skill)
	}
	if len(report) > 0 {
		// Logged as surfaced, not loaded, so skills-report can measure
		// whether a suggestion was ever acted on.
		suggested := stackctx.Suggested(cfg, stackctx.Signals(string(report)))
		out.WriteString(stackctx.SuggestedBlock(cfg, suggested))
		for _, skill := range suggested {
			h.Log("skills", "suggested-skill", "cwd", h.Payload.String("cwd"), "skill_file", skill)
		}
	}
	out.WriteString(tooling(cfg, root))
}

// AgentContext is the subagent counterpart: the resolved scratch path (which
// overrides the harness's /tmp scratchpad line), then the same repo context
// and skill blocks, unlogged.
func AgentContext(paths config.Paths, cfg *config.Config, cwd string) string {
	var b strings.Builder
	if scratch, err := project.Dir(cfg, paths, cwd, "scratch", true); err == nil {
		fmt.Fprintf(&b, "<scratch>\npath: %s\n", scratch)
		b.WriteString("Write every file you produce here - reports, plans, previews, logs, downloads, test artifacts, POC scripts.\n" +
			`This overrides the "Scratchpad directory" line in your system prompt: use this path, never the /private/tmp session scratchpad.` + "\n" +
			"Name structured artifacts with `kit scratch-dir <kind> <scope-slug>`, which prints the full path with a real timestamp.\n" +
			"</scratch>\n")
	}
	name, root, ok := projectOf(cfg, cwd)
	if !ok {
		return b.String()
	}
	cache := stackctx.CacheFile(paths, cfg, name, root)
	stackctx.Refresh(paths, cfg, root, cache)
	report, _ := os.ReadFile(cache)
	if len(report) > 0 {
		fmt.Fprintf(&b, "<repo-context>\n%sbranch: %s\ndirty-files: %s\n</repo-context>\n", report, branch(root), dirtyCount(root))
	}
	b.WriteString(stackctx.RequiredBlock(stackctx.Required(cfg)))
	if len(report) > 0 {
		b.WriteString(stackctx.SuggestedBlock(cfg, stackctx.Suggested(cfg, stackctx.Signals(string(report)))))
	}
	return b.String()
}

// InjectSubagentContext is the SubagentStart hook. SubagentStart ignores
// plain stdout and a top-level additionalContext: it reads only
// hookSpecificOutput.additionalContext.
func InjectSubagentContext(h *hook.Hook) error {
	cfg := h.Config()
	if cfg == nil {
		return nil
	}
	context := strings.TrimRight(AgentContext(h.Paths, cfg, cwdOf(h)), "\n")
	if context == "" {
		return nil
	}
	hook.AddContext(h.Stdout, "SubagentStart", "", context)
	return nil
}

func branch(root string) string {
	b, err := project.Branch(root)
	if err != nil {
		return "unknown"
	}
	return b
}

func dirtyCount(root string) string {
	out, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		return "unknown"
	}
	return strconv.Itoa(strings.Count(string(out), "\n"))
}

// tooling is the <tooling> block, computed live: the package manager, then
// per subproject the run form of every task kit.yml's task_providers find
// and of every toolchain check its stacks get (the lists run-checks reads),
// then kit.yml's tools split by whether PATH has them.
func tooling(cfg *config.Config, root string) string {
	var body []string
	if pm := project.ResolvePackageManager(cfg, root); pm != "" {
		body = append(body, "package-manager: "+pm)
	}
	shown, capped := 0, false
	for _, sub := range project.Subprojects(cfg, root) {
		dir, header := root, "tasks:"
		if sub != "." {
			dir, header = filepath.Join(root, sub), "tasks ["+sub+"]:"
		}
		var lines []string
		for _, task := range project.Tasks(cfg, dir) {
			lines = append(lines, task.Cmd)
		}
		for _, tc := range cfg.ToolchainChecks {
			if cfg.HasStack(dir, tc.Stack) && cfg.ToolchainEnabled(tc) {
				if run := tools.ResolveToolchain(cfg, tc, dir, root); run.Words != nil {
					lines = append(lines, tools.ShellJoin(run.Shown))
				}
			}
		}
		if len(lines) == 0 {
			continue
		}
		if sub != "." {
			if shown >= 20 {
				capped = true
				break
			}
			shown++
		}
		body = append(body, header)
		for _, line := range lines {
			body = append(body, "  "+line)
		}
	}
	if capped {
		body = append(body, "(subprojects capped at 20; kit run-checks covers all)")
	}
	// Only the subprojects CI runs in, capped like the list above: planning
	// resolves every tool, too slow for every session start.
	if subs := checks.CISubprojects(cfg, root); len(subs) > 0 {
		var ci []string
		for _, g := range checks.Gates(cfg, root, subs[:min(len(subs), 20)]) {
			if g.CI != "" && g.Skip == "" {
				ci = append(ci, "  "+g.Label)
			}
		}
		if len(ci) > 0 {
			body = append(append(body, "ci gates (run-checks runs these from CI config):"), ci...)
		}
	}
	if len(body) > 0 {
		body = append(body, "checks: `kit run-checks --plan` lists what kit run-checks runs, without running it")
	}

	var available, missing []string
	for _, tool := range cfg.Tools {
		if _, err := exec.LookPath(tool); err == nil {
			available = append(available, tool)
		} else {
			missing = append(missing, tool)
		}
	}
	var cli []string
	if len(available) > 0 {
		cli = append(cli, "available: "+strings.Join(available, ", "))
	}
	if len(missing) > 0 {
		cli = append(cli, "missing: "+strings.Join(missing, ", "))
	}
	if len(body) == 0 && len(cli) == 0 {
		return ""
	}

	var out strings.Builder
	out.WriteString("\n<tooling>\n")
	for _, line := range append(body, cli...) {
		out.WriteString(line)
		out.WriteString("\n")
	}
	if len(body) > 0 {
		out.WriteString("\nguidance: Run scripts only through the package manager named above, prefer these scripts and kit run-checks over direct tool invocation, and never substitute a different package manager.\n")
	}
	out.WriteString("</tooling>\n")
	return out.String()
}

// prerequisites names what the install is missing, "" when nothing is: a
// readable kit.yml. The kit itself needs no tools beyond git and a POSIX
// shell.
func prerequisites(paths config.Paths) string {
	f, err := os.Open(paths.Base)
	if err != nil {
		return "a readable kit.yml at the kit root (reinstall the plugin)"
	}
	f.Close()
	return ""
}
