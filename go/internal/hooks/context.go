package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/run"
	"github.com/ku5ic/claude-kit/go/internal/stackctx"
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
		now := time.Now()
		cache.Prune(h.Paths.CacheDir(), now)
		if top := git.Toplevel(h.Payload.Cwd()); top != "" {
			run.PruneRestorePoints(top, now)
		}
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
	if !h.DryRun {
		classifyInBackground(root)
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
	// Logged as surfaced, not loaded: the log shows whether a suggestion was
	// ever acted on, and guard-skills never counts one as a load.
	out.WriteString(stackctx.SuggestedBlock(cfg, ctx.Suggested))
	for _, skill := range ctx.Suggested {
		h.Log(kitlog.Skills, kitlog.EventSuggestedSkill, "cwd", h.Payload.String("cwd"), "skill_file", skill)
	}
	out.WriteString(tooling(cfg, root, h.Paths.CacheDir()))
}

// agentContext is the subagent counterpart: the resolved scratch path (which
// overrides the harness's /tmp scratchpad line), then the same repo context
// and skill blocks, unlogged.
func agentContext(paths config.Paths, cfg *config.Config, cwd, session string) string {
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
	context := strings.TrimRight(agentContext(h.Paths, cfg, h.Payload.Cwd(), h.Payload.SessionID()), "\n")
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

// guidance follows the gates when gap-fill has verified a package manager.
const guidance = "Run each directory's scripts through the package manager <repo-context> names for it, prefer these commands and kit run-checks over direct tool invocation, and never substitute a different package manager."

// tooling is the <tooling> block: what kit run-checks runs, planned from
// the gap-fill cache (a session start never waits on the classifier), then
// kit.yml's tools split by whether PATH has them.
func tooling(cfg *config.Config, root, cacheDir string) string {
	p := enforce.Build(cfg, enforce.Options{Root: root, CacheDir: cacheDir})
	return toolingBlock(p, len(gapfill.Managers(cfg, root, cacheDir)) > 0, cliLines(cfg))
}

// toolingBlock is p's gates, its unclassified entries or discovery
// instruction, and cli, with guidance when managed; "" when there's none
// of it.
func toolingBlock(p enforce.Plan, managed bool, cli []string) string {
	body := gateLines(p)
	if p.Unclassified > 0 {
		body = append(body, fmt.Sprintf("unclassified: %d (config entries gap-fill hasn't answered; it runs in the background, and Stop skips them until it answers)", p.Unclassified))
	}
	if len(body) > 0 {
		body = append(body, "`kit run-checks --plan` lists them, with where each comes from, without running them")
	}
	if p.Discovery != "" {
		body = append(body, "discovery: "+p.Discovery)
	}
	body = append(body, cli...)
	if len(body) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("\n<tooling>\n")
	for _, line := range body {
		out.WriteString(line + "\n")
	}
	if managed {
		out.WriteString("\nguidance: " + guidance + "\n")
	}
	out.WriteString("</tooling>\n")
	return out.String()
}

// gateLines are the commands p runs under a header per directory, the
// root's first, past 20 subprojects capped with a note. A body of several
// lines shows as its gate's label.
func gateLines(p enforce.Plan) []string {
	dirs := []string{"."}
	byDir := map[string][]string{}
	for _, g := range p.Gates {
		if g.Skip != "" {
			continue
		}
		dir := fsx.Rel(p.Root, g.Dir)
		if !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
		cmd := strings.TrimSpace(g.Body)
		if strings.Contains(cmd, "\n") {
			cmd = g.Label
		}
		byDir[dir] = append(byDir[dir], "  "+cmd)
	}
	var out []string
	shown := 0
	for _, dir := range dirs {
		if len(byDir[dir]) == 0 {
			continue
		}
		if dir != "." {
			if shown == 20 {
				return append(out, "(subprojects capped at 20; kit run-checks covers all)")
			}
			shown++
		}
		out = append(out, "run-checks runs"+project.SubLabel(dir)+":")
		out = append(out, byDir[dir]...)
	}
	return out
}

// classifyInBackground starts `kit enforce classify` at root, detached, so
// gap-fill answers what its cache lacks before the next checkpoint reads
// it; the run ends at once when the cache already has it all.
func classifyInBackground(root string) {
	self, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(self, "enforce", "classify")
	cmd.Dir = root
	// Its own session: the hook's timeout kills the hook's group, not this.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if cmd.Start() == nil {
		_ = cmd.Process.Release()
	}
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
