package claims

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
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
			got, err := Verify(cfg, t.TempDir(), "The build fails because Foo was renamed.", session(t))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if !tc.wantErr && (got.OK != tc.want.OK || !slices.Equal(got.Unsupported, tc.want.Unsupported)) {
				t.Errorf("verdict = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestVerifyRunsInDirWithGuardAndEvidence(t *testing.T) {
	t.Parallel()
	cfg := baseConfig()
	argv, dir := stub(t, envelope(map[string]any{"ok": true, "unsupported": []any{}}))
	cfg.Claims.Command = argv
	work := testutil.Physical(t, t.TempDir())
	if _, err := Verify(cfg, work, "The build fails.", session(t)); err != nil {
		t.Fatal(err)
	}
	if calls := testutil.Calls(t, filepath.Join(dir, "calls")); len(calls) != 1 || !strings.HasPrefix(calls[0], work+"|") {
		t.Errorf("calls = %q, want one run in %s", calls, work)
	}
	if guard := testutil.Read(t, filepath.Join(dir, "guard")); guard != "1" {
		t.Errorf("%s = %q, want 1", Guard, guard)
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
			if _, err := Verify(cfg, t.TempDir(), "The build fails.", path); err != nil {
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
		if _, err := Verify(cfg, t.TempDir(), "text", ""); err == nil {
			t.Errorf("%+v: no error from a verifier that is off", claims)
		}
		if got := testutil.Calls(t, filepath.Join(calls, "calls")); got != nil {
			t.Errorf("%+v: the verifier ran: %q", claims, got)
		}
	}
}

func TestClaudeAllowsOnlyReads(t *testing.T) {
	t.Parallel()
	cfg := baseConfig()
	cfg.Claims.Allow = []string{"WebFetch", "mcp__docs__query_docs", "mcp__chat__create_post", "Bash", "Write", "Edit", "mcp__chat", "mcp__chat__*", "mcp__browser__navigate"}
	entries, err := transcript.Load(session(t))
	if err != nil {
		t.Fatal(err)
	}
	argv := claude(cfg, entries)
	flag := func(name string) []string {
		i := slices.Index(argv, name)
		if i < 0 || i+1 >= len(argv) {
			t.Fatalf("argv lacks %s: %q", name, argv)
		}
		return strings.Split(argv[i+1], ",")
	}
	allowed, tools := flag("--allowedTools"), flag("--tools")
	for _, want := range []string{"Read", "Grep", "WebSearch", "ToolSearch", "mcp__tracker__get_issue", "WebFetch", "mcp__docs__query_docs"} {
		if !slices.Contains(allowed, want) {
			t.Errorf("--allowedTools lacks %s: %q", want, allowed)
		}
	}
	for _, never := range []string{"mcp__chat__send_message", "mcp__chat__create_post", "Bash", "Edit", "Write", "mcp__chat", "mcp__chat__*", "mcp__browser__navigate"} {
		if slices.Contains(allowed, never) || slices.Contains(tools, never) {
			t.Errorf("%s is let through: allowed %q, tools %q", never, allowed, tools)
		}
	}
	if !slices.Contains(tools, "WebFetch") || slices.Contains(tools, "mcp__tracker__get_issue") {
		t.Errorf("--tools = %q, want the builtins and WebFetch, no MCP tool", tools)
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

func TestEvidenceClipsTheText(t *testing.T) {
	t.Parallel()
	got := evidence(nil, strings.Repeat("The page says hello. ", 200000), false)
	if len(got) > textCap+200 || !strings.Contains(got, "bytes cut]") {
		t.Errorf("evidence is %d bytes, want the text clipped to %d with a note", len(got), textCap)
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
		"## Verification: a header":          false,
	} {
		if got := Declarative(text); got != want {
			t.Errorf("Declarative(%q) = %v, want %v", text, got, want)
		}
	}
}
