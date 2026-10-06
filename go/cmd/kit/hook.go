package main

import (
	"fmt"
	"io"

	"github.com/ku5ic/claude-kit/go/internal/bashguard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/hooks"
)

// singleChecks are hooks that run one check; dispatchers get their own case.
var singleChecks = map[string]hook.Check{
	"plan-mode-context":       hooks.PlanModeContext,
	"guard-edit":              hooks.GuardEdit,
	"guard-skills":            hooks.GuardSkills,
	"guard-commit":            hooks.GuardCommit,
	"log-skills":              hooks.LogSkills,
	"sanitize-output":         hooks.SanitizeOutput,
	"inject-context":          hooks.InjectContext,
	"inject-subagent-context": hooks.InjectSubagentContext,
	"inject-rules":            hooks.InjectRules,
	"format-dispatch":         hooks.FormatDispatch,
	"stop-checks":             hooks.StopChecks,
	"guard-bash":              bashguard.Check,
}

// cmdHook runs `kit hook <name>`: stdin is the payload, the exit status is
// Claude Code's (0 allow, 2 block). Anything unexpected fails open.
func cmdHook(e *env, args []string, stdin io.Reader) (status int) {
	if len(args) == 0 {
		fmt.Fprintln(e.stderr, "kit hook: missing hook name")
		return 0
	}
	name := args[0]
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(e.stderr, "%s: unexpected error, failing open\n", name)
			status = 0
		}
	}()

	raw, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(e.stderr, "%s: unexpected error, failing open\n", name)
		return 0
	}
	h := &hook.Hook{
		Name:    name,
		Args:    args[1:],
		Payload: hook.ParsePayload(raw),
		Paths:   e.paths,
		Stdout:  e.stdout,
		Stderr:  e.stderr,
	}

	if args[0] == "guard-dispatch" {
		return hooks.GuardDispatch(h)
	}
	check, ok := singleChecks[args[0]]
	if !ok {
		fmt.Fprintf(e.stderr, "kit hook: unknown hook %q\n", args[0])
		return 0
	}
	return hook.Run(h, hook.NamedCheck{Name: name, Check: check})
}
