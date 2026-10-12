package main

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

// The registry and hooks/hooks.json describe the same hooks: every command
// hooks.json runs is a registered hook under an event the registry names,
// every event the registry names has hooks.json run it, and every step is a
// check.
func TestRegistryMatchesHooksJSON(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../../hooks/hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Hooks map[string][]struct {
			Hooks []struct{ Command string }
		}
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	wired := map[string][]string{} // hook name -> events hooks.json runs it on
	for event, groups := range file.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				_, rest, ok := strings.Cut(h.Command, " hook ")
				if !ok {
					t.Errorf("%s: %q isn't a kit hook", event, h.Command)
					continue
				}
				name := strings.Fields(rest)[0]
				def, known := registry[name]
				switch {
				case !known:
					t.Errorf("%s runs %q, which isn't registered", event, name)
				case !slices.Contains(def.events, event):
					t.Errorf("%s runs %q, whose events are %v", event, name, def.events)
				}
				wired[name] = append(wired[name], event)
			}
		}
	}
	for name, def := range registry {
		for _, event := range def.events {
			if !slices.Contains(wired[name], event) {
				t.Errorf("%s lists %s, but hooks.json doesn't run it there", name, event)
			}
		}
		steps := def.steps
		if steps == nil && def.run == nil {
			steps = []string{name}
		}
		for _, step := range steps {
			if _, ok := hookChecks[step]; !ok {
				t.Errorf("%s: step %q is no check", name, step)
			}
		}
	}
}
