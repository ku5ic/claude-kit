package claims

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// session is a transcript: a prompt, a Bash call and its result, a reply,
// a connector read and a connector write, and a sidechain entry.
func session(t *testing.T) string {
	t.Helper()
	lines := []string{
		`{"type":"user","message":{"content":"why does the build fail?"}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"go build ./..."}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"undefined: Foo"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Foo was renamed."}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t2","name":"mcp__tracker__get_issue","input":{"id":"7"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t3","name":"mcp__chat__send_message","input":{"text":"hi"}}]}}`,
		`{"type":"user","isSidechain":true,"message":{"content":"sidechain prompt"}}`,
	}
	path := filepath.Join(t.TempDir(), "session.jsonl")
	testutil.Write(t, path, strings.Join(lines, "\n")+"\n")
	return path
}

func baseConfig() *config.Config {
	return &config.Config{
		Connectors: config.Connectors{WriteVerbs: []string{"send", "create"}, ReadVerbs: []string{"get", "list", "query"}},
		Claims:     config.Claims{Model: "sonnet", Timeout: 30, BudgetUSD: 1},
	}
}

// stub is a verifier command that saves its stdin and KIT_VERIFIER to dir,
// then prints answer.
func stub(t *testing.T, answer string) (argv []string, dir string) {
	t.Helper()
	dir = t.TempDir()
	testutil.Write(t, filepath.Join(dir, "answer"), answer)
	path := filepath.Join(dir, "verifier")
	testutil.FakeTool(t, path, filepath.Join(dir, "calls"),
		`cat >"`+dir+`/stdin"; printf %s "$`+Guard+`" >"`+dir+`/guard"; cat "`+dir+`/answer"`)
	return []string{path}, dir
}

// envelope is claude's --output-format json reply carrying structured.
func envelope(structured any) string {
	data, _ := json.Marshal(map[string]any{"is_error": false, "subtype": "success", "structured_output": structured})
	return string(data)
}

