package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// PlanChanges shows the user what a resubmitted plan changed, from the
// hook's own diff against the text it last saw, so the list isn't Claude's
// account of it: each section added, removed, or changed, with line
// counts, then the full diff in a scratch file. A first submit and an
// unchanged resubmit print nothing. PreToolUse for ExitPlanMode.
func PlanChanges(h *hook.Hook) error {
	p := h.Payload
	plan, text := p.String("tool_input.planFilePath"), p.String("tool_input.plan")
	if p.Err != nil || plan == "" || text == "" {
		return nil
	}
	file := h.Paths.SessionFile(cache.PlanCritic, p.SessionID(), filepath.Base(plan), "text")
	if file == "" {
		return nil
	}
	prev, err := os.ReadFile(file)
	if !h.DryRun {
		_ = fsx.WriteAtomic(file, []byte(text), 0o600)
	}
	if err != nil || string(prev) == text {
		return nil
	}
	changes := sectionChanges(string(prev), text)
	if len(changes) == 0 {
		return nil
	}
	lines := []string{fmt.Sprintf("kit plan: resubmitted, %d sections changed", len(changes))}
	if len(changes) > sheetLimit-1 {
		changes = append(changes[:sheetLimit-2], layout.KV("more", fmt.Sprintf("%d sections, in the diff", len(changes)-(sheetLimit-2))))
	}
	lines = append(lines, changes...)
	if diff := planDiff(h, string(prev), text); diff != "" {
		lines = append(lines, layout.KV("diff", diff))
	}
	h.Notify(strings.Join(lines, "\n"))
	return nil
}

// planSection is one heading's lines, the text before the first heading
// under "(top)".
type planSection struct {
	heading string
	lines   []string
}

func planSections(text string) []planSection {
	sections := []planSection{{heading: "(top)"}}
	for line := range strings.Lines(text) {
		line = strings.TrimRight(line, "\n")
		if strings.HasPrefix(line, "#") {
			sections = append(sections, planSection{heading: line})
			continue
		}
		// Blank lines move with every edit and say nothing about one.
		if strings.TrimSpace(line) != "" {
			last := &sections[len(sections)-1]
			last.lines = append(last.lines, line)
		}
	}
	return sections
}

// sectionChanges is one line per section that differs between old and
// new, in new's order with removed sections last: what happened to it and
// its lines added and removed.
func sectionChanges(old, new string) []string {
	before := map[string][]string{}
	for _, s := range planSections(old) {
		before[s.heading] = append(before[s.heading], s.lines...)
	}
	var out, seen []string
	for _, s := range planSections(new) {
		seen = append(seen, s.heading)
		prev, had := before[s.heading]
		add, del := lineDelta(prev, s.lines)
		switch {
		case !had && (s.heading != "(top)" || add > 0):
			out = append(out, layout.KV("added", fmt.Sprintf("%s  +%d", s.heading, add+1)))
		case add+del > 0:
			out = append(out, layout.KV("changed", fmt.Sprintf("%s  +%d -%d", s.heading, add, del)))
		}
	}
	for _, s := range planSections(old) {
		if !slices.Contains(seen, s.heading) && (s.heading != "(top)" || len(s.lines) > 0) {
			out = append(out, layout.KV("removed", fmt.Sprintf("%s  -%d", s.heading, len(s.lines)+1)))
			seen = append(seen, s.heading)
		}
	}
	return out
}

// lineDelta counts the lines new has that old doesn't, and the reverse,
// each line matched once.
func lineDelta(old, new []string) (add, del int) {
	count := map[string]int{}
	for _, l := range old {
		count[l]++
	}
	for _, l := range new {
		if count[l] > 0 {
			count[l]--
			continue
		}
		add++
	}
	for _, n := range count {
		del += n
	}
	return add, del
}

// planDiff writes the full diff from old to new to scratch and returns its
// path, "" when it can't.
func planDiff(h *hook.Hook, old, new string) string {
	dir, err := project.ScratchDir(h.Paths, h.Payload.Cwd(), !h.DryRun)
	if err != nil || h.DryRun {
		return ""
	}
	before, after := filepath.Join(dir, "kit-plan-before.md"), filepath.Join(dir, "kit-plan-after.md")
	if fsx.WriteAtomic(before, []byte(old), 0o600) != nil || fsx.WriteAtomic(after, []byte(new), 0o600) != nil {
		return ""
	}
	// git diff --no-index exits 1 when the files differ; its output is the diff.
	diff, _ := git.Output("", "diff", "--no-index", "--no-color", before, after)
	file := filepath.Join(dir, "kit-plan-changes.diff")
	if diff == "" || fsx.WriteAtomic(file, []byte(diff), 0o600) != nil {
		return ""
	}
	return file
}
