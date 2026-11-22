package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/md"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

var (
	// openStep matches an unticked checklist item, the shape skills/kit-rules/reference/workflow.md
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
	if p.String(hook.KeyPermissionMode) == "plan" {
		h.Print("In this repo, a plan starts from the investigate skill's findings, and its gatherer questions go to kit:gatherer.\n")
		return nil
	}
	marker := planFile(h, cache.PlanActive, p.Cwd())
	if marker == "" {
		return nil
	}

	if p.String(hook.KeyHookEventName) == "PostToolUse" {
		if p.String(hook.KeyToolName) != "ExitPlanMode" {
			return nil
		}
		pause := pauseOf(h)
		approved := " is approved under step_pause: " + pause + ". " + approvedPause[pause] + pauseSource
		msg := "The plan" + approved
		if plan := newestOpenPlan(h); plan != "" {
			// Best effort: without the marker, later prompts just aren't reminded.
			_ = fsx.WriteAtomic(marker, []byte(plan), 0o644)
			msg = "Plan " + plan + approved
			// A batched pause's first review sheet starts from the tree as approved.
			_ = os.Remove(planFile(h, cache.PlanTicks, p.Cwd()))
			if data, err := os.ReadFile(plan); err == nil {
				if root := git.Toplevel(p.Cwd()); root != "" {
					markTicks(h, root, data)
				}
			}
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
	pause := pauseOf(h)
	h.Print(fmt.Sprintf("Plan %s has open steps. Under step_pause: %s, %s%s\n", plan, pause, openPause[pause], pauseSource))
	return nil
}

// approvedPause and openPause say what each step_pause value means for a
// turn, on approval and on each later prompt; pauseSource names their owner.
var (
	approvedPause = map[string]string{
		"step": "Its first unchecked step is this turn's whole scope, and review follows it",
		"plan": "Its unchecked steps run in order, and one review follows the last",
		"off":  "Its unchecked steps run in order, with no review stop",
	}
	openPause = map[string]string{
		"step": "a turn covers the next unchecked step, then review",
		"plan": "a turn covers the next unchecked step, and review waits for the last one",
		"off":  "a turn covers the next unchecked step, with no review stop",
	}
)

const pauseSource = " (step_pause; kit config names the file that set it; skills/kit-rules/reference/workflow.md section 3)."

// planFile is the session's kind file (plan-active, plan-ticks) for the work
// tree holding dir, keyed by its root too, so one session can run a plan in
// each worktree; by the session alone outside a work tree.
func planFile(h *hook.Hook, kind, dir string) string {
	if root := git.Toplevel(dir); root != "" {
		return h.Paths.SessionFile(kind, h.Payload.SessionID(), cache.RootKey(root))
	}
	return h.Paths.SessionFile(kind, h.Payload.SessionID())
}

// pauseOf is the configured step_pause, step without a config.
func pauseOf(h *hook.Hook) string {
	if cfg := h.Config(); cfg != nil {
		return cfg.Pause()
	}
	return "step"
}

// planCritic is the agent GuardPlan wants run on a plan before approval.
const planCritic = "kit:plan-critic"

// criticAsk opens GuardPlan's deny, which verify-claims looks for so a plan
// doesn't wait on two agents at once.
const criticAsk = "the " + planCritic + " agent hasn't reviewed"

// GuardPlan sends ExitPlanMode back once when no plan-critic ran after the
// plan was written this session, so a plan whose sections contradict each
// other or the repo isn't approved unchecked. The same plan, resent, goes
// through. PreToolUse for ExitPlanMode.
func GuardPlan(h *hook.Hook) error {
	p := h.Payload
	plan := p.String(hook.KeyInputPlanFilePath)
	if p.Err != nil || p.String(hook.KeyToolName) != "ExitPlanMode" || plan == "" || h.RuleDisabled("plan-critic") || planCritiqued(p) {
		return nil
	}
	if line, reason, ok := criticSkip(p.String(hook.KeyInputPlan)); ok {
		h.Log(kitlog.Guards, "critic-skip", "rule", "plan-critic", "path", plan, "line", strconv.Itoa(line), "reason", reason)
		return nil
	}
	marker := h.Paths.SessionFile(cache.PlanCritic, p.SessionID(), filepath.Base(plan))
	if marker == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(p.String(hook.KeyInputPlan)))
	sent := hex.EncodeToString(sum[:])
	if prev, err := os.ReadFile(marker); err == nil && string(prev) == sent {
		return nil
	}
	if !markOnce(h, marker, []byte(sent), "plan-critic") {
		return nil
	}
	h.Deny(fmt.Sprintf("%s %s\n", criticAsk, plan)+
		layout.KV("do", "run it before asking for approval")+"\n"+
		layout.KV("then", "raise each contradiction it finds, between the plan's sections or against the repo, as a question for the user; the same plan passes when ExitPlanMode is called again"), "plan-critic")
	return nil
}

// criticSkipMarker is a prose line that is only an HTML comment skipping the
// critic with a reason, the form an `unslop: allow` marker takes. A mention
// in a code span or inside a sentence doesn't fill the line, so it doesn't
// match.
var criticSkipMarker = regexp.MustCompile(`^[ \t]*<!--[ \t]*critic: skip[ \t]+(\S.*?)[ \t]*-->[ \t]*$`)

// criticSkip is the first critic-skip marker in the plan's prose, by its
// line number and reason; ok is false when there is none.
func criticSkip(plan string) (line int, reason string, ok bool) {
	n := 0
	for l := range md.Lines(plan) {
		n++
		if l.Kind != md.Prose {
			continue
		}
		if m := criticSkipMarker.FindStringSubmatch(strings.TrimRight(l.Text, "\r\n")); m != nil {
			return n, m[1], true
		}
	}
	return 0, "", false
}

// planCritiqued reports whether the ExitPlanMode in p follows a plan-critic
// run on its plan. A transcript that can't be read shows no run, so the plan
// is sent back once like any uncritiqued one.
func planCritiqued(p *hook.Payload) bool {
	entries, err := transcript.Load(p.String(hook.KeyTranscriptPath))
	return err == nil && critiqued(entries, p.String(hook.KeyInputPlanFilePath))
}

// critiqued reports whether a plan-critic call follows the first write to
// plan. Revising the plan from its critique is the expected next step, so
// later writes don't undo it. A run that ended in an error, in its result
// or its background notice, didn't critique it; one still running counts.
func critiqued(entries []transcript.Entry, plan string) bool {
	for first, b := range transcript.Blocks(entries) {
		if b.Type == "tool_use" && slices.Contains(transcript.EditTools, b.Name) && b.Path() == plan {
			return slices.ContainsFunc(transcript.AgentRuns(entries, planCritic), func(run transcript.Run) bool {
				return run.At.After(first) && run.Status != transcript.RunFailed
			})
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