func TestVerify(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		answer  string
		want    Verdict
		wantErr bool
	}{
		{"ok", envelope(map[string]any{"ok": true, "unsupported": []any{}}), Verdict{OK: true}, false},
		{"unsupported claims", envelope(map[string]any{"ok": false, "unsupported": []map[string]string{{"claim": "Foo was renamed.", "check": "git log -S Foo"}}}),
			Verdict{Unsupported: []Unsupported{{"Foo was renamed.", "git log -S Foo"}}}, false},
		{"ok with a claim listed is not ok", envelope(map[string]any{"ok": true, "unsupported": []map[string]string{{"claim": "x", "check": "y"}}}),
			Verdict{Unsupported: []Unsupported{{"x", "y"}}}, false},
		{"an error envelope", `{"is_error":true,"subtype":"error_max_budget_usd"}`, Verdict{}, true},
		{"not JSON", "Sure! Here is the verdict.", Verdict{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := baseConfig()
			cfg.Claims.Command, _ = stub(t, tc.answer)
			got, err := Verify(cfg, "", t.TempDir(), t.TempDir(), "The build fails because Foo was renamed.", session(t))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if !tc.wantErr && (got.OK != tc.want.OK || !slices.Equal(got.Unsupported, tc.want.Unsupported)) {
				t.Errorf("verdict = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestAVerifierThatFailsStillCountsItsRun(t *testing.T) {
	t.Parallel()
	cfg := baseConfig()
	dir := t.TempDir()
	cfg.Claims.Command = []string{filepath.Join(dir, "verifier")}
	testutil.FakeTool(t, cfg.Claims.Command[0], filepath.Join(dir, "calls"), "exit 1")
	got, err := Verify(cfg, "", t.TempDir(), t.TempDir(), "The build fails.", "")
	if err == nil || got.Usage.Runs != 1 {
		t.Errorf("a failed run: err %v, runs %d; want an error and 1 run", err, got.Usage.Runs)
	}
}

func TestVerifyRunsInDirWithGuardAndEvidence(t *testing.T) {
	t.Parallel()
	cfg := baseConfig()
	argv, dir := stub(t, envelope(map[string]any{"ok": true, "unsupported": []any{}}))
	cfg.Claims.Command = argv
	work, cacheDir := testutil.Physical(t, t.TempDir()), t.TempDir()
	if _, err := Verify(cfg, "", cacheDir, work, "The build fails.", session(t)); err != nil {
		t.Fatal(err)
	}
	if calls := testutil.Calls(t, filepath.Join(dir, "calls")); len(calls) != 1 || !strings.HasPrefix(calls[0], work+"|") {
		t.Errorf("calls = %q, want one run in %s", calls, work)
	}
	// A fresh random value per run, never a literal a user could set.
	if guard := testutil.Read(t, filepath.Join(dir, "guard")); len(guard) != 32 {
		t.Errorf("%s = %q, want a 32-character token", Guard, guard)
	}
	if left, _ := os.ReadDir(filepath.Join(cacheDir, cache.Guard)); len(left) != 0 {
		t.Errorf("the run's marker outlived it: %v", left)
	}
	stdin := testutil.Read(t, filepath.Join(dir, "stdin"))
	// A subagent's own transcript is all sidechain, so sidechain entries count.
	for _, want := range []string{"why does the build fail?", `{"command":"go build ./..."}`, "Result of Bash", "undefined: Foo", "Foo was renamed.", "sidechain prompt", "## Text to verify\n\nThe build fails."} {
		if !strings.Contains(stdin, want) {
			t.Errorf("evidence lacks %q:\n%s", want, stdin)
		}
	}
}

func TestVerifyKeepsAForkedSkillsTask(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		forked bool
	}{{"a forked skill", true}, {"no sidecar", false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := baseConfig()
			argv, dir := stub(t, envelope(map[string]any{"ok": true, "unsupported": []any{}}))
			cfg.Claims.Command = argv
			path := filepath.Join(t.TempDir(), "agent-a1.jsonl")
			testutil.Write(t, path, strings.Join([]string{
				`{"type":"user","isMeta":true,"isSidechain":true,"message":{"content":"review the diff"}}`,
				`{"type":"user","isMeta":true,"isSidechain":true,"message":{"content":"skill body"}}`,
			}, "\n")+"\n")
			if tc.forked {
				testutil.Write(t, strings.TrimSuffix(path, ".jsonl")+".forked-skill.json", `{"skillName":"code-review"}`)
			}
			if _, err := Verify(cfg, "", t.TempDir(), t.TempDir(), "The build fails.", path); err != nil {
				t.Fatal(err)
			}
			stdin := testutil.Read(t, filepath.Join(dir, "stdin"))
			if got := strings.Contains(stdin, "review the diff"); got != tc.forked {
				t.Errorf("task in evidence = %v, want %v:\n%s", got, tc.forked, stdin)
			}
			if strings.Contains(stdin, "skill body") {
				t.Errorf("a later meta entry got in:\n%s", stdin)
			}
		})
	}
}

func TestVerifyOff(t *testing.T) {
	t.Parallel()
	for _, claims := range []config.Claims{{Timeout: 0, BudgetUSD: 1}, {Timeout: 30, BudgetUSD: 0}} {
		cfg := baseConfig()
		cfg.Claims = claims
		var calls string
		cfg.Claims.Command, calls = stub(t, `{"structured_output":{"ok":true}}`)
		if _, err := Verify(cfg, "", t.TempDir(), t.TempDir(), "text", ""); err == nil {
			t.Errorf("%+v: no error from a verifier that is off", claims)
		}
		if got := testutil.Calls(t, filepath.Join(calls, "calls")); got != nil {
			t.Errorf("%+v: the verifier ran: %q", claims, got)
		}
	}
}

// The hook's own run and the agent Claude starts read one text.
func TestSystemPromptIsTheAgentFilesBody(t *testing.T) {
	t.Parallel()
	prompt, err := systemPrompt("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(prompt, "You verify an AI assistant's text") || strings.Contains(prompt, "name: claim-checker") {
		t.Errorf("prompt:\n%.200s", prompt)
	}
	if _, err := systemPrompt(t.TempDir()); err == nil {
		t.Error("no error without the agent file")
	}
}

func TestClaudeAllowsOnlyReads(t *testing.T) {
	t.Parallel()
	cfg := baseConfig()
	cfg.Claims.Allow = []string{"mcp__docs__query_docs", "mcp__chat__create_post", "Bash", "Write", "Edit", "mcp__chat", "mcp__chat__*", "mcp__browser__navigate"}
	entries, err := transcript.Load(session(t))
	if err != nil {
		t.Fatal(err)
	}
	argv := claude(cfg, "/kit", entries, "prompt")
	flag := func(name string) []string {
		i := slices.Index(argv, name)
		if i < 0 || i+1 >= len(argv) {
			t.Fatalf("argv lacks %s: %q", name, argv)
		}
		return strings.Split(argv[i+1], ",")
	}
	allowed, tools := flag("--allowedTools"), flag("--tools")
	for _, want := range []string{"Read", "Grep", "WebSearch", "WebFetch", "ToolSearch", "Bash(/kit/bin/kit read *)", "mcp__tracker__get_issue", "mcp__docs__query_docs"} {
		if !slices.Contains(allowed, want) {
			t.Errorf("--allowedTools lacks %s: %q", want, allowed)
		}
	}
	for _, never := range []string{"mcp__chat__send_message", "mcp__chat__create_post", "Bash", "Edit", "Write", "mcp__chat", "mcp__chat__*", "mcp__browser__navigate"} {
		if slices.Contains(allowed, never) || never != "Bash" && slices.Contains(tools, never) {
			t.Errorf("%s is let through: allowed %q, tools %q", never, allowed, tools)
		}
	}
	if !slices.Contains(tools, "WebFetch") || !slices.Contains(tools, "Bash") || slices.Contains(tools, "mcp__tracker__get_issue") {
		t.Errorf("--tools = %q, want the builtins, WebFetch, and Bash, no MCP tool", tools)
	}
	if prompt := flag("--system-prompt"); !strings.Contains(strings.Join(prompt, ","), "/kit/bin/kit read git") {
		t.Errorf("the system prompt doesn't name kit read: %q", prompt)
	}
	for _, want := range []string{"--restricted", "dontAsk", "sonnet"} {
		if !slices.Contains(argv, want) {
			t.Errorf("argv lacks %s: %q", want, argv)
		}
	}
}

// result is a tool_result entry whose output is text.
func result(t *testing.T, text string) transcript.Entry {
	t.Helper()
	content, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	return transcript.Entry{Type: "user", Blocks: []transcript.Block{{Type: "tool_result", Content: content}}}
}

func TestEvidenceCutsOldestFirst(t *testing.T) {
	t.Parallel()
	var entries []transcript.Entry
	for i := range 40 {
		entries = append(entries, result(t, strings.Repeat("x", 10000)+"result-"+string(rune('A'+i%26))))
	}
	got := evidence(entries, "claim", false)
	if len(got) > evidenceCap+1000 {
		t.Errorf("evidence is %d bytes, cap %d", len(got), evidenceCap)
	}
	if !strings.Contains(got, "older items cut") || !strings.HasSuffix(got, "## Text to verify\n\nclaim\n") {
		t.Errorf("evidence lacks the cut note or the text:\n%s", got[:200])
	}
	if !strings.Contains(got, "result-N") { // the 40th entry, newest
		t.Error("the newest entry was cut")
	}
}

// A prompt past the cap once cut everything, itself included, so the
// verifier saw no evidence and flagged every claim.
func TestEvidenceKeepsEveryPromptWhole(t *testing.T) {
	t.Parallel()
	entries := []transcript.Entry{
		{Type: "user", Text: "first prompt"},
		result(t, "old result"),
		{Type: "user", Text: strings.Repeat("y", evidenceCap+1) + "pasted log end"},
		result(t, "new result"),
	}
	got := evidence(entries, "claim", false)
	for _, want := range []string{"[2 older items cut]", "first prompt", "pasted log end", "## Text to verify\n\nclaim\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("evidence lacks %q", want)
		}
	}
	if strings.Contains(got, "old result") || strings.Contains(got, "new result") {
		t.Error("a result past the cap was kept")
	}
}

// A result a pending statement names is kept past the cap, with its call,
// whatever its age; the rest fills newest first.
func TestEvidenceKeepsAResultTheTextNames(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input, result, text string
	}{
		{"a path its call read", `{"file_path":"/r/go/internal/claims/refs.go"}`, "old-marker", "The cap is in go/internal/claims/refs.go."},
		{"a file:line's path", `{"file_path":"/r/go/internal/claims/refs.go"}`, "old-marker", "The cap is set at `go/internal/claims/refs.go:42`."},
		{"a SHA its result shows", `{}`, "8888325 old-marker", "The ledger landed in 8888325 last week."},
		{"a backticked symbol", `{}`, "func evidenceCap() old-marker", "The `evidenceCap` constant caps it."},
		{"a number", `{}`, "300000 old-marker", "The cap is 300000 bytes in all."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			content, _ := json.Marshal(tc.result)
			entries := []transcript.Entry{
				{Type: "assistant", Blocks: []transcript.Block{{Type: "tool_use", ID: "r0", Name: "Read", RawInput: json.RawMessage(tc.input)}}},
				{Type: "user", Blocks: []transcript.Block{{Type: "tool_result", ToolUseID: "r0", Content: content}}},
			}
			for range 40 {
				entries = append(entries, result(t, strings.Repeat("x", 10000)))
			}
			got := evidence(entries, tc.text, false)
			if !strings.Contains(got, "old-marker") || !strings.Contains(got, "#### Tool Read") {
				t.Errorf("evidence lacks the named result or its call")
			}
			if len(got) > evidenceCap+1000 {
				t.Errorf("evidence is %d bytes, cap %d", len(got), evidenceCap)
			}
		})
	}
}

