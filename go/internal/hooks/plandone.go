package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

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

// planDone reads the Stop hook's end-of-plan moment (rules/verify.md) from
// the transcript: plan is the plan in plansDir whose last open step this
// turn ticked, else ""; reviewed is whether a /code-review started after the
// last code edit, anywhere in the transcript, has finished. isCode is false
// for paths whose edits aren't code (scratch, outside the repo).
func planDone(path, plansDir string, isCode func(string) bool) (plan string, reviewed bool) {
	// Transcript line numbers; -1 is never.
	lastEdit, lastReview := -1, -1
	var ticked []string         // plans this turn edited
	started := map[string]int{} // review tool_use or task id -> its start line
	// A read error leaves what was read: the hook fails open.
	_ = transcript.Each(path, func(e transcript.Entry) {
		if id := launchedReview(e.Notice); id != "" {
			started[id] = e.Line
		}
		// A notification arrives as a user message or a queued command.
		lastReview = max(lastReview, finishedReview(e.Text, started), finishedReview(e.Notice, started))
		if e.StartsTurn() {
			ticked = ticked[:0]
			return
		}
		if e.Type != "assistant" {
			return
		}
		for _, b := range e.ToolUses() {
			switch {
			case b.Name == "Skill" && b.Input.Skill == reviewSkill:
				started[b.ID] = e.Line
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
		if data, err := os.ReadFile(path); err == nil {
			if s := steps(data); !openStep.Match(s) && doneStep.Match(s) {
				plan = path
			}
		}
	}
	return plan, lastReview > lastEdit
}
