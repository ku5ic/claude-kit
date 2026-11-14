package transcript

import (
	"regexp"
	"strings"
)

// RunStatus is how an Agent tool call ended, as the transcript shows it.
type RunStatus int

const (
	RunGoing  RunStatus = iota // no result yet, or a background run with no notice
	RunDone                    // a result, or a notice saying completed
	RunFailed                  // an error result, or a notice saying anything else
)

// Run is one Agent tool call: its tool_use id, the entry it's in, its
// prompt, the agent its result names, and how it ended.
type Run struct {
	Call   string
	Index  int
	Prompt string
	Agent  string
	Status RunStatus
}

// asyncLaunched opens the Agent tool's result for a run it started in the
// background: the report comes later, in a task notification.
const asyncLaunched = "Async agent launched"

// agentIDLine is where the Agent tool's result names the run's agent, in
// both a finished run's report and a background launch.
var agentIDLine = regexp.MustCompile(`\bagentId: ([\w-]+)`)

// AgentRuns is every Agent call of agentType in entries, in order, each
// with how it ended. Once a run is done, a later notice doesn't undo it.
func AgentRuns(entries []Entry, agentType string) []Run {
	var runs []Run
	waiting := map[string]int{}    // calls awaiting their result, by tool_use id -> index in runs
	background := map[string]int{} // background runs, until a notice says completed
	for i, e := range entries {
		for call, r := range background {
			switch status := noticeStatus(e, call); status {
			case "":
			case "completed":
				runs[r].Status = RunDone
				delete(background, call)
			default:
				runs[r].Status = RunFailed
			}
		}
		for _, b := range e.Blocks {
			switch {
			case b.Type == "tool_use" && b.Name == "Agent" && b.Input.SubagentType == agentType:
				waiting[b.ID] = len(runs)
				runs = append(runs, Run{Call: b.ID, Index: i, Prompt: b.Input.Prompt})
			case b.Type == "tool_result":
				r, ok := waiting[b.ToolUseID]
				if !ok {
					continue
				}
				delete(waiting, b.ToolUseID)
				text := b.ResultText()
				if m := agentIDLine.FindStringSubmatch(text); m != nil {
					runs[r].Agent = m[1]
				}
				switch {
				case b.IsError:
					runs[r].Status = RunFailed
				case strings.HasPrefix(text, asyncLaunched):
					background[b.ToolUseID] = r
				default:
					runs[r].Status = RunDone
				}
			}
		}
	}
	return runs
}

// noticeStatus is the status e's task notification gives the background
// Agent call call, "" when e isn't one for it.
func noticeStatus(e Entry, call string) string {
	text := e.Notice + e.Text
	for _, b := range e.Blocks {
		text += b.Text
	}
	if !strings.Contains(text, "<tool-use-id>"+call+"</tool-use-id>") {
		return ""
	}
	_, rest, _ := strings.Cut(text, "<status>")
	status, _, _ := strings.Cut(rest, "</status>")
	return status
}
