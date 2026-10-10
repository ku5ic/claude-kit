package hooks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

var (
	// openStep matches an unticked checklist item, the shape rules/workflow.md
	// section 3 requires a plan's Steps list to take.
	openStep = regexp.MustCompile(`(?m)^\s*[-*] \[ \]`)
	// stepsHeading opens that list; the next heading at its level or above ends it.
	stepsHeading = regexp.MustCompile(`(?m)^##[ \t]+Steps[ \t]*$`)
	nextHeading  = regexp.MustCompile(`(?m)^#{1,2}[ \t]`)
)

// steps is the part of a plan its steps are read from: the "## Steps"
// section, else the whole plan, minus fenced code, whose checkboxes are
// examples rather than steps.
func steps(plan []byte) []byte {
	if loc := stepsHeading.FindIndex(plan); loc != nil {
		plan = plan[loc[1]:]
		if next := nextHeading.FindIndex(plan); next != nil {
			plan = plan[:next[0]]
		}
	}
	var kept []byte
	fenced := false
	for line := range bytes.Lines(plan) {
		if trimmed := bytes.TrimSpace(line); bytes.HasPrefix(trimmed, []byte("```")) || bytes.HasPrefix(trimmed, []byte("~~~")) {
			fenced = !fenced
			continue
		}
		if !fenced {
			kept = append(kept, line...)
		}
	}
	return kept
}

// PlanModeContext keeps plan work to one step per turn, which rule text
// alone did not hold against an "execute autonomously" output style.
//
//   - UserPromptSubmit in plan mode: point Claude at investigate.
//   - PostToolUse on ExitPlanMode: remember the session's plan, ask for
//     step 1 only. PostToolUse takes context as JSON, not plain stdout.
//   - UserPromptSubmit otherwise: while that plan has an open step, remind
//     Claude to do only the next one.
//
// Silent on anything else, a bad payload included.
func PlanModeContext(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return nil
	}
	if p.String("permission_mode") == "plan" {
		h.Print("Plan mode: load the investigate skill and follow it; its findings feed the plan.\n")
		return nil
	}
	marker := h.Paths.SessionFile(cache.PlanActive, p.SessionID())
	if marker == "" {
		return nil
	}

	if p.String("hook_event_name") == "PostToolUse" {
		if p.String("tool_name") != "ExitPlanMode" {
			return nil
		}
		const pause = "implement only its first unchecked step, then stop for review, per rules/workflow.md section 3."
		msg := "Plan approved: " + pause
		if plan := newestOpenPlan(h); plan != "" {
			// Best effort: without the marker, later prompts just aren't reminded.
			_ = os.MkdirAll(filepath.Dir(marker), 0o755)
			_ = os.WriteFile(marker, []byte(plan), 0o644)
			msg = "Plan approved (" + plan + "): " + pause
		}
		h.AddContext("PostToolUse", msg)
		return nil
	}

	plan, err := os.ReadFile(marker)
	if err != nil {
		return nil
	}
	if !hasOpenStep(string(plan)) {
		os.Remove(marker)
		return nil
	}
	// scratch-rotate prunes markers by age: a plan still in use stays fresh.
	now := time.Now()
	_ = os.Chtimes(marker, now, now)
	h.Print(fmt.Sprintf("Active plan %s has open steps: do only the next unchecked one, then stop for review, per rules/workflow.md section 3.\n", plan))
	return nil
}

// newestOpenPlan is the most recently modified plan file in the project's
// plans dir that still has an open step. Picking by open step skips
// plan-critic reports, which share the directory.
func newestOpenPlan(h *hook.Hook) string {
	cfg := h.Config()
	if cfg == nil {
		return ""
	}
	dir, err := project.Dir(cfg, h.Paths, h.Payload.Cwd(), "plans", false)
	if err != nil {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestMod int64
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().UnixNano() <= bestMod {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if hasOpenStep(path) {
			best, bestMod = path, info.ModTime().UnixNano()
		}
	}
	return best
}

func hasOpenStep(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && openStep.Match(steps(data))
}
