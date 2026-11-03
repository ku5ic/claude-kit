package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/slop"
)

// chatHit is a tell a reply held, kept for the next prompt's correction.
type chatHit struct {
	Match string `json:"match"`
	Fix   string `json:"fix"`
}

// ChatSlop corrects AI tells in chat replies on the next turn: a Stop runs
// after the reply is on screen, so nothing can stop one. At Stop it logs
// every hit and keeps those in a slop.block_tiers tier; on UserPromptSubmit
// it says them in context the user never sees, and clears them.
func ChatSlop(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return nil
	}
	marker := h.Paths.SessionFile(cache.Slop, p.SessionID(), "chat")
	if marker == "" {
		return nil
	}
	var kept []chatHit
	if data, err := os.ReadFile(marker); err == nil {
		_ = json.Unmarshal(data, &kept)
	}
	switch p.String("hook_event_name") {
	case "UserPromptSubmit":
		if len(kept) > 0 {
			_ = os.Remove(marker)
			h.AddContext("UserPromptSubmit", chatCorrection(kept))
		}
	case "Stop":
		cfg := h.Config()
		if cfg == nil {
			return nil
		}
		_, blocking := slop.New(cfg).Check(slop.Prose(p.String("last_assistant_message")), "chat", logHit(h, "chat reply"))
		if len(blocking) == 0 || h.DryRun {
			return nil
		}
		for _, hit := range blocking {
			if c := (chatHit{hit.Match, hit.Fix}); !slices.Contains(kept, c) {
				kept = append(kept, c)
			}
		}
		if data, err := json.Marshal(kept); err == nil {
			_ = fsx.WriteAtomic(marker, data, 0o644)
		}
	}
	return nil
}

// chatCorrection is the context naming each tell the last reply held.
func chatCorrection(hits []chatHit) string {
	tells := make([]string, len(hits))
	for i, c := range hits {
		tells[i] = fmt.Sprintf("%q -> %s", c.Match, c.Fix)
	}
	return "Your last reply held AI tells: " + strings.Join(tells, "; ") +
		". Don't repeat them: write each sentence around the fact it points at, or cut it; a synonym keeps the tell (slop-patterns skill)."
}
