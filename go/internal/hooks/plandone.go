package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

var (
	// doneStep matches a ticked checklist item.
	doneStep = regexp.MustCompile(`(?m)^\s*[-*] \[[xX]\]`)
	// A typed /code-review forks with this launch record; a Skill call is
	// known by its tool_use id. Either one's task-notification names it.
	launchTag   = regexp.MustCompile(`<forked-skill-launch>(.*?)</forked-skill-launch>`)
	notifyField = regexp.MustCompile(`<(task-id|tool-use-id|status)>([^<]*)</`)
	// fixReview is a typed /code-review that asks for its fixes.
	fixReview = regexp.MustCompile(`(?s)code-review\b.*--fix\b`)
)

// launchedReview is the task id of a /code-review that text records forking, or "".
func launchedReview(text string) string {
	m := launchTag.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	var launch struct {
		AgentID   string `json:"agentId"`
		SkillName string `json:"skillName"`
	}
	if json.Unmarshal([]byte(m[1]), &launch) != nil || launch.SkillName != reviewSkill {
		return ""
	}
	return launch.AgentID
}

// isNotification is whether text is a task-notification, which arrives
// as a user message or a queued command.
func isNotification(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "<task-notification>")
}

// finishedReview is the transcript line where the review a completed
// task-notification reports was started, and its id, or -1. A killed
// review never counts.
func finishedReview(text string, started map[string]int) (int, string) {
	if !isNotification(text) {
		return -1, ""
	}
	fields := map[string]string{}
	for _, m := range notifyField.FindAllStringSubmatch(text, -1) {
		if _, seen := fields[m[1]]; !seen {
			fields[m[1]] = m[2]
		}
	}
	if fields["status"] != "completed" {
		return -1, ""
	}
	for _, id := range []string{fields["tool-use-id"], fields["task-id"]} {
		if line, ok := started[id]; ok && id != "" {
			return line, id
		}
	}
	return -1, ""
}

// planWalk is the plan-done state as it walks a transcript, kept between
// Stop runs so each one reads only the lines since the last. Lines are
// transcript line numbers; -1 is never.
type planWalk struct {
	LastEdit   int            `json:"last_edit"`
	LastReview int            `json:"last_review"`
	Started    map[string]int `json:"started"` // review tool_use or task id -> its start line
	Ticked     []string       `json:"ticked"`  // plans this turn edited, or changed through Bash
	// ReviewArrived is where a finished review's notification landed, and
	// ReviewReported where a review scratch report was last written.
	ReviewArrived  int `json:"review_arrived"`
	ReviewReported int `json:"review_reported"`
	// ReportWrites is a report write's tool_use id -> its line, until its
	// result: a write another guard denied wrote no report.
	ReportWrites map[string]int `json:"report_writes"`
	// CodeEdits is the code edits, by an edit tool or a recorded Bash call,
	// whose result hasn't come: an edit counts from its call, then again
	// from its result, so a review launched beside it in one message
	// doesn't cover it.
	CodeEdits map[string]bool `json:"code_edits"`
	// HoldFrom is where a finished review that didn't ask for its fixes
	// landed, and Answered where the user last answered: a typed prompt,
	// or an AskUserQuestion's result. Asks is the questions with no result
	// yet, Fixes the --fix reviews by id, and FixTurn whether this turn's
	// prompt typed one.
	HoldFrom int             `json:"hold_from"`
	Answered int             `json:"answered"`
	Asks     map[string]bool `json:"asks"`
	Fixes    map[string]bool `json:"fixes"`
	FixTurn  bool            `json:"fix_turn"`
	// bashed is bashEdits, read fresh by each run: a Bash call's record is
	// written before its result, so it's there by the time the walk reads it.
	bashed map[string][]string
	// plans is the plans dir's files, each with the pattern that finds its
	// name in a command, listed at the walk's first Bash call.
	plans  []listedPlan
	listed bool
}

func newPlanWalk() *planWalk {
	return &planWalk{LastEdit: -1, LastReview: -1, Started: map[string]int{}, ReviewArrived: -1, ReviewReported: -1, ReportWrites: map[string]int{}, CodeEdits: map[string]bool{},
		HoldFrom: -1, Answered: -1, Asks: map[string]bool{}, Fixes: map[string]bool{}}
}

// start records review id starting at line, as a --fix one when fix.
func (w *planWalk) start(id string, line int, fix bool) {
	w.Started[id] = line
	if fix {
		w.Fixes[id] = true
	}
}

