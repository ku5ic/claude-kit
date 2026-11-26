package hooks

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// replaces matches a plan's line naming the plan file it replaces.
var replaces = regexp.MustCompile("(?m)^Replaces\\s+`?([^`\\s]+\\.md)")

// heldPlan is a changed plan PlanChanges denied, and the user's last answer
// at the time, by transcript line.
type heldPlan struct {
	Text     string `json:"text"`
	Answered int    `json:"answered"`
}

// PlanChanges holds a changed plan's dialog until the user has answered the
// changes in chat: it denies the submit, listing each section added,
// removed, or changed with line counts, from the hook's own diff against the
// text the user last saw, and the full diff in scratch, so the list isn't
// Claude's account of it. The same text passes once the user answered since,
// as ReviewPick counts it; a later edit is a new change. A first submit is
// diffed against the plan its "Replaces" line names, and passes without one.
// Last in the plan's checks, and only when none of them sent it back: a plan
// no dialog opened for was never seen. PreToolUse for ExitPlanMode.
func PlanChanges(h *hook.Hook) error {
	p := h.Payload
	plan, text := p.String(hook.KeyInputPlanFilePath), p.String(hook.KeyInputPlan)
	if p.Err != nil || plan == "" || text == "" {
		return nil
	}
	if decision, _ := h.Decision(); decision == "deny" {
		return nil
	}
	name := filepath.Base(plan)
	shownFile := h.Paths.SessionFile(cache.PlanCritic, p.SessionID(), name, "text")
	heldFile := h.Paths.SessionFile(cache.PlanCritic, p.SessionID(), name, "held")
	if shownFile == "" {
		return nil
	}
	save := func(file, data string) {
		if !h.DryRun {
			_ = fsx.WriteAtomic(file, []byte(data), 0o600)
		}
	}
	held := readHeld(heldFile)
	answered, ok := lastAnswer(h)
	shown, seen := lastShown(h, shownFile, plan, text)
	if held.Text != "" && answered > held.Answered {
		shown = held.Text
		save(shownFile, shown)
	}
	changes := sectionChanges(shown, text)
	if !seen || !ok || len(changes) == 0 || !h.Deny(planReason(h, changes, shown, text), "plan-changes") {
		save(shownFile, text)
		if !h.DryRun {
			_ = os.Remove(heldFile) // the dialog shows text, so nothing is held; none is fine
		}
		return nil
	}
	if data, err := json.Marshal(heldPlan{text, answered}); err == nil {
		save(heldFile, string(data))
	}
	return nil
}

// readHeld is the held plan in file, zero when there is none.
func readHeld(file string) heldPlan {
	var held heldPlan
	if data, err := os.ReadFile(file); err != nil || json.Unmarshal(data, &held) != nil {
		return heldPlan{}
	}
	return held
}

// lastShown is the plan text the user last saw, from file, or on a first
// submit the plan text's "Replaces" line names, from the repo root or the
// plan's own folder; false when neither is there.
func lastShown(h *hook.Hook, file, plan, text string) (string, bool) {
	if data, err := os.ReadFile(file); err == nil {
		return string(data), true
	}
	m := replaces.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	cwd := h.Payload.Cwd()
	for _, dir := range []string{cmp.Or(git.Toplevel(cwd), cwd), filepath.Dir(fsx.Abs(cwd, plan))} {
		if data, err := os.ReadFile(fsx.Abs(dir, m[1])); err == nil {
			return string(data), true
		}
	}
	return "", false
}

// lastAnswer is the transcript line of the user's last answer, a typed
// prompt or an AskUserQuestion's result; false when the transcript can't be
// read, since no answer could then lift a deny.
func lastAnswer(h *hook.Hook) (int, bool) {
	path, cwd := h.Payload.String(hook.KeyTranscriptPath), h.Payload.Cwd()
	if !fsx.IsFile(path) {
		return 0, false
	}
	root := git.Toplevel(cwd)
	_, walk := readSession(h, path, root, project.PlanDir(h.Paths, cwd), codeIn(h, root))
	return walk.Answered, true
}

// planReason is PlanChanges' deny: what changed, the diff, what to do, and
// last the plan's path, which closes Claude's message too.
func planReason(h *hook.Hook, changes []string, old, new string) string {
	lines := []string{"the plan changed since the user last saw it"}
	if len(changes) > sheetLimit-1 {
		changes = append(changes[:sheetLimit-2], layout.KV("more", fmt.Sprintf("%d sections, in the diff", len(changes)-(sheetLimit-2))))
	}
	lines = append(lines, changes...)
	if diff := planDiff(h, old, new); diff != "" {
		lines = append(lines, layout.KV("diff", diff))
	}
	return strings.Join(append(lines,
		layout.KV("do", "post the changes in chat, in the format of skills/kit-rules/reference/workflow.md section 3, and end your turn; call ExitPlanMode again once the user answers"),
		layout.KV("plan", fsx.Abs(h.Payload.Cwd(), h.Payload.String(hook.KeyInputPlanFilePath)))), "\n")
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
