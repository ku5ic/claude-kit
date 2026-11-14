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
	entries, err := Load(write(t,
		call("done", "kit:x"), result("done", "report\\nagentId: a1", false),
		call("error", "kit:x"), result("error", "denied", true),
		call("other", "kit:y"), result("other", "report", false),
		call("bg", "kit:x"), result("bg", "Async agent launched successfully.\\nagentId: a2", false),
		call("bgfail", "kit:x"), result("bgfail", "Async agent launched successfully.", false),
		call("going", "kit:x"),
		notice("bgfail", "killed"), notice("bg", "completed"), notice("bg", "killed"),
	))
	if err != nil {
		t.Fatal(err)
	}
	type got struct {
		Call, Agent string
		Status      RunStatus
	}
	var runs []got
	for _, r := range AgentRuns(entries, "kit:x") {
		if r.Prompt != "check "+r.Call {
			t.Errorf("%s: prompt %q", r.Call, r.Prompt)
		}
		runs = append(runs, got{r.Call, r.Agent, r.Status})
	}
	// A completed notice isn't undone by a later one.
	want := []got{{"done", "a1", RunDone}, {"error", "", RunFailed}, {"bg", "a2", RunDone}, {"bgfail", "", RunFailed}, {"going", "", RunGoing}}
	if !slices.Equal(runs, want) {
		t.Errorf("AgentRuns =\n%v\nwant\n%v", runs, want)
	}
}