// step takes in entry e. isCode is false for paths whose edits aren't code
// (scratch, outside the repo).
func (w *planWalk) step(h *hook.Hook, e transcript.Entry, plansDir string, isCode func(string) bool) {
	if id := launchedReview(e.Notice); id != "" {
		w.start(id, e.Line, w.FixTurn)
	}
	// A notification arrives as a user message or a queued command.
	for _, text := range []string{e.Text, e.Notice} {
		if started, id := finishedReview(text, w.Started); started >= 0 {
			w.LastReview, w.ReviewArrived = max(w.LastReview, started), e.Line
			if !w.Fixes[id] {
				w.HoldFrom = e.Line
			}
		}
	}
	if e.StartsTurn() {
		if e.FromUser() && !isNotification(e.Text) {
			w.Answered, w.FixTurn = e.Line, fixReview.MatchString(e.Text)
		}
		w.Ticked = nil
		return
	}
	for _, b := range e.Blocks {
		if b.Type != "tool_result" {
			continue
		}
		if w.Asks[b.ToolUseID] {
			if !b.IsError {
				w.Answered = e.Line
			}
			delete(w.Asks, b.ToolUseID)
		}
		if line, ok := w.ReportWrites[b.ToolUseID]; ok {
			if !b.IsError {
				w.ReviewReported = max(w.ReviewReported, line)
			}
			delete(w.ReportWrites, b.ToolUseID)
		}
		if w.CodeEdits[b.ToolUseID] {
			w.LastEdit = max(w.LastEdit, e.Line)
			delete(w.CodeEdits, b.ToolUseID)
		}
	}
	if e.Type != "assistant" {
		return
	}
	for _, b := range e.ToolUses() {
		switch {
		case b.Name == "Skill" && b.Input.Skill == reviewSkill:
			w.start(b.ID, e.Line, w.FixTurn || strings.Contains(b.Input.Args, "--fix"))
		case b.Name == "AskUserQuestion":
			w.Asks[b.ID] = true
		case slices.Contains(transcript.EditTools, b.Name) && scratchReport(h, b.Path(), "review"):
			w.ReportWrites[b.ID] = e.Line
		case slices.Contains(transcript.EditTools, b.Name):
			if p := b.Path(); fsx.Within(plansDir, p) {
				w.Ticked = append(w.Ticked, p)
			} else if isCode(p) {
				w.LastEdit, w.CodeEdits[b.ID] = e.Line, true
			}
		case b.Name == "Bash":
			if slices.ContainsFunc(w.bashed[b.ID], isCode) {
				w.LastEdit, w.CodeEdits[b.ID] = e.Line, true
			}
			w.Ticked = append(w.Ticked, w.bashTicked(b.Input.Command, e.Timestamp, plansDir)...)
		}
	}
}

// bashTicked is each plan in plansDir that command names by its file name,
// a whole word of a path, and that changed after at, the call's timestamp:
// the bash-edits record holds only code files, so it can't show a plan a
// Bash call ticked.
func (w *planWalk) bashTicked(command, at, plansDir string) []string {
	start, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return nil
	}
	if !w.listed {
		paths, _ := filepath.Glob(filepath.Join(plansDir, "*.md"))
		for _, p := range paths {
			w.plans = append(w.plans, listedPlan{p, namesFile(filepath.Base(p))})
		}
		w.listed = true
	}
	var ticked []string
	for _, p := range w.plans {
		if !p.named.MatchString(command) {
			continue
		}
		if info, err := os.Stat(p.path); err == nil && info.ModTime().After(start) {
			ticked = append(ticked, p.path)
		}
	}
	return ticked
}

// done is the Stop hook's end-of-plan moment (skills/kit-rules/reference/verify.md): plan
// is the plan whose last open step this turn ticked, else ""; reviewed is
// whether a /code-review started after the last code edit, anywhere in the
// transcript, has finished.
func (w *planWalk) done() (plan string, reviewed bool) {
	for _, path := range w.Ticked {
		if data, err := os.ReadFile(path); err == nil {
			if s := steps(data); !openStep.Match(s) && doneStep.Match(s) {
				plan = path
			}
		}
	}
	return plan, w.LastReview > w.LastEdit
}

// launched is true when the transcript shows a /code-review started.
func (w *planWalk) launched() bool { return len(w.Started) > 0 }

// listedPlan is a plan's path, and named, which matches a command naming it.
type listedPlan struct {
	path  string
	named *regexp.Regexp
}

// namesFile matches name with no file-name character on either side:
// "plan.md" is in "plans/plan.md", not in "myplan.md".
func namesFile(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?:^|[^\w.-])` + regexp.QuoteMeta(name) + `(?:$|[^\w.-])`)
}
