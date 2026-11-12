package cite

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestBroken(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Put(t, root, "go/stop.go", "a\nb\nc\n")
	testutil.Put(t, root, "README.md", "one line, no newline")
	for name, tc := range map[string]struct {
		text string
		want []Problem
	}{
		"a line in range":          {"see `go/stop.go:3`", nil},
		"a range in range":         {"go/stop.go:1-3 holds it", nil},
		"a line past the end":      {"see `go/stop.go:4`", []Problem{{"go/stop.go:4", "the file has 3 lines"}}},
		"a range past the end":     {"(go/stop.go:2-9)", []Problem{{"go/stop.go:2-9", "the file has 3 lines"}}},
		"a backwards range":        {"go/stop.go:3-1", []Problem{{"go/stop.go:3-1", "the range ends before it starts"}}},
		"line zero":                {"go/stop.go:0", []Problem{{"go/stop.go:0", "lines start at 1"}}},
		"a missing file":           {"go/start.go:1", []Problem{{"go/start.go:1", "no such file"}}},
		"a root file":              {"README.md:1 and README.md:2", []Problem{{"README.md:2", "the file has 1 lines"}}},
		"each citation once":       {"go/stop.go:9, again go/stop.go:9", []Problem{{"go/stop.go:9", "the file has 3 lines"}}},
		"an absolute path":         {filepath.Join(root, "go/stop.go") + ":7", []Problem{{filepath.Join(root, "go/stop.go") + ":7", "the file has 3 lines"}}},
		"another tree":             {"vendor/x.go:99 and /etc/hosts.txt:1", nil},
		"a bare name not at root":  {"stop.go:99", nil},
		"a URL host and port":      {"https://example.com:443/x", nil},
		"a time of day":            {"at 12:30", nil},
		"a parent-relative escape": {"../other/x.go:1", nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := Broken(root, tc.text); !slices.Equal(got, tc.want) {
				t.Errorf("Broken(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}
