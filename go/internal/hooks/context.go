package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/stackctx"
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

// InjectContext is the SessionStart hook: prerequisite warnings, then
// <repo-context>, <required-skills>, <suggested-skills>, and <tooling>.
// Plain stdout on SessionStart becomes context.
func InjectContext(h *hook.Hook) error {
	if missing := prerequisites(h.Paths); missing != "" {
		hook.WriteJSON(h.Stdout, map[string]string{
			"systemMessage": "claude-kit guards fail open until this is fixed. Missing: " + missing,
		})
		return nil
	}
	cfg := h.Config()
	if cfg == nil {
		return nil
	}
	cwd := cwdOf(h)
	name, root, ok := projectOf(cfg, cwd)
	if !ok {
		return nil
	}
	cache := stackctx.CacheFile(h.Paths, cfg, name, root)
	stackctx.Refresh(h.Paths, cfg, root, cache)
	report, _ := os.ReadFile(cache)

	if len(report) > 0 {
		scratch, _ := project.Dir(cfg, h.Paths, root, "scratch", false)
		fmt.Fprint(h.Stdout, "\n<repo-context>\n"+string(report)+
			"branch (at session start): "+branch(root)+"\n"+
			"dirty-files (at session start): "+dirtyCount(root)+"\n"+
			"scratch: "+scratch+"\n</repo-context>\n")
	}

	required := stackctx.Required(cfg)
	fmt.Fprint(h.Stdout, stackctx.RequiredBlock(required))
	for _, skill := range required {
		h.Log("skills", "required-skill", "cwd", h.Payload.String("cwd"), "skill_file", skill)
	}
	if len(report) > 0 {
		// Logged as surfaced, not loaded, so skills-report can measure
		// whether a suggestion was ever acted on.
		suggested := stackctx.Suggested(cfg, stackctx.Signals(string(report)))
		fmt.Fprint(h.Stdout, stackctx.SuggestedBlock(cfg, suggested))
		for _, skill := range suggested {
			h.Log("skills", "suggested-skill", "cwd", h.Payload.String("cwd"), "skill_file", skill)
		}
	}
	fmt.Fprint(h.Stdout, tooling(cfg, root))
	return nil
}

// AgentContext is the subagent counterpart: the resolved scratch path (which
// overrides the harness's /tmp scratchpad line), then the same repo context
// and skill blocks, unlogged.
func AgentContext(paths config.Paths, cfg *config.Config, cwd string) string {
	var b strings.Builder
	if scratch, err := project.Dir(cfg, paths, cwd, "scratch", true); err == nil {
		b.WriteString("<scratch>\npath: " + scratch + "\n" +
			"Write every file you produce here - reports, plans, previews, logs, downloads, test artifacts, POC scripts.\n" +
			`This overrides the "Scratchpad directory" line in your system prompt: use this path, never the /private/tmp session scratchpad.` + "\n" +
			"Name structured artifacts with `scratch-dir.sh <kind> <scope-slug>`, which prints the full path with a real timestamp.\n" +
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
		b.WriteString("<repo-context>\n" + string(report) +
			"branch: " + branch(root) + "\n" +
			"dirty-files: " + dirtyCount(root) + "\n</repo-context>\n")
	}
	b.WriteString(stackctx.RequiredBlock(stackctx.Required(cfg)))
	if len(report) > 0 {
		b.WriteString(stackctx.SuggestedBlock(cfg, stackctx.Suggested(cfg, stackctx.Signals(string(report)))))
	}
	return b.String()
}

// InjectSubagentContext is the SubagentStart hook. SubagentStart takes
// additionalContext in JSON, not plain stdout, so agent-context's text is
// wrapped.
func InjectSubagentContext(h *hook.Hook) error {
	cfg := h.Config()
	if cfg == nil {
		return nil
	}
	context := strings.TrimRight(AgentContext(h.Paths, cfg, cwdOf(h)), "\n")
	if context == "" {
		return nil
	}
	hook.WriteJSON(h.Stdout, map[string]string{"additionalContext": context})
	return nil
}

func branch(root string) string {
	out, err := exec.Command("git", "-C", root, "branch", "--show-current").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
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
			if cfg.HasStack(dir, tc.Stack) {
				if cmd, skip := project.ToolchainCmd(tc, dir); skip == "" {
					lines = append(lines, cmd)
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
		body = append(body, "(subprojects capped at 20; run-checks.sh covers all)")
	}

	var available, missing []string
	for _, tool := range cfg.Tools {
		if _, err := exec.LookPath(tool); err == nil {
			available = append(available, tool)
		} else {
			missing = append(missing, tool)
		}
	}
	var tools []string
	if len(available) > 0 {
		tools = append(tools, "available: "+strings.Join(available, ", "))
	}
	if len(missing) > 0 {
		tools = append(tools, "missing: "+strings.Join(missing, ", "))
	}
	if len(body) == 0 && len(tools) == 0 {
		return ""
	}

	out := "\n<tooling>\n"
	for _, line := range append(body, tools...) {
		out += line + "\n"
	}
	if len(body) > 0 {
		out += "\nguidance: Run scripts only through the package manager named above, prefer these scripts and run-checks.sh over direct tool invocation, and never substitute a different package manager.\n"
	}
	return out + "</tooling>\n"
}

// prerequisites names what the install is missing, "; "-separated, "" when
// nothing is: the kit rules linked and a readable kit.yml. The kit itself
// needs no tools beyond git and a POSIX shell.
func prerequisites(paths config.Paths) string {
	var missing []string
	if !rulesLinked(paths) {
		missing = append(missing, "the kit rules linked under ~/.claude/rules (run ~/.claude/plugins/marketplaces/ku5ic/install-rules.sh)")
	}
	if f, err := os.Open(paths.Base); err != nil {
		missing = append(missing, "a readable kit.yml at the kit root (reinstall the plugin)")
	} else {
		f.Close()
	}
	return strings.Join(missing, "; ")
}

// rulesLinked is true when some directory under <home>/rules is the kit's
// rules dir, or a copy holding every rule file it has: a plugin's hooks run
// from the versioned plugin cache while install-rules.sh links the
// marketplace clone, so the two paths never match.
func rulesLinked(paths config.Paths) bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return false
	}
	kitRules, err := filepath.EvalSymlinks(filepath.Join(filepath.Dir(real), "..", "rules"))
	if err != nil {
		return false
	}
	ruleFiles, _ := filepath.Glob(filepath.Join(kitRules, "*.md"))
	entries, _ := os.ReadDir(filepath.Join(paths.Home, "rules"))
	for _, entry := range entries {
		dir := filepath.Join(paths.Home, "rules", entry.Name())
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		if physical, err := filepath.EvalSymlinks(dir); err == nil && physical == kitRules {
			return true
		}
		if len(ruleFiles) == 0 {
			continue
		}
		complete := true
		for _, rule := range ruleFiles {
			if _, err := os.Stat(filepath.Join(dir, filepath.Base(rule))); err != nil {
				complete = false
				break
			}
		}
		if complete {
			return true
		}
	}
	return false
}