// Named results past the cap are cut too, oldest first.
func TestEvidenceCapsNamedResults(t *testing.T) {
	t.Parallel()
	var entries []transcript.Entry
	for range 40 {
		entries = append(entries, result(t, strings.Repeat("x", 10000)+" 300000"))
	}
	if got := evidence(entries, "The cap is 300000 bytes in all.", false); len(got) > evidenceCap+1000 {
		t.Errorf("evidence is %d bytes, cap %d", len(got), evidenceCap)
	}
}

func TestEvidenceClipsTheText(t *testing.T) {
	t.Parallel()
	got := evidence(nil, strings.Repeat("The page says hello. ", 200000), false)
	if len(got) > textCap+200 || !strings.Contains(got, "bytes cut]") {
		t.Errorf("evidence is %d bytes, want the text clipped to %d with a note", len(got), textCap)
	}
}

// A subagent's report is its own claims, so it can't confirm the text's.
func TestEvidenceMarksAnAgentResultAsNoEvidence(t *testing.T) {
	t.Parallel()
	call := func(id, name string) transcript.Entry {
		return transcript.Entry{Type: "assistant", Blocks: []transcript.Block{{Type: "tool_use", ID: id, Name: name, RawInput: json.RawMessage(`{}`)}}}
	}
	answer := func(id, text string) transcript.Entry {
		e := result(t, text)
		e.Blocks[0].ToolUseID = id
		return e
	}
	got := evidence([]transcript.Entry{call("r", "Read"), answer("r", "file text"), call("a", "Agent"), answer("a", "the tags stop at v0.2.2")}, "claim", false)
	for _, want := range []string{"#### Result of Read\n\nfile text", "#### Result of Agent (not evidence)\n\nthe tags stop at v0.2.2"} {
		if !strings.Contains(got, want) {
			t.Errorf("evidence lacks %q:\n%s", want, got)
		}
	}
}

