package transcript

import (
	"iter"
	"regexp"
	"strings"
)

// RunStatus is how an Agent tool call ended, as the transcript shows it.
type RunStatus int

const (
	RunGoing  RunStatus = iota // no result yet, or a background run with no notice
	RunDone                    // a result, or a notice saying completed
	RunFailed                  // an error result, one cut off at maxTurns, or a notice saying anything else
)

// Pos is a block's place in a transcript: its entry's index, then the
// block's within that entry.
type Pos struct{ Entry, Block int }

// After reports whether p comes later in the transcript than q.
func (p Pos) After(q Pos) bool {
	return p.Entry > q.Entry || p.Entry == q.Entry && p.Block > q.Block
}

// Blocks is every block in entries, in order, with its place.
func Blocks(entries []Entry) iter.Seq2[Pos, Block] {
	return func(yield func(Pos, Block) bool) {
		for i, e := range entries {
			for j, b := range e.Blocks {
				if !yield(Pos{i, j}, b) {
					return
				}
			}
		}
	}
}

// Run is one Agent tool call: its block's place, its prompt, the model it
// overrode the agent's with, the agent its result names, how it ended, and
// a foreground run's report.
type Run struct {
	At     Pos
	Prompt string
	Model  string
	Agent  string
	Status RunStatus
	Result string
}

// asyncLaunched opens the Agent tool's result for a run it started in the
// background: the report comes later, in a task notification.
const asyncLaunched = "Async agent launched"

// agentIDLine is where the Agent tool's result names the run's agent, in
// both a finished run's report and a background launch.
var agentIDLine = regexp.MustCompile(`\bagentId: ([\w-]+)`)

// cutOff opens the result of a run that reached its maxTurns, in the Agent
// tool's result or a notice, which says completed all the same.
var cutOff = regexp.MustCompile(`(?:^|<result>)\s*NOTE: this agent stopped at its \d+-turn limit`)

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
		for j, b := range e.Blocks {
			switch {
			case b.Type == "tool_use" && b.Name == "Agent" && b.Input.SubagentType == agentType:
				waiting[b.ID] = len(runs)
				runs = append(runs, Run{At: Pos{i, j}, Prompt: b.Input.Prompt, Model: b.Input.Model})
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
				case cutOff.MatchString(text):
					runs[r].Status = RunFailed
				case strings.HasPrefix(text, asyncLaunched):
					background[b.ToolUseID] = r
				default:
					runs[r].Status, runs[r].Result = RunDone, text
				}
			}
		}
	}
	return runs
}

// noticeStatus is the status e's task notification gives the background
// Agent call call, "" when e isn't one for it, and "cut off" for a run that
// reached its maxTurns.
func noticeStatus(e Entry, call string) string {
	text := e.Notice + e.Text
	for _, b := range e.Blocks {
		text += b.Text
	}
	if !strings.Contains(text, "<tool-use-id>"+call+"</tool-use-id>") {
		return ""
	}
	if cutOff.MatchString(text) {
		return "cut off"
	}
	_, rest, _ := strings.Cut(text, "<status>")
	status, _, _ := strings.Cut(rest, "</status>")
	return status
}
