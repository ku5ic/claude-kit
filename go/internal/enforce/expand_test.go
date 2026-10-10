package enforce

import (
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/gapfill"
)

func TestSegmentKindTakesTheClosestSegment(t *testing.T) {
	lint := gapfill.Segment{Text: "eslint .", Role: "check", Kind: "lint"}
	fix := gapfill.Segment{Text: "eslint . --fix", Role: "fixer"}
	wrapped := gapfill.Segment{Text: "dotenv -- vitest run", Role: "check", Kind: "test"}
	for _, tc := range []struct {
		name, stmt string
		segments   []gapfill.Segment
		kind       string
		isCheck    bool
	}{
		{"an exact segment wins over one it contains", "eslint . --fix", []gapfill.Segment{lint, fix}, "", false},
		{"an exact segment wins over one containing it", "eslint .", []gapfill.Segment{fix, lint}, "lint", true},
		{"a statement inside a segment takes it", "vitest run", []gapfill.Segment{wrapped}, "test", true},
		{"a segment inside a statement takes it", "FOO=1 eslint .", []gapfill.Segment{lint}, "lint", true},
		{"no segment holds the statement", "tsc --noEmit", []gapfill.Segment{lint, fix}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kind, isCheck := segmentKind(gapfill.Verdict{Role: "check", Segments: tc.segments}, tc.stmt, 2)
			if kind != tc.kind || isCheck != tc.isCheck {
				t.Errorf("segmentKind(%q) = %q, %v; want %q, %v", tc.stmt, kind, isCheck, tc.kind, tc.isCheck)
			}
		})
	}
}
