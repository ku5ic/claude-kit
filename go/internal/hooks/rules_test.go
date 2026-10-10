package hooks

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// hookContextCap is Claude Code's cap on one hook's context: past it,
// Claude sees only a 2,000-character preview.
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
	if err := json.Unmarshal([]byte(testutil.Read(t, filepath.Join(kitRoot, "hooks/hooks.json"))), &cfg); err != nil {
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
	rule := func(k *sandbox, part int) result {
		return k.run("inject-rules", `{"hook_event_name":"SessionStart"}`, strconv.Itoa(part))
	}

	t.Run("SessionStart: the registered parts carry every rule file, each part under the cap", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		registered := registeredRuleParts(t, "SessionStart")
		if registered == 0 {
			t.Fatal("hooks.json registers no inject-rules entries for SessionStart")
		}
		var all strings.Builder
		for part := 1; part <= registered; part++ {
			r := rule(k, part)
			r.Want(t, 0)
			if len(r.Stdout) >= hookContextCap {
				t.Errorf("part %d is %d characters, over the hook context cap", part, len(r.Stdout))
			}
			all.WriteString(r.Stdout)
		}
		files, _ := filepath.Glob(filepath.Join(kitRoot, "rules", "*.md"))
		for _, file := range files {
			first, _, _ := strings.Cut(testutil.Read(t, file), "\n")
			if !strings.Contains(all.String(), first) {
				t.Errorf("no registered part carries %s (%q): register more inject-rules entries in hooks.json", filepath.Base(file), first)
			}
		}
	})
	t.Run("a part past the last prints nothing", func(t *testing.T) {
		t.Parallel()
		rule(newSandbox(t), 99).Empty(t)
	})
	t.Run("a plugin with no rules dir prints nothing", func(t *testing.T) {
		t.Parallel()
		k := newPlugin(t)
		k.kitYML("")
		if exists(filepath.Join(k.paths.Root, "rules")) {
			t.Fatal("sandbox unexpectedly has rules")
		}
		rule(k, 1).Empty(t)
	})
}
