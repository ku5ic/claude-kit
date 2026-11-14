package transcript

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func write(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStartsTurn(t *testing.T) {
	path := write(t,
		`{"type":"user","message":{"content":"a prompt"}}`,
		`{"type":"user","isMeta":true,"message":{"content":"meta"}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"}]}}`,
		`{"type":"user","message":{"content":[{"type":"image"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`,
		`not json`,
	)
	var got []bool
	if err := Each(path, func(e Entry) { got = append(got, e.StartsTurn()) }); err != nil {
		t.Fatal(err)
	}
	// The image-only prompt starts a turn; the unparseable line is skipped.
	if want := []bool{true, false, false, true, false}; !slices.Equal(got, want) {
		t.Errorf("StartsTurn = %v, want %v", got, want)
	}
}

func TestEachReadsAnOversizedLineAndNumbersLines(t *testing.T) {
	big := strings.Repeat("x", 100*1024)
	path := write(t,
		`{"type":"user","message":{"content":"`+big+`"}}`,
		`bad`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"/a.go"}},{"type":"tool_use","name":"NotebookEdit","input":{"notebook_path":"/b.ipynb"}}]}}`,
	)
	var lines []int
	var paths []string
	err := Each(path, func(e Entry) {
		lines = append(lines, e.Line)
		for _, b := range e.ToolUses() {
			paths = append(paths, b.Path())
		}
	})
	if err != nil || !slices.Equal(lines, []int{1, 3}) || !slices.Equal(paths, []string{"/a.go", "/b.ipynb"}) {
		t.Errorf("err=%v lines=%v paths=%v", err, lines, paths)
	}
}

func TestPromptTextAndTail(t *testing.T) {
	path := write(t,
		`{"type":"user","message":{"content":"first"}}`,
		`{"type":"user","message":{"content":[{"type":"text","text":"a"},{"type":"image"},{"type":"text","text":"b"}]}}`,
	)
	entries, err := Tail(path, 1)
	if err != nil || len(entries) != 1 || entries[0].PromptText() != "a\nb" {
		t.Errorf("err=%v entries=%+v", err, entries)
	}
	if _, err := Tail(filepath.Join(t.TempDir(), "missing"), 1); err == nil {
		t.Error("missing file: no error")
	}
}

func TestEdits(t *testing.T) {
	path := write(t,
		`{"type":"user","message":{"content":"first"}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/old.go"}}]}}`,
		`{"type":"user","message":{"content":"second"}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"/a.go"}},{"type":"tool_use","name":"Read","input":{"file_path":"/r.go"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"NotebookEdit","input":{"notebook_path":"/b.ipynb"}}]}}`,
	)
	entries, err := Load(path)
	if err != nil || len(entries) != 6 {
		t.Fatalf("Load: %d entries, %v", len(entries), err)
	}
	if got := Edits(entries); !slices.Equal(got, []string{"/old.go", "/a.go", "/b.ipynb"}) {
		t.Errorf("all edits = %q", got)
	}
}

func TestReadAgentStart(t *testing.T) {
	t.Parallel()
	const prompt = `{"type":"user","message":{"role":"user","content":"Go."}}`
	for name, tc := range map[string]struct {
		lines []string
		want  AgentStart
	}{
		"a shape object":          {[]string{prompt, `{"type":"api-request-shape","shape":{"tools":[{"name":"WebFetch"},{"name":"Read"}]}}`}, AgentStart{Tools: []string{"WebFetch", "Read"}}},
		"a shape encoded as text": {[]string{`{"type":"api-request-shape","shape":"{\"tools\":[{\"name\":\"WebFetch\"}]}"}`}, AgentStart{Tools: []string{"WebFetch"}}},
		"a shape with no tools":   {[]string{`{"type":"api-request-shape","shape":{"system":[]}}`}, AgentStart{Tools: []string{}}},
		"a fork":                  {[]string{`{"type":"fork-context-ref"}`, prompt}, AgentStart{Fork: true}},
		"neither record":          {[]string{prompt}, AgentStart{}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := ReadAgentStart(write(t, tc.lines...))
			if err != nil || got.Fork != tc.want.Fork || !slices.Equal(got.Tools, tc.want.Tools) || (got.Tools == nil) != (tc.want.Tools == nil) {
				t.Errorf("ReadAgentStart = %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

// EachFrom reads on from an offset, numbering lines on, and leaves a line
// still being written for the next read.
func TestEachFromResumesAndLeavesAPartialLine(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "t.jsonl")
	one := `{"type":"user","message":{"content":"one"}}` + "\n"
	two := `{"type":"user","message":{"content":"two"}}`
	if err := os.WriteFile(path, []byte(one+two), 0o600); err != nil {
		t.Fatal(err)
	}
	var got []string
	collect := func(e Entry, _ int64) { got = append(got, fmt.Sprintf("%d:%s", e.Line, e.Text)) }
	off, line, err := EachFrom(path, 0, 0, collect)
	if err != nil || off != int64(len(one)) || line != 1 {
		t.Fatalf("first read: offset %d, line %d, err %v", off, line, err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("\n")
	_ = f.Close()
	if _, line, _ = EachFrom(path, off, line, collect); line != 2 || strings.Join(got, ",") != "1:one,2:two" {
		t.Errorf("entries %q, last line %d", got, line)
	}
}
