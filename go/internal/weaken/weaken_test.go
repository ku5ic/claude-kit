package weaken

import (
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

var checks = config.WeakenedChecks{
	Skip:         []string{"*.md", "*/testdata/*"},
	TestFiles:    []string{"*_test.go", "*.test.ts", "*/test_*.py"},
	Skips:        []string{`\bt\.Skip(Now|f)?\(`, `\b(it|test|describe)\.(skip|only)\(`, `@pytest\.mark\.(skip|skipif|xfail)\b`},
	Suppressions: []string{`//\s*nolint\b`, `eslint-disable`, `@ts-(ignore|expect-error)\b`, `#\s*noqa\b`},
	Assertions:   []string{`\bt\.(Error|Errorf|Fatal|Fatalf)\(`, `\bexpect\(`, `^\s*assert\b`},
}

func lines(l ...string) string { return strings.Join(l, "\n") }

func TestCheck(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, before, after string
		hits                      []Hit
		lost                      int
	}{
		{"an added skip in a test", "pkg/a_test.go",
			lines("func TestA(t *testing.T) {", "}"),
			lines("func TestA(t *testing.T) {", `	t.Skip("flaky")`, "}"),
			[]Hit{{Line: 2, Text: `t.Skip("flaky")`, Kind: "skip"}}, 0},
		{"a skip with kit: allow above it", "pkg/a_test.go",
			lines("func TestA(t *testing.T) {", "}"),
			lines("func TestA(t *testing.T) {", "	// kit: allow needs a network the CI box lacks", `	t.Skip("no network")`, "}"),
			[]Hit{{Line: 3, Text: `t.Skip("no network")`, Kind: "skip", Reason: "needs a network the CI box lacks"}}, 0},
		{"kit: allow with no reason is no marker", "pkg/a_test.go",
			"",
			lines("// kit: allow", `t.Skip("x")`),
			[]Hit{{Line: 2, Text: `t.Skip("x")`, Kind: "skip"}}, 0},
		{"a skip that was already there", "pkg/a_test.go",
			lines(`t.Skip("old")`, "x := 1"),
			lines(`t.Skip("old")`, "x := 2"),
			nil, 0},
		{"a skip outside a test file is not a test skip", "pkg/a.go",
			"", `t.Skip("x")`,
			nil, 0},
		{"an added suppression in any code file", "pkg/a.go",
			"err := f()",
			lines("err := f() //nolint:errcheck"),
			[]Hit{{Line: 1, Text: "err := f() //nolint:errcheck", Kind: "suppression"}}, 0},
		{"a suppression with a reason on the same line", "web/a.ts",
			"",
			"// @ts-expect-error upstream type is wrong, kit: allow fixed in v3",
			[]Hit{{Line: 1, Text: "// @ts-expect-error upstream type is wrong, kit: allow fixed in v3", Kind: "suppression", Reason: "fixed in v3"}}, 0},
		{"a focused test", "web/a.test.ts",
			"it('works', () => {})",
			"it.only('works', () => {})",
			[]Hit{{Line: 1, Text: "it.only('works', () => {})", Kind: "skip"}}, 0},
		{"a pytest mark", "app/test_views.py",
			"def test_a():",
			lines("@pytest.mark.skip", "def test_a():"),
			[]Hit{{Line: 1, Text: "@pytest.mark.skip", Kind: "skip"}}, 0},
		{"deleted assertions", "pkg/a_test.go",
			lines(`if got != 1 {`, `	t.Errorf("got %d", got)`, `}`, `if err != nil {`, `	t.Fatal(err)`, `}`),
			lines(`_ = got`),
			nil, 2},
		{"a rewritten assertion loses nothing", "web/a.test.ts",
			"expect(a).toBe(1)",
			"expect(a).toEqual(1)",
			nil, 0},
		{"more assertions loses nothing", "app/test_a.py",
			"assert a",
			lines("assert a", "assert b"),
			nil, 0},
		{"assertions outside a test file aren't counted", "pkg/a.go",
			`t.Errorf("x")`, "",
			nil, 0},
		{"a skipped path", "docs/testing.md",
			"", "Use t.Skip( //nolint",
			nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Check(checks, tc.path, tc.before, tc.after)
			if len(got.Hits) != len(tc.hits) {
				t.Fatalf("hits = %+v, want %+v", got.Hits, tc.hits)
			}
			for i := range tc.hits {
				if got.Hits[i] != tc.hits[i] {
					t.Errorf("hit %d = %+v, want %+v", i, got.Hits[i], tc.hits[i])
				}
			}
			if got.Lost != tc.lost {
				t.Errorf("lost = %d, want %d", got.Lost, tc.lost)
			}
		})
	}
}
