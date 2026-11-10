package hooks

import (
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/design"
	"github.com/ku5ic/claude-kit/go/internal/hook"
)

// GuardTokens blocks an edit of a UI file that uses a design token the
// project doesn't define: a var() with no fallback, or a Tailwind color
// shade outside the palette and the project's theme.
// PreToolUse for Edit, Write, and MultiEdit, inside guard-dispatch.
func GuardTokens(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	path := h.Payload.FilePath()
	if path == "" || h.RuleDisabled("ui-undefined-token") {
		return nil
	}
	before, after, ok := written(h.Payload, path)
	if !ok {
		return nil
	}
	undefined := design.UndefinedTokens(path, before, after)
	if len(undefined) == 0 {
		return nil
	}
	return h.Block("this edit uses "+strings.Join(undefined, ", ")+", which the project never defines. Use a token the project's CSS or @theme sets, or define it there first.", "ui-undefined-token")
}
