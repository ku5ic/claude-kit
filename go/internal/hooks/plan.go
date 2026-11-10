package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/md"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
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
	for l := range md.Lines(string(plan)) {
		if l.Kind == md.Prose || l.Kind == md.Front {
			kept = append(kept, l.Text...)
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
			_ = fsx.WriteAtomic(marker, []byte(plan), 0o644)
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

// planCritic is the agent GuardPlan wants run on a plan before approval.
const planCritic = "kit:plan-critic"

// GuardPlan sends ExitPlanMode back once when no plan-critic ran after the
// plan was written this session, so a plan whose sections contradict each
// other or the repo isn't approved unchecked. The same plan, resent, goes
// through. PreToolUse for ExitPlanMode.
func GuardPlan(h *hook.Hook) error {
	p := h.Payload
	plan := p.String("tool_input.planFilePath")
	if p.Err != nil || p.String("tool_name") != "ExitPlanMode" || plan == "" || h.RuleDisabled("plan-critic") {
		return nil
	}
	entries, err := transcript.Load(p.String("transcript_path"))
	if err != nil || critiqued(entries, plan) {
		return nil
	}
	marker := h.Paths.SessionFile(cache.PlanCritic, p.SessionID(), filepath.Base(plan))
	if marker == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(p.String("tool_input.plan")))
	sent := hex.EncodeToString(sum[:])
	if prev, err := os.ReadFile(marker); err == nil && string(prev) == sent {
		return nil
	}
	if !h.DryRun {
		_ = fsx.WriteAtomic(marker, []byte(sent), 0o644)
	}
	h.Deny(fmt.Sprintf("run the %s agent on %s before asking for approval. Raise each contradiction it finds, between the plan's sections or against the repo, as a question for the user. Calling ExitPlanMode again with the same plan goes through.", planCritic, plan), "plan-critic")
	return nil
}

// critiqued reports whether a plan-critic call follows the first write to
// plan. Revising the plan from its critique is the expected next step, so
// later writes don't undo it.
func critiqued(entries []transcript.Entry, plan string) bool {
	written := false
	for _, e := range entries {
		if e.Type != "assistant" {
			continue
		}
		for _, b := range e.ToolUses() {
			switch {
			case slices.Contains(transcript.EditTools, b.Name) && b.Path() == plan:
				written = true
			case b.Name == "Agent" && b.Input.SubagentType == planCritic && written:
				return true
			}
		}
	}
	return false
}

// newestOpenPlan is the most recently modified plan file in the project's
// plans dir that still has an open step. In the default dir, shared by
// every project, that is the one ExitPlanMode just wrote.
func newestOpenPlan(h *hook.Hook) string {
	if h.Config() == nil {
		return ""
	}
	dir := project.PlanDir(h.Paths, h.Payload.Cwd())
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