// A finding's Evidence names the read or command behind it. One the
// session never ran, or a field naming none, is flagged with no model run.
func TestAFindingsEvidenceMustNameWhatTheSessionRan(t *testing.T) {
	t.Parallel()
	finding := func(evidence string) string {
		return "### warning: the build fails\n\n`main.go:3` calls Foo, which is gone.\n\n- **Evidence:** " + evidence + "\n"
	}
	for _, tc := range []struct {
		name, evidence string
		flagged        bool
	}{
		{"a command that ran, and a value its output showed", "`go build ./...` printed `undefined: Foo`.", false},
		{"a command that never ran", "`go vet ./...` printed `undefined: Foo`.", true},
		{"a field naming no read or command", "the build output.", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := baseConfig()
			var dir string
			cfg.Claims.Command, dir = stub(t, envelope(map[string]any{"ok": true, "unsupported": []any{}}))
			v, err := Verify(cfg, "", t.TempDir(), t.TempDir(), finding(tc.evidence), session(t))
			if err != nil {
				t.Fatal(err)
			}
			if got := !v.OK && len(v.Unsupported) == 1 && strings.Contains(v.Unsupported[0].Claim, tc.evidence); got != tc.flagged {
				t.Errorf("flagged = %v, want %v: %+v", got, tc.flagged, v)
			}
			if runs := testutil.Calls(t, filepath.Join(dir, "calls")); tc.flagged && runs != nil {
				t.Errorf("a flagged finding still ran the model: %q", runs)
			}
		})
	}
}

