package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ku5ic/claude-kit/go/internal/bashguard"
	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claims"
	"github.com/ku5ic/claude-kit/go/internal/claudecli"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/hooks"
)

// hookDef is one `kit hook` name: the hooks.json events that run it (none
// for a check run only inside a dispatcher, or by hand), and the checks it
// runs in order against one payload read, or its own run for a dispatcher
// with a rule of its own.
type hookDef struct {
	events []string
	steps  []string
	run    func(*hook.Hook) int
}

// hookChecks are the single checks, by the name they report under.
var hookChecks = map[string]hook.Check{
	"plan-mode-context":       hooks.PlanModeContext,
	"guard-plan":              hooks.GuardPlan,
	"rules-gate":              hooks.RulesGate,
	"guard-edit":              hooks.GuardEdit,
	"guard-skills":            hooks.GuardSkills,
	"skills-nudge":            skillsNudge,
	"guard-slop":              hooks.GuardSlop,
	"guard-weaken":            hooks.GuardWeaken,
	"guard-imports":           hooks.GuardImports,
	"guard-tokens":            hooks.GuardTokens,
	"guard-handback":          hooks.GuardHandback,
	"guard-connector":         hooks.GuardConnector,
	"verify-claims":           hooks.VerifyClaims,
	"check-citations":         hooks.CheckCitations,
	"check-size":              hooks.CheckSize,
	"subagent-slop":           hooks.SubagentSlop,
	"chat-slop":               hooks.ChatSlop,
	"guard-commit":            bashguard.CheckCommit,
	"guard-pr":                bashguard.CheckPR,
	"log-skills":              hooks.LogSkills,
	"sanitize-output":         hooks.SanitizeOutput,
	"inject-context":          hooks.InjectContext,
	"inject-subagent-context": hooks.InjectSubagentContext,
	"format-dispatch":         hooks.FormatDispatch,
	"plan-tick":               hooks.PlanTick,
	"plan-changes":            hooks.PlanChanges,
	"stop-checks":             hooks.StopChecks,
	"turn-tree":               hooks.TurnTree,
	"bash-edits":              hooks.BashEdits,
	"edit-seen":               hooks.MarkEditSeen,
	"review-checks":           hooks.ReviewChecks,
	"reply-length":            hooks.ReplyLength,
	"guard-bash":              bashguard.Check,
}

// registry is every `kit hook` name. A test holds it and hooks/hooks.json
// to each other.
var registry = map[string]hookDef{
	"bash-dispatch":      {events: []string{"PreToolUse"}, steps: []string{"guard-bash", "guard-commit", "guard-pr", "skills-nudge"}},
	"guard-dispatch":     {events: []string{"PreToolUse"}, run: hooks.GuardDispatch},
	"gate-dispatch":      {events: []string{"PreToolUse"}, steps: []string{"rules-gate"}},
	"rules-gate":         {},
	"post-edit-dispatch": {events: []string{"PostToolUse"}, steps: []string{"sanitize-output", "format-dispatch", "edit-seen", "plan-tick"}},
	"edit-seen":          {},
	"bash-edits":         {events: []string{"PostToolUse"}},
	"log-skills":         {events: []string{"PostToolUse", "UserPromptExpansion"}},
	"plan-mode-context":  {events: []string{"PostToolUse"}},
	"prompt-dispatch":    {events: []string{"UserPromptSubmit"}, steps: []string{"turn-tree", "plan-mode-context", "reply-length", "chat-slop"}},
	// stop-checks runs last: the first block ends a run, and the others only record.
	"stop-dispatch":           {events: []string{"Stop"}, steps: []string{"reply-length", "chat-slop", "stop-checks"}},
	"inject-context":          {events: []string{"SessionStart"}},
	"inject-subagent-context": {events: []string{"SubagentStart"}},
	"review-checks":           {events: []string{"SubagentStop"}},
	"reply-length":            {},
	"chat-slop":               {},
	"stop-checks":             {},
	"turn-tree":               {},
	"guard-plan":              {},
	// verify-claims runs last, so a cheap guard's block saves its model run.
	"handback-dispatch":  {events: []string{"PreToolUse"}, steps: []string{"guard-handback", "check-size", "check-citations", "verify-claims"}},
	"plan-dispatch":      {events: []string{"PreToolUse"}, steps: []string{"plan-changes", "check-size", "check-citations", "guard-plan", "verify-claims"}},
	"connector-dispatch": {events: []string{"PreToolUse"}, steps: []string{"guard-connector", "check-size", "verify-claims"}},
	"subagent-dispatch":  {events: []string{"SubagentStop"}, steps: []string{"subagent-slop", "check-size", "check-citations", "verify-claims"}},
	"check-citations":    {},
	"check-size":         {},
	"guard-handback":     {},
	"guard-connector":    {},
	"subagent-slop":      {},
	"verify-claims":      {},
	"guard-bash":         {},
	"guard-commit":       {},
	"guard-edit":         {},
	"guard-pr":           {},
	"guard-skills":       {},
	"skills-nudge":       {},
	"guard-slop":         {},
	"guard-weaken":       {},
	"guard-imports":      {},
	"guard-tokens":       {},
	"sanitize-output":    {},
	"format-dispatch":    {},
	"plan-tick":          {},
	"plan-changes":       {},
}

// cmdHook runs `kit hook <name>`: stdin is the payload, the exit status is
// Claude Code's (0 allow, 2 block). Anything unexpected fails open.
func cmdHook(e *env, args []string, stdin io.Reader) (status int) {
	if len(args) == 0 {
		fmt.Fprintln(e.stderr, "kit hook: missing hook name")
		return 0
	}
	// The classifier's and the verifier's own claude sessions run no kit
	// hook. Only a value the kit set, with its marker, counts.
	markers := filepath.Join(e.paths.CacheDir(), cache.Guard)
	if claudecli.Honored(markers, gapfill.Guard) || claudecli.Honored(markers, claims.Guard) {
		return 0
	}
	name := args[0]
	def, ok := registry[name]
	if !ok {
		fmt.Fprintf(e.stderr, "kit hook: unknown hook %q\n", name)
		return 0
	}
	defer func() {
		if r := recover(); r != nil {
			hook.FailOpen(e.stderr, name, r)
			status = 0
		}
	}()

	raw, err := io.ReadAll(stdin)
	if err != nil {
		hook.FailOpen(e.stderr, name, err)
		return 0
	}
	h := &hook.Hook{
		Name:    name,
		Args:    args[1:],
		Payload: hook.ParsePayload(raw),
		Paths:   e.paths,
		Stdout:  e.stdout,
		Stderr:  e.stderr,
		Home:    os.Getenv("HOME"),
	}
	if def.run != nil {
		return def.run(h)
	}
	steps := def.steps
	if steps == nil {
		steps = []string{name}
	}
	named := make([]hook.NamedCheck, len(steps))
	for i, s := range steps {
		named[i] = hook.NamedCheck{Name: s, Check: hookChecks[s]}
	}
	return hook.Run(h, named...)
}

// skillsNudge is skills-nudge, a Bash command's reads included. hooks and
// bashguard share a layer, so the two meet here.
func skillsNudge(h *hook.Hook) error {
	if h.Payload.String("tool_name") != "Bash" {
		return hooks.SkillsNudge(h)
	}
	return hooks.NudgeFiles(h, bashguard.ReadTargets(h.Payload.String("tool_input.command"), h.Home, h.Payload.Cwd()))
}
