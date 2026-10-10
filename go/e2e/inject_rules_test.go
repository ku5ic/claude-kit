package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Claude Code caps one hook's context at 10,000 characters and shows only a
// 2,000-character preview past it, so each part must stay under the cap.
const hookContextCap = 10000

// registeredRuleParts counts hooks.json's inject-rules entries for event.
func registeredRuleParts(t *testing.T, event string) int {
	t.Helper()
	var cfg struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(Read(t, filepath.Join(kitRoot, "hooks/hooks.json"))), &cfg); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, group := range cfg.Hooks[event] {
		for _, h := range group.Hooks {
			if strings.Contains(h.Command, "hook inject-rules ") {
				n++
			}
		}
	}
	return n
}

func TestInjectRules(t *testing.T) {
	t.Parallel()
	rule := func(k *Kit, event string, part int) Result {
		return k.Run(`{"hook_event_name":"`+event+`"}`, "hook", "inject-rules", strconv.Itoa(part))
	}

	for _, event := range []string{"SessionStart", "SubagentStart"} {
		t.Run(event+": the registered parts carry every rule file, each part under the cap", func(t *testing.T) {
			t.Parallel()
			k := New(t)
			registered := registeredRuleParts(t, event)
			if registered == 0 {
				t.Fatalf("hooks.json registers no inject-rules entries for %s", event)
			}
			var all strings.Builder
			for part := 1; part <= registered; part++ {
				r := rule(k, event, part)
				r.Want(t, 0)
				text := r.Stdout
				if event == "SubagentStart" && text != "" {
					var out struct {
						HookSpecificOutput struct {
							HookEventName     string `json:"hookEventName"`
							AdditionalContext string `json:"additionalContext"`
						} `json:"hookSpecificOutput"`
					}
					if err := json.Unmarshal([]byte(text), &out); err != nil || out.HookSpecificOutput.HookEventName != "SubagentStart" {
						t.Fatalf("part %d is not SubagentStart JSON: %v\n%s", part, err, text)
					}
					text = out.HookSpecificOutput.AdditionalContext
				}
				if len(text) >= hookContextCap {
					t.Errorf("part %d is %d characters, over the hook context cap", part, len(text))
				}
				all.WriteString(text)
			}
			files, _ := filepath.Glob(filepath.Join(kitRoot, "rules", "*.md"))
			for _, file := range files {
				first, _, _ := strings.Cut(Read(t, file), "\n")
				if !strings.Contains(all.String(), first) {
					t.Errorf("no registered part carries %s (%q): register more inject-rules entries in hooks.json", filepath.Base(file), first)
				}
			}
		})
	}

	t.Run("a part past the last prints nothing", func(t *testing.T) {
		rule(New(t), "SessionStart", 99).Empty(t)
	})

	t.Run("a missing or bad part number fails open", func(t *testing.T) {
		t.Parallel()
		k := New(t)
		k.Hook("inject-rules", map[string]any{"hook_event_name": "SessionStart"}).Want(t, 0)
		r := k.Run(`{"hook_event_name":"SessionStart"}`, "hook", "inject-rules", "x")
		r.Want(t, 0)
		if r.Stdout != "" {
			t.Errorf("stdout = %q", r.Stdout)
		}
	})

	t.Run("a plugin with no rules dir prints nothing", func(t *testing.T) {
		t.Parallel()
		k := NewPlugin(t)
		Touch(t, filepath.Join(k.Claude, "kit.yml"))
		if _, err := os.Stat(filepath.Join(k.Root, "rules")); err == nil {
			t.Fatal("sandbox unexpectedly has rules")
		}
		rule(k, "SessionStart", 1).Empty(t)
	})
}