// A path read relative to the call's directory, its entry's cwd or a cd
// in the command, names the same file as the repo-relative path.
func TestAnEvidencePathMatchesARelativeRead(t *testing.T) {
	t.Parallel()
	bash := func(cwd, command string) transcript.Entry {
		b := transcript.Block{Type: "tool_use", ID: command, Name: "Bash"}
		b.Input.Command = command
		return transcript.Entry{Type: "assistant", Cwd: cwd, Blocks: []transcript.Block{b}}
	}
	entries := []transcript.Entry{
		bash("/r", "cd /r/go/internal && sed -n 1,9p claims/named.go"),
		bash("/r/go", "sed -n 1p internal/x.go"),
	}
	for evidence, flagged := range map[string]bool{
		"`go/internal/claims/named.go:3` reads it.": false,
		"`go/internal/x.go:1` reads it.":            false,
		"`go/internal/claims/other.go:1` reads it.": true,
	} {
		got := ungrounded("/r", entries, "- **Evidence:** "+evidence+"\n")
		if (len(got) > 0) != flagged {
			t.Errorf("%s: flagged = %v, want %v", evidence, len(got) > 0, flagged)
		}
	}
}

func TestStatements(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text string
		want       []string
	}{
		{"a sentence wrapped over two lines", "The verifier runs from\nthe project root.", []string{"The verifier runs from the project root."}},
		{"a line ending in a parenthesis", "Evidence is cut at 20 KB per result (`claims.go:68`)", []string{"Evidence is cut at 20 KB per result (`claims.go:68`)"}},
		{"a line ending in a colon", "The verifier has three tools:\n\n- Read", []string{"The verifier has three tools:"}},
		{"a heading stays apart from its paragraph", "## All 42 tests pass\nThe gate ran twice today.", []string{"## All 42 tests pass", "The gate ran twice today."}},
		{"a question isn't one", "Which branch should I use?", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Statements(tc.text); !slices.Equal(got, tc.want) {
				t.Errorf("Statements(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestDeclarative(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]bool{
		"The build fails because Foo is undefined.": true,
		"Committed as abc123.":                      true,
		"Tests pass!":                               false,
		"Done.":                                     false,
		"Which branch should I use?":                false,
		"Want me to commit it? Or should I wait for review?":                        false,
		"```go\nfunc main() { fmt.Println(\"it works.\") }\n```":                    false,
		"Here it is:\n\n```sh\nmake build\n```\n\nIt writes the binary to bin/kit.": true,
		"":                                   false,
		"- `foo.go:12` returns nil on error": true,
		"| auth.go | token never expires |":  true,
		"1. The cache is never pruned":       true,
		"- Which branch should I use?":       false,
		"- Done":                             false,
		"|------|------|":                    false,
		"## Verification":                    false,
		"## All 42 tests pass":               true,
		"foo.go:12 returns nil.":             true,
		"All 42 tests pass.":                 true,
		"- All 42 tests pass":                true,
		"Exit 0.":                            false,
	} {
		if got := Declarative(text); got != want {
			t.Errorf("Declarative(%q) = %v, want %v", text, got, want)
		}
	}
}

// The verifier gets a Read deny per sensitive_paths entry, so neither a
// claim nor an instruction injected into the evidence gets it to read a
// credential file; a "!" exception can't narrow a deny, so it's dropped. A
// glob whose literal part is too short to name a file gets no Bash deny,
// which would match unrelated commands.
func TestClaudeDeniesSensitiveReads(t *testing.T) {
	t.Parallel()
	cfg := baseConfig()
	cfg.SensitivePaths = []string{".env", "!.env.example", "*.pem", "~/.ssh/", "~/.netrc", "id_*"}
	argv := claude(cfg, "/kit", nil, "prompt")
	i := slices.Index(argv, "--disallowedTools")
	if i < 0 || i+1 >= len(argv) {
		t.Fatalf("argv lacks --disallowedTools: %q", argv)
	}
	denied := strings.Split(argv[i+1], ",")
	// "//" anchors at the filesystem root: a bare "**/" would cover only
	// the verifier's cwd (code.claude.com/docs/en/permissions).
	if want := []string{"Read(//**/.env)", "Bash(*.env*)", "Read(//**/*.pem)", "Bash(*.pem*)", "Read(~/.ssh/**)", "Bash(*.ssh/*)", "Read(~/.netrc)", "Bash(*.netrc*)", "Read(//**/id_*)"}; !slices.Equal(denied, want) {
		t.Errorf("--disallowedTools = %q, want %q", denied, want)
	}
}

// A run that fails on one text (its budget, its turns, a timeout) pauses
// that text alone; a failure of the verifier itself pauses every text.
func TestAFailureOnOneTextPausesOnlyThatText(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, script string
		othersPaused bool
	}{
		{"over its budget", `printf %s '{"is_error":true,"subtype":"error_max_budget_usd"}'`, false},
		{"the verifier failing", "exit 1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, cacheDir, dir := baseConfig(), t.TempDir(), t.TempDir()
			cfg.Claims.Command = []string{filepath.Join(dir, "verifier")}
			testutil.FakeTool(t, cfg.Claims.Command[0], filepath.Join(dir, "calls"), tc.script)
			if _, err := Verify(cfg, "", cacheDir, t.TempDir(), "The build fails.", ""); err == nil {
				t.Fatal("the first run didn't fail")
			}
			var paused *Paused
			if _, err := Verify(cfg, "", cacheDir, t.TempDir(), "The build fails.", ""); !errors.As(err, &paused) {
				t.Errorf("the same text again: err %v, want paused", err)
			}
			_, err := Verify(cfg, "", cacheDir, t.TempDir(), "The tests pass.", "")
			if got := errors.As(err, &paused); got != tc.othersPaused {
				t.Errorf("another text paused = %v (err %v), want %v", got, err, tc.othersPaused)
			}
		})
	}
}

