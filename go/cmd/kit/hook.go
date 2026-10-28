package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ku5ic/claude-kit/go/internal/bashguard"
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
	"guard-edit":              hooks.GuardEdit,
	"guard-skills":            hooks.GuardSkills,
	"guard-commit":            bashguard.CheckCommit,
	"log-skills":              hooks.LogSkills,
	"sanitize-output":         hooks.SanitizeOutput,
	"inject-context":          hooks.InjectContext,
	"inject-subagent-context": hooks.InjectSubagentContext,
	"inject-rules":            hooks.InjectRules,
	"format-dispatch":         hooks.FormatDispatch,
	"stop-checks":             hooks.StopChecks,
	"review-checks":           hooks.ReviewChecks,
	"reply-length":            hooks.ReplyLength,
	"guard-bash":              bashguard.Check,
}

// registry is every `kit hook` name. A test holds it and hooks/hooks.json
// to each other.
var registry = map[string]hookDef{
	"bash-dispatch":           {events: []string{"PreToolUse"}, steps: []string{"guard-bash", "guard-commit"}},
	"guard-dispatch":          {events: []string{"PreToolUse"}, run: hooks.GuardDispatch},
	"post-edit-dispatch":      {events: []string{"PostToolUse"}, steps: []string{"sanitize-output", "format-dispatch"}},
	"log-skills":              {events: []string{"PostToolUse", "UserPromptExpansion"}},
	"plan-mode-context":       {events: []string{"PostToolUse", "UserPromptSubmit"}},
	"reply-length":            {events: []string{"UserPromptSubmit", "Stop"}},
	"inject-context":          {events: []string{"SessionStart"}},
	"inject-rules":            {events: []string{"SessionStart", "SubagentStart"}},
	"inject-subagent-context": {events: []string{"SubagentStart"}},
	"stop-checks":             {events: []string{"Stop"}},
	"review-checks":           {events: []string{"SubagentStop"}},
	"guard-bash":              {},
	"guard-commit":            {},
	"guard-edit":              {},
	"guard-skills":            {},
	"sanitize-output":         {},
	"format-dispatch":         {},
}

// cmdHook runs `kit hook <name>`: stdin is the payload, the exit status is
// Claude Code's (0 allow, 2 block). Anything unexpected fails open.
func cmdHook(e *env, args []string, stdin io.Reader) (status int) {
	if len(args) == 0 {
		fmt.Fprintln(e.stderr, "kit hook: missing hook name")
		return 0
	}
	// The classifier's own claude session runs no kit hook.
	if os.Getenv(gapfill.Guard) == "1" {
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
			hook.FailOpen(e.stderr, name)
			status = 0
		}
	}()

	raw, err := io.ReadAll(stdin)
	if err != nil {
		hook.FailOpen(e.stderr, name)
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
