package hooks

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strings"

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

// finishedReview is the transcript line where the review a completed
// task-notification reports was started, or -1. A killed review never counts.
func finishedReview(text string, started map[string]int) int {
	if !strings.HasPrefix(strings.TrimSpace(text), "<task-notification>") {
		return -1
	}
	fields := map[string]string{}
	for _, m := range notifyField.FindAllStringSubmatch(text, -1) {
		if _, seen := fields[m[1]]; !seen {
			fields[m[1]] = m[2]
		}
	}
	if fields["status"] != "completed" {
		return -1
	}
	for _, id := range []string{fields["tool-use-id"], fields["task-id"]} {
		if line, ok := started[id]; ok && id != "" {
			return line
		}
	}
	return -1
}

// planWalk is the plan-done state as it walks a transcript, kept between
// Stop runs so each one reads only the lines since the last. Lines are
// transcript line numbers; -1 is never.
type planWalk struct {
	LastEdit   int            `json:"last_edit"`
	LastReview int            `json:"last_review"`
	Started    map[string]int `json:"started"` // review tool_use or task id -> its start line
	Ticked     []string       `json:"ticked"`  // plans this turn edited
	// ReviewArrived is where a finished review's notification landed, and
	// ReviewReported where a review-*.md scratch report was last written.
	ReviewArrived  int `json:"review_arrived"`
	ReviewReported int `json:"review_reported"`
	// ReportWrites is a report write's tool_use id -> its line, until its
	// result: a write another guard denied wrote no report.
	ReportWrites map[string]int `json:"report_writes"`
}

func newPlanWalk() *planWalk {
	return &planWalk{LastEdit: -1, LastReview: -1, Started: map[string]int{}, ReviewArrived: -1, ReviewReported: -1, ReportWrites: map[string]int{}}
}

// step takes in entry e. isCode is false for paths whose edits aren't code
// (scratch, outside the repo).
func (w *planWalk) step(h *hook.Hook, e transcript.Entry, plansDir string, isCode func(string) bool) {
	if id := launchedReview(e.Notice); id != "" {
		w.Started[id] = e.Line
	}
	// A notification arrives as a user message or a queued command.
	if started := max(finishedReview(e.Text, w.Started), finishedReview(e.Notice, w.Started)); started >= 0 {
		w.LastReview, w.ReviewArrived = max(w.LastReview, started), e.Line
	}
	if e.StartsTurn() {
		w.Ticked = nil
		return
	}
	for _, b := range e.Blocks {
		if line, ok := w.ReportWrites[b.ToolUseID]; ok && b.Type == "tool_result" {
			if !b.IsError {
				w.ReviewReported = max(w.ReviewReported, line)
			}
			delete(w.ReportWrites, b.ToolUseID)
		}
	}
	if e.Type != "assistant" {
		return
	}
	for _, b := range e.ToolUses() {
		switch {
		case b.Name == "Skill" && b.Input.Skill == reviewSkill:
			w.Started[b.ID] = e.Line
		case slices.Contains(transcript.EditTools, b.Name) && isReviewReport(h, b.Path()):
			if w.ReportWrites == nil {
				w.ReportWrites = map[string]int{}
			}
			w.ReportWrites[b.ID] = e.Line
		case slices.Contains(transcript.EditTools, b.Name):
			if p := b.Path(); fsx.Within(plansDir, p) {
				w.Ticked = append(w.Ticked, p)
			} else if isCode(p) {
				w.LastEdit = e.Line
			}
		}
	}
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