// One text timing out pauses only it, but a second text timing out within
// the cooldown is the verifier hanging, so it pauses every text.
func TestASecondTextTimingOutPausesEveryText(t *testing.T) {
	t.Parallel()
	cfg, cacheDir, dir := baseConfig(), t.TempDir(), t.TempDir()
	cfg.Claims.Timeout = 1
	cfg.Claims.Command = []string{filepath.Join(dir, "verifier")}
	testutil.FakeTool(t, cfg.Claims.Command[0], filepath.Join(dir, "calls"), "sleep 5")
	var paused *Paused
	for i, text := range []string{"The build fails.", "The tests pass."} {
		if _, err := Verify(cfg, "", cacheDir, t.TempDir(), text, ""); err == nil || errors.As(err, &paused) {
			t.Fatalf("text %d: err %v, want a timeout", i, err)
		}
	}
	if _, err := Verify(cfg, "", cacheDir, t.TempDir(), "The lint is clean.", ""); !errors.As(err, &paused) {
		t.Errorf("a third text: err %v, want paused", err)
	}
}

// repo is a git repo holding go/x.go, three lines defining Hello, and
// .github/ci.yml, naming cache-key, in one commit, and that commit's SHA;
// gen/out.go, defining Generated, is gitignored.
func repo(t *testing.T) (dir, sha string) {
	t.Helper()
	dir = testutil.Physical(t, t.TempDir())
	testutil.Put(t, filepath.Join(dir, "go"), "x.go", "package x\n\nfunc Hello() {}\n")
	testutil.Put(t, filepath.Join(dir, ".github"), "ci.yml", "cache-key: v1\n")
	testutil.Put(t, dir, ".gitignore", "gen/\n")
	testutil.Put(t, filepath.Join(dir, "gen"), "out.go", "package gen\n\nfunc Generated() {}\n")
	testutil.Git(t, dir, "init", "-q")
	testutil.Git(t, dir, "add", ".")
	testutil.GitID(t, dir, "commit", "-qm", "x")
	sha, err := git.Line(dir, "rev-parse", "--short=12", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return dir, sha
}

// A statement that is only references, each resolved in the repo, needs no
// model; an invented one is flagged without one; a statement that says
// something about a reference goes to the model, without the bare ones.
func TestBareReferencesResolveWithoutAModel(t *testing.T) {
	t.Parallel()
	dir, sha := repo(t)
	for _, tc := range []struct {
		name, text string
		runs       int
		flagged    []string
	}{
		{"a real file:line", "See `go/x.go:3`.", 0, nil},
		{"an invented line", "See `go/x.go:9`.", 0, []string{"See `go/x.go:9`."}},
		{"a real path", "- `go/x.go`", 0, nil},
		{"an invented path", "See go/nope.go.", 0, []string{"See go/nope.go."}},
		{"a real SHA", "See commit " + sha + ".", 0, nil},
		{"an invented SHA", "See commit 0123abc4567f.", 0, []string{"See commit 0123abc4567f."}},
		{"a real symbol", "See `Hello` in `go/x.go`.", 0, nil},
		{"an invented symbol", "See `Goodbye` in `go/x.go`.", 0, []string{"See `Goodbye` in `go/x.go`."}},
		{"a symbol only in a hidden file", "See `cache-key`.", 0, nil},
		{"a symbol only in a gitignored file", "See `Generated` in `gen/out.go`.", 0, nil},
		{"a bare reference wrapped across lines", "See `go/x.go:3`\nand `go/x.go`.", 0, nil},
		{"a false claim about a real file:line", "`go/x.go:3` returns an error.", 1, nil},
		{"a behavior sentence about a real symbol", "The `Hello` function panics on empty input.", 1, nil},
		{"a reference outside the repo", "See `other/y.go:4`.", 1, nil},
		{"a table row pairing real references", "| `Hello` | `go/x.go:3` | `.github/ci.yml` |", 1, nil},
		{"a list item pairing real references", "- `Hello`: `go/x.go:3` and `.github/ci.yml`", 1, nil},
		{"a table row with an invented reference", "| `Goodbye` | `go/x.go:3` | `.github/ci.yml` |", 0, []string{"| `Goodbye` | `go/x.go:3` | `.github/ci.yml` |"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := baseConfig()
			argv, stubDir := stub(t, envelope(map[string]any{"ok": true, "unsupported": []any{}}))
			cfg.Claims.Command = argv
			got, err := Verify(cfg, "", t.TempDir(), dir, tc.text, "")
			if err != nil {
				t.Fatal(err)
			}
			if calls := testutil.Calls(t, filepath.Join(stubDir, "calls")); len(calls) != tc.runs || got.Usage.Runs != tc.runs {
				t.Errorf("runs = %d (usage %d), want %d", len(calls), got.Usage.Runs, tc.runs)
			}
			var claims []string
			for _, u := range got.Unsupported {
				claims = append(claims, u.Claim)
			}
			if !slices.Equal(claims, tc.flagged) || got.OK != (len(tc.flagged) == 0) {
				t.Errorf("verdict = %+v, want flagged %q", got, tc.flagged)
			}
		})
	}
}

// Past its deadline the no-model pass looks nothing up, so a reply full of
// references can't hold Stop past its timeout; what's left goes to the model.
func TestReferencesPastTheDeadlineStayUnsettled(t *testing.T) {
	t.Parallel()
	dir, _ := repo(t)
	const text = "See `go/x.go:9`. See `Goodbye`."
	flagged, rest := settleReferences(dir, text, time.Now().Add(-time.Second))
	if len(flagged) != 0 || rest != text {
		t.Errorf("flagged %+v, rest %q; want none flagged and the text whole", flagged, rest)
	}
}

func TestTheModelGetsOnlyWhatReferencesDontSettle(t *testing.T) {
	t.Parallel()
	dir, _ := repo(t)
	cfg := baseConfig()
	argv, stubDir := stub(t, envelope(map[string]any{"ok": true, "unsupported": []any{}}))
	cfg.Claims.Command = argv
	if _, err := Verify(cfg, "", t.TempDir(), dir, "See `go/x.go:3`. The `Hello` function panics on empty input.", ""); err != nil {
		t.Fatal(err)
	}
	_, sent, _ := strings.Cut(testutil.Read(t, filepath.Join(stubDir, "stdin")), "## Text to verify")
	testutil.Has(t, sent, "The `Hello` function panics on empty input.")
	testutil.Lacks(t, sent, "See `go/x.go:3`.")
}

func TestCitedFilesNamesOnlyFilesUnderTheRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Put(t, root, "go/x.go", "package x\n")
	text := "See `go/x.go:3`, go/x.go, `go/gone.go`, /etc/hosts, and `go` for the 12 cases."
	if got, want := CitedFiles(root, text), []string{"go/x.go"}; !slices.Equal(got, want) {
		t.Errorf("CitedFiles = %q, want %q", got, want)
	}
}
