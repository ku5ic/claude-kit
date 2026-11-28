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

func TestFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Put(t, root, "go/stop.go", "a\n")
	text := "`go/stop.go:1`, go/stop.go:3-4, README.md:2, vendor/x.go:1, stop.go:9, https://example.com:443/x"
	want := []string{filepath.Join(root, "go/stop.go"), filepath.Join(root, "README.md")}
	testutil.Put(t, root, "README.md", "x\n")
	if got := Files(root, text); !slices.Equal(got, want) {
		t.Errorf("Files = %q, want %q", got, want)
	}
}

// A citation on a line naming a commit points at that revision, which the
// working tree can't check.
func TestBrokenSkipsAnotherRevision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Put(t, root, "go/stop.go", "a\n")
	for _, text := range []string{
		"`go/stop.go:9` at 909e23e",
		"go/stop.go:9 (commit b71e789, new-side lines)",
		"go/stop.go:9 on HEAD (5bc6e0e)",
	} {
		if got := Broken(root, text); got != nil {
			t.Errorf("Broken(%q) = %v, want none", text, got)
		}
	}
	if got := Broken(root, "go/stop.go:9 in deadbeef and 1234567"); len(got) != 1 {
		t.Errorf("a hex word or a plain number isn't a commit: got %v", got)
	}
}

// A path short of its leading directories resolves to the one tracked file
// it ends, checked like any other.
func TestBrokenResolvesAShortenedPath(t *testing.T) {
	t.Parallel()
	root := testutil.Physical(t, t.TempDir())
	testutil.Put(t, root, "hooks/hooks.json", "{}\n")
	testutil.Put(t, root, "go/internal/hooks/stop.go", "a\nb\nc\n")
	testutil.Git(t, root, "init", "-q")
	testutil.Git(t, root, "add", ".")
	for text, want := range map[string][]Problem{
		"hooks/stop.go:2":  nil,
		"hooks/stop.go:9":  {{"hooks/stop.go:9", "the file has 3 lines"}},
		"hooks/start.go:1": {{"hooks/start.go:1", "no such file"}},
	} {
		if got := Broken(root, text); !slices.Equal(got, want) {
			t.Errorf("Broken(%q) = %v, want %v", text, got, want)
		}
	}
}

// A commit named after one citation leaves the others in its sentence
// checked.
func TestBrokenChecksTheCitationsBesideARevision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Put(t, root, "go/stop.go", "a\n")
	text := "See go/stop.go:9 at 909e23e, and go/stop.go:7."
	if got := Broken(root, text); !slices.Equal(got, []Problem{{"go/stop.go:7", "the file has 1 lines"}}) {
		t.Errorf("Broken(%q) = %v", text, got)
	}
}
