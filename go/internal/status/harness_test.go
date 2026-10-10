package status_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/status"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestMain(m *testing.M) {
	// Cases build their own repos; git must see only those, never the
	// developer's config or a GIT_DIR inherited from a git hook.
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); strings.HasPrefix(name, "GIT_") {
			os.Unsetenv(name)
		}
	}
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Unsetenv("STATUSLINE_CACHE_TTL")
	os.Exit(m.Run())
}

// sandbox is one case's Claude config dir, where the git segment is
// cached, and a git repository on main called "repo".
type sandbox struct {
	t          *testing.T
	home, repo string
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	tmp := t.TempDir()
	s := &sandbox{t: t, home: filepath.Join(tmp, "claude"), repo: filepath.Join(tmp, "repo")}
	testutil.Git(t, tmp, "init", "-q", "-b", "main", "repo")
	testutil.Git(t, s.repo, "config", "user.email", "test@example.com")
	testutil.Git(t, s.repo, "config", "user.name", "Test")
	return s
}

// render is Statusline's output for payload p.
func (s *sandbox) render(p map[string]any) string {
	s.t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		s.t.Fatal(err)
	}
	var out bytes.Buffer
	status.Statusline(bytes.NewReader(raw), &out, s.home)
	return out.String()
}

// change commits name holding body, then appends extra unstaged: one more
// line added and one deleted in the git segment.
func (s *sandbox) change(name, body, extra string) {
	s.t.Helper()
	testutil.Put(s.t, s.repo, name, body)
	testutil.Git(s.t, s.repo, "add", name)
	testutil.Git(s.t, s.repo, "commit", "-qm", name)
	if err := fsx.Append(filepath.Join(s.repo, name), []byte(extra)); err != nil {
		s.t.Fatal(err)
	}
}

// cacheFile is where the git segment of session "s1" is cached.
func (s *sandbox) cacheFile() string {
	return filepath.Join(s.home, "cache", "statusline", "git-s1")
}

// payload is a statusLine payload for session "s1" in dir, at ctx percent
// of the context window, with a 1s duration.
func payload(dir string, ctx float64) map[string]any {
	return map[string]any{
		"model":          map[string]any{"display_name": "Opus"},
		"workspace":      map[string]any{"current_dir": dir},
		"session_id":     "s1",
		"context_window": map[string]any{"used_percentage": ctx},
		"cost":           map[string]any{"total_cost_usd": 1, "total_duration_ms": 1000},
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// plain strips color codes and trailing newlines, so a case asserts on
// segment text without hardcoding what is colored.
func plain(out string) string {
	return strings.TrimRight(ansi.ReplaceAllString(out, ""), "\n")
}

func firstLine(out string) string {
	line, _, _ := strings.Cut(plain(out), "\n")
	return line
}

func has(t *testing.T, out string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(out, s) {
			t.Errorf("output lacks %q:\n%q", s, out)
		}
	}
}

func lacks(t *testing.T, out string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if strings.Contains(out, s) {
			t.Errorf("output has %q:\n%q", s, out)
		}
	}
}
