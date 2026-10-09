package tools

import (
	"slices"
	"testing"
)

// Outputs captured from the real tools (eslint 10, stylelint 17, ruff 0.15,
// rubocop 1.89, shellcheck 0.11).
func TestFindingsParse(t *testing.T) {
	cases := []struct {
		tool, out string
		want      []Finding
	}{
		{"eslint", "\n/r/src/a.ts\n  0:0  error  Parsing error: nope\nThe file was not found\n  3:7  warning  Unexpected any  no-explicit-any\n\n\x1b[31m✖ 2 problems\x1b[0m\n", []Finding{
			{"/r/src/a.ts", 0, "/r/src/a.ts:0:0  error  Parsing error: nope"},
			{"/r/src/a.ts", 3, "/r/src/a.ts:3:7  warning  Unexpected any  no-explicit-any"},
		}},
		{"stylelint", "/r/a.css:1:18: Unknown property \"colr\" (property-no-unknown) [error]\n\n1 problem (1 error, 0 warnings)\n", []Finding{
			{"/r/a.css", 1, "/r/a.css:1:18: Unknown property \"colr\" (property-no-unknown) [error]"},
		}},
		{"ruff", "zz.py:2:5: F821 Undefined name `a`\nFound 1 error.\n", []Finding{{"zz.py", 2, "zz.py:2:5: F821 Undefined name `a`"}}},
		{"rubocop", "Please also note that you can opt-in:\n  AllCops:\n/r/zz.rb:1:7: C: [Correctable] Layout/SpaceInsideParens: Space inside parentheses detected.\n", []Finding{
			{"/r/zz.rb", 1, "/r/zz.rb:1:7: C: [Correctable] Layout/SpaceInsideParens: Space inside parentheses detected."},
		}},
		{"shellcheck", "a.sh:2:6: note: Double quote to prevent globbing. [SC2086]\n", []Finding{{"a.sh", 2, "a.sh:2:6: note: Double quote to prevent globbing. [SC2086]"}}},
		{"biome", "src/a.ts:1:1 lint/style/useConst  FIXABLE  ━━━\n  × Use const\nsrc/b.ts format ━━━━━━\n", []Finding{
			{"src/a.ts", 1, "src/a.ts:1:1 lint/style/useConst  FIXABLE  ━━━"},
			{"src/b.ts", 0, "src/b.ts format ━━━━━━"},
		}},
		{"golangci-lint", "pkg/c.go:3:2: ineffectual assignment to x (ineffassign)\n\tx := 1\n1 issues:\n", []Finding{{"pkg/c.go", 3, "pkg/c.go:3:2: ineffectual assignment to x (ineffassign)"}}},
	}
	for _, c := range cases {
		got, ok := adapter(t, c.tool).Findings.Parse(c.out)
		if !ok || !slices.Equal(got, c.want) {
			t.Errorf("%s: %v %+v, want %+v", c.tool, ok, got, c.want)
		}
	}
	if _, ok := adapter(t, "biome").Findings.Parse("a.ts:1:1 lint/x\nDiagnostics not shown: 12.\n"); ok {
		t.Error("biome: a truncated report must not be trusted")
	}
}
