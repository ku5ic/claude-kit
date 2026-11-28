package hooks

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Every kit agent whose tools: line holds neither Edit nor Write only
// reads, so a held review lets the main thread launch it.
func TestReadingAgentsListEveryReadOnlyAgent(t *testing.T) {
	t.Parallel()
	tools := regexp.MustCompile(`(?m)^tools:(.*)$`)
	agents, _ := filepath.Glob(filepath.Join("..", "..", "..", "agents", "*.md"))
	if len(agents) == 0 {
		t.Fatal("no agents found")
	}
	for _, agent := range agents {
		text, err := os.ReadFile(agent)
		if err != nil {
			t.Fatal(err)
		}
		m := tools.FindSubmatch(text)
		if m == nil {
			t.Fatalf("%s has no tools: line", agent)
		}
		names := strings.FieldsFunc(string(m[1]), func(r rune) bool { return r == ',' || r == ' ' })
		readOnly := !slices.Contains(names, "Edit") && !slices.Contains(names, "Write")
		name := "kit:" + strings.TrimSuffix(filepath.Base(agent), ".md")
		if listed := slices.Contains(readingAgents, name); readOnly != listed {
			t.Errorf("%s: read-only %v, in readingAgents %v", name, readOnly, listed)
		}
	}
}
