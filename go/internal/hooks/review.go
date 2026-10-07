package hooks

import (
	"bytes"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// ReviewChecks makes every /code-review run the full suite (rules/verify.md),
// typed (UserPromptExpansion) or invoked (PostToolUse on Skill), once, and
// adds the result to Claude's context so the review reports it. It never
// blocks: a failing suite is a finding, not a reason to skip the review.
func ReviewChecks(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return nil
	}
	event := p.String("hook_event_name")
	switch {
	case event == "UserPromptExpansion" && p.String("command_name") == "code-review":
	case event == "PostToolUse" && p.String("tool_name") == "Skill" && p.String("tool_input.skill") == "code-review":
	default:
		return nil
	}
	cfg := h.Config()
	if cfg == nil {
		return nil
	}
	root := project.Toplevel(cwdOf(h))
	if root == "" {
		return nil
	}
	var suite bytes.Buffer
	failed := checks.RunAll(cfg, root, nil, &suite)
	verdict := "passed"
	if failed > 0 {
		verdict = "FAILED; each failure is a finding of this review"
	}
	hook.WriteJSON(h.Stdout, map[string]any{"hookSpecificOutput": map[string]string{
		"hookEventName":     event,
		"additionalContext": "kit run-checks ran for this /code-review and " + verdict + "; don't run it again:\n" + strings.TrimSpace(suite.String()),
	}})
	return nil
}
