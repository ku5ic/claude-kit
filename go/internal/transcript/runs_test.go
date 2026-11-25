package transcript

import (
	"slices"
	"testing"
)

func TestAgentRuns(t *testing.T) {
	call := func(id, agent string) string {
		return `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"` + id + `","name":"Agent","input":{"subagent_type":"` + agent + `","prompt":"check ` + id + `"}}]}}`
	}
	result := func(id, text string, failed bool) string {
		isError := "false"
		if failed {
			isError = "true"
		}
		return `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"` + id + `","is_error":` + isError + `,"content":"` + text + `"}]}}`
	}
	notice := func(id, status string) string {
		return `{"type":"user","message":{"content":"<task-notification>\n<tool-use-id>` + id + `</tool-use-id>\n<status>` + status + `</status>\n</task-notification>"}}`
	}
	// As Claude Code 2.1.332 writes a run that reached its maxTurns.
	const cut = `NOTE: this agent stopped at its 2-turn limit before finishing. It was still calling tools and had produced no report.`
	cutNotice := func(id string) string {
		return `{"type":"user","message":{"content":"<task-notification>\n<tool-use-id>` + id + `</tool-use-id>\n<status>completed</status>\n<summary>Agent stopped at its 2-turn limit</summary>\n<result>` + cut + `\n</result>\n</task-notification>"}}`
	}
	entries, err := Load(write(t,
		call("done", "kit:x"), result("done", "report\\nagentId: a1", false),
		call("error", "kit:x"), result("error", "denied", true),
		call("other", "kit:y"), result("other", "report", false),
		call("bg", "kit:x"), result("bg", "Async agent launched successfully.\\nagentId: a2", false),
		call("bgfail", "kit:x"), result("bgfail", "Async agent launched successfully.", false),
		call("going", "kit:x"),
		call("cut", "kit:x"), result("cut", "  "+cut+"\\n  \\nagentId: a3", false),
		call("cutbg", "kit:x"), result("cutbg", "Async agent launched successfully.", false), cutNotice("cutbg"),
		call("resumed", "kit:x"), result("resumed", "Async agent launched successfully.", false), cutNotice("resumed"), notice("resumed", "completed"),
		call("quoted", "kit:x"), result("quoted", "The plan says "+cut, false),
		notice("bgfail", "killed"), notice("bg", "completed"), notice("bg", "killed"),
		// An api-request-blob echoes a call and its result; neither counts again.
		`{"type":"api-request-blob","message":{"role":"assistant","content":[{"type":"tool_use","id":"error","name":"Agent","input":{"subagent_type":"kit:x","prompt":"check error"}}]}}`,
		`{"type":"api-request-blob","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"going","content":"report"}]}}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	type got struct {
		Prompt, Agent string
		Status        RunStatus
	}
	var runs []got
	for _, r := range AgentRuns(entries, "kit:x") {
		runs = append(runs, got{r.Prompt, r.Agent, r.Status})
	}
	// A completed notice isn't undone by a later one.
	want := []got{{"check done", "a1", RunDone}, {"check error", "", RunFailed}, {"check bg", "a2", RunDone}, {"check bgfail", "", RunFailed}, {"check going", "", RunGoing},
		{"check cut", "a3", RunFailed}, {"check cutbg", "", RunFailed}, {"check resumed", "", RunDone}, {"check quoted", "", RunDone}}
	if !slices.Equal(runs, want) {
		t.Errorf("AgentRuns =\n%v\nwant\n%v", runs, want)
	}
}
