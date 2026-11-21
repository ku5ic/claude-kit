package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claims"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/slop"
)

// chatHit is a tell a reply held, kept for the next prompt's correction.
type chatHit struct {
	Match string `json:"match"`
	Fix   string `json:"fix"`
}

// ChatSlop corrects AI tells in chat replies on the next turn: a Stop runs
// after the reply is on screen, so nothing can stop one. At Stop it logs
// every hit and keeps those in a slop.block_tiers tier.
func ChatSlop(h *hook.Hook) error {
	nextTurn(h, h.Paths.SessionFile(cache.Slop, h.Payload.SessionID(), "chat"), func() []chatHit {
		cfg := h.Config()
		if cfg == nil {
			return nil
		}
		_, blocking := slop.New(cfg).Check(slop.Prose(h.Payload.String(hook.KeyLastAssistantMessage)), "chat", logHit(h, "chat reply"))
		hits := make([]chatHit, len(blocking))
		for i, hit := range blocking {
			hits[i] = chatHit{hit.Match, hit.Fix}
		}
		return hits
	}, func(c chatHit) string { return fmt.Sprintf("%q -> %s", c.Match, c.Fix) },
		"The last reply held AI tells: ",
		". A synonym keeps the tell. The slop-patterns skill has the rewrite: each sentence written around the fact it points at, or cut.")
	return nil
}

// ChatClaims corrects a chat reply's bare references that don't resolve on
// the next turn, as ChatSlop does its tells, with the no-model pass. A
// reply's other claims rest on Claude's own checking.
func ChatClaims(h *hook.Hook) error {
	nextTurn(h, h.Paths.SessionFile(cache.Claims, h.Payload.SessionID(), "chat"), func() []claims.Unsupported {
		cfg := h.Config()
		if cfg == nil || h.RuleDisabled("claims") || cfg.Claims.Off() {
			return nil
		}
		root, _ := project.Root(h.Payload.Cwd())
		return claims.References(root, h.Payload.String(hook.KeyLastAssistantMessage))
	}, func(c claims.Unsupported) string { return fmt.Sprintf("%q (%s)", c.Claim, c.Check) },
		"The last reply cited what doesn't resolve: ",
		". Open your next reply with the corrected fact, checked with a tool (skills/kit-rules/reference/evidence.md section 1).")
	return nil
}

// nextTurn is a reply's correction on the next turn. At Stop, found's items
// are kept in marker, each once; on UserPromptSubmit they go to context the
// user never sees, each as its line between head and tail, and the marker
// is cleared.
func nextTurn[T comparable](h *hook.Hook, marker string, found func() []T, line func(T) string, head, tail string) {
	if h.Payload.Err != nil || marker == "" {
		return
	}
	var kept []T
	if data, err := os.ReadFile(marker); err == nil {
		_ = json.Unmarshal(data, &kept)
	}
	switch h.Payload.String(hook.KeyHookEventName) {
	case "UserPromptSubmit":
		if len(kept) > 0 {
			_ = os.Remove(marker)
			lines := make([]string, len(kept))
			for i, k := range kept {
				lines[i] = line(k)
			}
			h.AddContext("UserPromptSubmit", head+strings.Join(lines, "; ")+tail)
		}
	case "Stop":
		fresh := found()
		if len(fresh) == 0 || h.DryRun {
			return
		}
		for _, f := range fresh {
			if !slices.Contains(kept, f) {
				kept = append(kept, f)
			}
		}
		if data, err := json.Marshal(kept); err == nil {
			_ = fsx.WriteAtomic(marker, data, 0o600)
		}
	}
}
