package stackctx

import (
	"slices"
	"testing"

	"github.com/ku5ic/dotfiles/claude-kit/go/internal/config"
)

// The stack-line parser reads extras from the current report format, and
// skips root and versions lines.
func TestSignals(t *testing.T) {
	report := "root: /x\njs: yes (react) [pnpm] at ., packages/a\npython: yes [uv] at services/api\nversions [packages/a]: react 19.0.0 (declared)\n"
	got := Signals(report)
	if want := []string{"js", "js+react", "python"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := Signals("js: yes (typescript, react)\n"); !slices.Equal(got, []string{"js", "js+typescript", "js+react"}) {
		t.Errorf("comma-space extras: got %q", got)
	}
}

func TestSuggestedSkipsGlobalAndDedupes(t *testing.T) {
	cfg := &config.Config{
		GlobalSkills: []string{"fix-sizing"},
		Stacks: map[string]config.Stack{
			"js": {Skills: []string{"javascript-patterns", "fix-sizing"}, Extras: []config.Extra{
				{Name: "react", Skills: []string{"react-patterns", "javascript-patterns"}},
			}},
		},
	}
	got := Suggested(cfg, []string{"js", "js+react", "js+vue"})
	if want := []string{"javascript-patterns", "react-patterns"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if RequiredBlock(Required(cfg)) != "\n<required-skills>\nBLOCKING REQUIREMENT: invoke the Skill tool for each of these skills NOW, before any other action: fix-sizing\n</required-skills>\n" {
		t.Error("required block shape changed")
	}
}
