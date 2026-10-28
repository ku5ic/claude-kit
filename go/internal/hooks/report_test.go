package hooks

import "testing"

// A run-checks run's output, a failing gate's detail under its FAIL line.
const gatesOutput = `PASS lint (ci.yml: go/vet) [go]
FAIL test (ci.yml: go/test) [go]
  cmd: go test ./...
FAIL	github.com/x/y	0.1s
PASS src/a.test.ts from a tool's own output, indented by nothing
SKIP typecheck [go] (nothing enforces it)

checks: 2 passed, 1 failed, 1 skipped
`

// report-checks shows the user every verdict line of a Bash call that
// printed a run-checks summary, and nothing for any other call.
func TestReportChecks(t *testing.T) {
	t.Parallel()
	bash := func(command, stdout string) map[string]any {
		return map[string]any{"hook_event_name": "PostToolUse", "tool_name": "Bash",
			"tool_input": map[string]any{"command": command}, "tool_response": map[string]any{"stdout": stdout, "stderr": ""}}
	}
	t.Run("a run's summary and every verdict line reach the user", func(t *testing.T) {
		t.Parallel()
		r := newSandbox(t).run("report-checks", bash("kit run-checks 2>&1 | rg '^(PASS|FAIL|SKIP|checks:)'", gatesOutput))
		r.Want(t, 0)
		r.Has(t, `"systemMessage":"run-checks checks: 2 passed, 1 failed, 1 skipped\n  PASS lint (ci.yml: go/vet) [go]\n  FAIL test (ci.yml: go/test) [go]\n`,
			`\n  SKIP typecheck [go] (nothing enforces it)"`)
		r.Lacks(t, "cmd: go test", "github.com/x/y", "additionalContext")
	})
	for name, call := range map[string][2]string{
		"a plan prints no summary":                  {"kit run-checks --plan", "RUN lint (ci.yml: go/vet)\n  cmd: go vet ./...\n"},
		"kit explain stop reprints a Stop run":      {"kit explain stop", gatesOutput},
		"another command":                           {"echo hi", "PASS is a word here, but nothing ran\n"},
		"a run-checks call with no output (killed)": {"kit run-checks", ""},
	} {
		t.Run(name+": nothing reaches the user", func(t *testing.T) {
			t.Parallel()
			r := newSandbox(t).run("report-checks", bash(call[0], call[1]))
			r.Want(t, 0)
			r.Empty(t)
		})
	}
}
