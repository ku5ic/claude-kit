package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/stackctx"
	"github.com/ku5ic/claude-kit/go/internal/tools"
)

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
		h.Notify("claude-kit guards fail open until this is fixed. Missing: " + missing)
		return nil
	}
	if !h.DryRun {
		cache.Prune(h.Paths.CacheDir(), time.Now())
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
		h.Print(out.String())
		return nil
	}
	notice := "claude-kit install problems:\n" + strings.Join(lines, "\n")
	h.Notify(notice)
	h.AddContext("SessionStart", notice+"\n"+out.String())
	return nil
}

func writeContext(h *hook.Hook, cfg *config.Config, out *strings.Builder) {
	cwd := h.Payload.Cwd()
	name, root, ok := projectOf(cfg, cwd)
	if !ok {
		return
	}
	ctx := stackctx.Build(h.Paths, cfg, name, root, h.Payload.SessionID())
	if ctx.Report != "" {
		scratch, _ := project.Dir(cfg, h.Paths, root, "scratch", false)
		fmt.Fprintf(out, "\n<repo-context>\n%sbranch (at session start): %s\ndirty-files (at session start): %s\nscratch: %s\n</repo-context>\n",
			ctx.Report, branch(root), dirtyCount(root), scratch)
	}
	out.WriteString(ctx.RequiredBlock())
	for _, skill := range ctx.Required {
		h.Log(kitlog.Skills, kitlog.EventRequiredSkill, "cwd", h.Payload.String("cwd"), "skill_file", skill)
	}
	// Logged as surfaced, not loaded, so skills-report can measure whether a
	// suggestion was ever acted on.
	out.WriteString(stackctx.SuggestedBlock(cfg, ctx.Suggested))
	for _, skill := range ctx.Suggested {
		h.Log(kitlog.Skills, kitlog.EventSuggestedSkill, "cwd", h.Payload.String("cwd"), "skill_file", skill)
	}
	out.WriteString(tooling(cfg, root))
}

// AgentContext is the subagent counterpart: the resolved scratch path (which
// overrides the harness's /tmp scratchpad line), then the same repo context
// and skill blocks, unlogged.
func AgentContext(paths config.Paths, cfg *config.Config, cwd, session string) string {
	var b strings.Builder
	if scratch, err := project.Dir(cfg, paths, cwd, "scratch", true); err == nil {
		fmt.Fprintf(&b, "<scratch>\npath: %s\n", scratch)
		b.WriteString("Write every file you produce here - reports, previews, logs, downloads, test artifacts, POC scripts.\n" +
			`This overrides the "Scratchpad directory" line in your system prompt: use this path, never the /private/tmp session scratchpad.` + "\n" +
			"Name structured artifacts with `kit scratch-dir <kind> <scope-slug>`, which prints the full path with a real timestamp.\n" +
			"</scratch>\n")
	}
	name, root, ok := projectOf(cfg, cwd)
	if !ok {
		return b.String()
	}
	ctx := stackctx.Build(paths, cfg, name, root, session)
	if ctx.Report != "" {
		fmt.Fprintf(&b, "<repo-context>\n%sbranch: %s\ndirty-files: %s\n</repo-context>\n", ctx.Report, branch(root), dirtyCount(root))
	}
	b.WriteString(ctx.RequiredBlock())
	b.WriteString(stackctx.SuggestedBlock(cfg, ctx.Suggested))
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
	context := strings.TrimRight(AgentContext(h.Paths, cfg, h.Payload.Cwd(), h.Payload.SessionID()), "\n")
	if context == "" {
		return nil
	}
	h.AddContext("SubagentStart", context)
	return nil
}

func branch(root string) string {
	b, err := git.Branch(root)
	if err != nil {
		return "unknown"
	}
	return b
}

func dirtyCount(root string) string {
	changes, err := git.Status(root, false)
	if err != nil {
		return "unknown"
	}
	return strconv.Itoa(len(changes))
}

// tooling is the <tooling> block, computed live: the package manager, then
// per subproject the run form of every task kit.yml's task_providers find
// and of every toolchain check its stacks get (the lists run-checks reads),
// then kit.yml's tools split by whether PATH has them.
func tooling(cfg *config.Config, root string) string {
	var body []string
	pm := project.ResolvePackageManager(cfg, root)
	if pm != "" {
		body = append(body, "package-manager: "+pm)
	}
	body = append(body, taskLines(cfg, root)...)
	// Labels only: resolving every tool is too slow for every session start.
	if gates := checks.CIGates(cfg, root); len(gates) > 0 {
		body = append(body, "ci gates (from CI config; run-checks runs each whose tool the project has):")
		for _, label := range gates {
			body = append(body, "  "+label)
		}
	}
	if len(body) > 0 {
		body = append(body, "checks: `kit run-checks --plan` lists what kit run-checks runs, without running it")
	}
	cli := cliLines(cfg)
	if len(body) == 0 && len(cli) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("\n<tooling>\n")
	for _, line := range append(body, cli...) {
		out.WriteString(line)
		out.WriteString("\n")
	}
	if pm != "" {
		out.WriteString("\nguidance: Run scripts only through the package manager named above, prefer these scripts and kit run-checks over direct tool invocation, and never substitute a different package manager.\n")
	}
	out.WriteString("</tooling>\n")
	return out.String()
}

// taskLines is each subproject's task commands and toolchain checks, under
// a "tasks [sub]:" header, past 20 subprojects capped with a note.
func taskLines(cfg *config.Config, root string) []string {
	var out []string
	shown := 0
	for _, sub := range project.Subprojects(cfg, root) {
		dir := filepath.Join(root, sub)
		var lines []string
		for _, task := range project.Tasks(cfg, dir) {
			lines = append(lines, task.Cmd)
		}
		// Without the planner's state, a check another gate covers still shows.
		for _, tc := range cfg.ToolchainChecks {
			if cfg.HasStack(dir, tc.Stack) && checks.ToolchainSkip(cfg, tc, sub, nil) == "" {
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
				return append(out, "(subprojects capped at 20; kit run-checks covers all)")
			}
			shown++
		}
		out = append(out, "tasks"+project.SubLabel(sub)+":")
		for _, line := range lines {
			out = append(out, "  "+line)
		}
	}
	return out
}

// cliLines says which of kit.yml's CLI tools are on PATH.
func cliLines(cfg *config.Config) []string {
	var available, missing, out []string
	for _, tool := range cfg.Tools {
		if _, err := exec.LookPath(tool); err == nil {
			available = append(available, tool)
		} else {
			missing = append(missing, tool)
		}
	}
	if len(available) > 0 {
		out = append(out, "available: "+strings.Join(available, ", "))
	}
	if len(missing) > 0 {
		out = append(out, "missing: "+strings.Join(missing, ", "))
	}
	return out
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
