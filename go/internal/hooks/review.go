package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// ReviewChecks makes every /code-review of the working tree run the full suite
// (rules/verify.md), typed (UserPromptExpansion) or invoked (PostToolUse on
// Skill), once, and adds the result to Claude's context so the review reports
// it. A review of a PR, URL, or other branch skips the suite and says so. It
// never blocks: a failing suite is a finding, not a reason to skip the review.
func ReviewChecks(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return nil
	}
	event := p.String("hook_event_name")
	var args string
	switch {
	case event == "UserPromptExpansion" && p.String("command_name") == "code-review":
		args = p.String("command_args")
	case event == "PostToolUse" && p.String("tool_name") == "Skill" && p.String("tool_input.skill") == "code-review":
		args = p.String("tool_input.args")
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
	if target := reviewTarget(args); target != "" && !inWorkingTree(root, target) {
		hook.WriteJSON(h.Stdout, map[string]any{"hookSpecificOutput": map[string]string{
			"hookEventName":     event,
			"additionalContext": "kit run-checks did not run: this /code-review targets " + target + ", not the working tree, so local check results don't apply.",
		}})
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

// reviewTarget is the first /code-review argument that is not an effort
// level, a flag, or the value of --max-findings; "" when there is none.
func reviewTarget(args string) string {
	fields := strings.Fields(args)
	for i := 0; i < len(fields); i++ {
		switch f := fields[i]; {
		case f == "--max-findings":
			i++
		case strings.HasPrefix(f, "--"):
		case slices.Contains([]string{"low", "medium", "high", "xhigh", "max"}, f):
		default:
			return f
		}
	}
	return ""
}

// inWorkingTree reports whether a review target is local code: an existing
// path, or the branch checked out at root.
func inWorkingTree(root, target string) bool {
	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, target)
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	branch, err := project.Branch(root)
	return err == nil && branch != "" && branch == target
}
