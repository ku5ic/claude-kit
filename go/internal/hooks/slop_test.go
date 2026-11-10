package hooks

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// tell blocks; dense is tier 3 vocabulary dense enough to report, the only
// kind of hit the guards log without blocking.
const (
	tell  = "I hope this helps."
	dense = "A robust, meticulous, comprehensive tool."
)

// slopCall is a tool call on path with input as tool_input, file_path set.
func slopCall(tool, path string, input map[string]any) map[string]any {
	input["file_path"] = path
	return map[string]any{"tool_name": tool, "session_id": "s", "tool_input": input}
}

func edit(from, to string) map[string]any {
	return map[string]any{"old_string": from, "new_string": to}
}

// Each case writes on (when set) to the file first, then runs one tool call
// against it with the real kit.yml.
func TestGuardSlop(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, file, on, tool string
		input                map[string]any
		deny                 bool
		has                  string
	}{
		{"deny: Write of a new Markdown file with a tier 1 tell", "doc.md", "", "Write", map[string]any{"content": "# Doc\n\n" + tell + "\n"}, true, `L3 \"I hope this helps\" -> cut it`},
		{"deny: Edit adding a tell", "doc.md", "# Doc\n\nBody.\n", "Edit", edit("Body.", "Body. "+tell), true, ""},
		{"allow: Edit next to a tell already there", "doc.md", "# Doc\n\n" + tell + "\n\nBody.\n", "Edit", edit("Body.", "Body, longer."), false, ""},
		{"allow: Edit inside an existing fence", "doc.md", "# Doc\n\n```text\nold\n```\n", "Edit", edit("old", tell), false, ""},
		{"allow: a tell under an allow marker", "doc.md", "", "Write", map[string]any{"content": "unslop: allow quoting the phrase\n" + tell + "\n"}, false, ""},
		{"deny: MultiEdit whose second edit adds a tell", "doc.md", "a\n\nb\n", "MultiEdit", map[string]any{"edits": []any{edit("a", "A"), edit("b", tell)}}, true, ""},
		{"deny: Edit with replace_all adding a tell", "doc.md", "x\n\nx\n", "Edit", map[string]any{"old_string": "x", "new_string": tell, "replace_all": true}, true, "L3"},
		{"allow (fails open): Edit whose old_string isn't in the file", "doc.md", "a\n", "Edit", edit("zzz", tell), false, ""},
		{"deny: Markdown in scratch is a report", ".claude/scratch/report.md", "", "Write", map[string]any{"content": tell + "\n"}, true, ""},
		{"allow: a log in scratch isn't read", ".claude/scratch/run.log", "", "Write", map[string]any{"content": tell + "\n"}, false, ""},
		{"allow: a slop.skip path", "pkg/testdata/doc.md", "", "Write", map[string]any{"content": tell + "\n"}, false, ""},
		{"allow: kit.yml is skipped", "kit.yml", "", "Write", map[string]any{"content": "# " + tell + "\nx: 1\n"}, false, ""},
		{"deny: a tell in a Go comment", "x.go", "", "Write", map[string]any{"content": "package x\n\n// " + tell + "\nfunc f() {}\n"}, true, ""},
		{"allow: a tell in a Go string literal", "x.go", "", "Write", map[string]any{"content": "package x\n\nvar s = \"" + tell + "\"\n"}, false, ""},
		{"deny: a tier 2 tell", "doc.md", "", "Write", map[string]any{"content": "It serves as the entry point.\n"}, true, "serves as the"},
		{"allow: a dense tier 3 passage is only logged", "doc.md", "", "Write", map[string]any{"content": dense + "\n"}, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			path := filepath.Join(testutil.Physical(t, t.TempDir()), tc.file)
			if tc.on != "" {
				testutil.Write(t, path, tc.on)
			}
			r := k.run("guard-slop", slopCall(tc.tool, path, tc.input))
			if tc.deny {
				r.Denied(t)
				r.Has(t, "slop-patterns")
			} else {
				r.Allowed(t)
			}
			if tc.has != "" {
				r.Has(t, tc.has)
			}
		})
	}
}

func TestGuardSlopLogs(t *testing.T) {
	t.Parallel()
	t.Run("a dense tier 3 passage logs its rule, path, line and match", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		path := filepath.Join(t.TempDir(), "doc.md")
		k.run("guard-slop", slopCall("Write", path, map[string]any{"content": "# Doc\n\n" + dense + "\n"})).Allowed(t)
		log := read(t, k.paths.LogFile(kitlog.Guards))
		for _, want := range []string{`"slop-vocabulary"`, `"line":"3"`, `"match":"robust"`} {
			if !strings.Contains(log, want) {
				t.Errorf("guards log lacks %s:\n%s", want, log)
			}
		}
	})
	t.Run("a blocked tell logs its rule and match too", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		path := filepath.Join(t.TempDir(), "doc.md")
		k.run("guard-slop", slopCall("Write", path, map[string]any{"content": "It serves as the entry point.\n"})).Denied(t)
		log := read(t, k.paths.LogFile(kitlog.Guards))
		for _, want := range []string{`"slop-copula"`, `"tier":"2"`, `"match":"serves as the"`} {
			if !strings.Contains(log, want) {
				t.Errorf("guards log lacks %s:\n%s", want, log)
			}
		}
	})
	t.Run("an allow marker logs its reason", func(t *testing.T) {
		t.Parallel()
		k := newSandbox(t)
		path := filepath.Join(t.TempDir(), "doc.md")
		k.run("guard-slop", slopCall("Write", path, map[string]any{"content": "unslop: allow quoting the phrase\n" + tell + "\n"})).Allowed(t)
		if log := read(t, k.paths.LogFile(kitlog.Guards)); !strings.Contains(log, "quoting the phrase") {
			t.Errorf("guards log lacks the reason:\n%s", log)
		}
	})
}

func TestGuardSlopHonorsDisabledRules(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "doc.md")
	call := slopCall("Write", path, map[string]any{"content": tell + "\n"})
	for _, tc := range []struct {
		name, overlay string
		deny          bool
	}{
		{"the tell's own slug turns it off", "disabled_rules: [slop-hope-helps]\n", false},
		{"slop turns the guard off", "disabled_rules: [slop]\n", false},
		{"another tell's slug leaves it on", "disabled_rules: [slop-copula]\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newSandbox(t)
			k.overlay(tc.overlay)
			r := k.run("guard-slop", call)
			if tc.deny {
				r.Denied(t)
			} else {
				r.Allowed(t)
			}
		})
	}
}

func TestGuardDispatchRunsGuardSlop(t *testing.T) {
	t.Parallel()
	k := newSandbox(t)
	path := filepath.Join(t.TempDir(), "doc.md")
	k.run("guard-dispatch", slopCall("Write", path, map[string]any{"content": tell + "\n"})).Denied(t)
}
