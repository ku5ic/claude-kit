package hooks

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

var (
	// doneStep matches a ticked checklist item.
	doneStep    = regexp.MustCompile(`(?m)^\s*[-*] \[[xX]\]`)
	typedReview = regexp.MustCompile(`^\s*/code-review(\s|$)|<command-name>/code-review</command-name>`)
)

// planDone reads the Stop hook's end-of-plan moment (rules/verify.md) from
// the transcript: plan is the plan in plansDir whose last open step this
// turn ticked, else ""; reviewed is whether /code-review ran after the last
// code edit, anywhere in the transcript. isCode is false for paths whose
// edits aren't code (scratch, outside the repo).
func planDone(path, plansDir string, isCode func(string) bool) (plan string, reviewed bool) {
	// Transcript line numbers; -1 is never.
	lastEdit, lastReview := -1, -1
	var ticked []string // plans this turn edited
	// A read error leaves what was read: the hook fails open.
	_ = transcript.Each(path, func(e transcript.Entry) {
		// A typed /code-review arrives as the user's own message:
		// "/code-review high", or a <command-name> block.
		if e.Type == "user" && typedReview.MatchString(e.Text) {
			lastReview = e.Line
		}
		if e.StartsTurn() {
			ticked = ticked[:0]
			return
		}
		if e.Type != "assistant" {
			return
		}
		for _, b := range e.ToolUses() {
			switch {
			case b.Name == "Skill" && b.Input.Skill == "code-review":
				lastReview = e.Line
			case slices.Contains(transcript.EditTools, b.Name):
				if p := b.Path(); strings.HasPrefix(p, plansDir+string(filepath.Separator)) {
					ticked = append(ticked, p)
				} else if isCode(p) {
					lastEdit = e.Line
				}
			}
		}
	})

	for _, path := range ticked {
		if data, err := os.ReadFile(path); err == nil && !openStep.Match(data) && doneStep.Match(data) {
			plan = path
		}
	}
	return plan, lastReview > lastEdit
}
