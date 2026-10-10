package status_test

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

const (
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
)

func TestStatuslineContextBar(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		ctx  float64
		has  []string
	}{
		{"context bar renders green below the yellow threshold", 50, []string{green}},
		{"context bar renders yellow at the yellow threshold", 75, []string{yellow}},
		{"context bar renders red at the red threshold", 95, []string{red}},
		{"context bar stays green just below the yellow threshold", 69, []string{green}},
		{"context bar turns yellow exactly at the yellow threshold", 70, []string{yellow}},
		{"context bar stays yellow just below the red threshold", 89, []string{yellow}},
		{"context bar turns red exactly at the red threshold", 90, []string{red}},
		{"negative context percentage clamps to zero and renders green", -15, []string{"0%", green}},
		{"context percentage above 100 clamps to 100 and renders red", 150, []string{"100%", red}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := newSandbox(t)
			has(t, s.render(payload(s.repo, c.ctx)), c.has...)
		})
	}
}

// Each case adds extra to the base payload, then checks the plain
// output's prefix and suffix and the colored output's substrings.
func TestStatuslineSegments(t *testing.T) {
	t.Parallel()
	cost := func(ms int) map[string]any {
		return map[string]any{"cost": map[string]any{"total_cost_usd": 1, "total_duration_ms": ms}}
	}
	for _, c := range []struct {
		name           string
		extra          map[string]any
		prefix, suffix string
		has, lacks     []string
	}{
		{name: "renders model name, dir basename, and context percentage", has: []string{"Opus", "repo", "50%"}},
		// The base payload has no agent keys because a plain interactive
		// session has none; only a main thread running as a named agent gets them.
		{
			name:   "agent name renders beside the model when present",
			extra:  map[string]any{"agent": map[string]any{"name": "scout"}},
			prefix: "Opus (scout)  repo",
		},
		{
			name:   "agent name falls back to agent_type when the agent key is absent",
			extra:  map[string]any{"agent_type": "reviewer"},
			prefix: "Opus (reviewer)",
		},
		{
			name:   "agent key wins over agent_type when both are present",
			extra:  map[string]any{"agent": map[string]any{"name": "scout"}, "agent_type": "reviewer"},
			prefix: "Opus (scout)",
			lacks:  []string{"reviewer"},
		},
		{name: "agent segment is omitted for a plain interactive session", prefix: "Opus  repo", lacks: []string{"("}},
		{
			name:  "effort segment renders when present",
			extra: map[string]any{"effort": map[string]any{"level": "high"}},
			has:   []string{"effort:high"},
		},
		{name: "effort segment is omitted when absent", lacks: []string{"effort:"}},
		{
			name:  "5h segment renders when present",
			extra: map[string]any{"rate_limits": map[string]any{"five_hour": map[string]any{"used_percentage": 12.7}}},
			has:   []string{"5h:12%"},
		},
		{name: "5h segment is omitted when absent", lacks: []string{"5h:"}},
		{name: "duration under a minute renders as seconds only", extra: cost(46000), suffix: " 46s"},
		{name: "duration under an hour renders as minutes only, seconds dropped", extra: cost(125000), suffix: " 2m"},
		{name: "duration of an hour or more rolls into space-separated hours and minutes", extra: cost(16546000), suffix: " 4h 35m"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := newSandbox(t)
			p := payload(s.repo, 50)
			maps.Copy(p, c.extra)
			out := s.render(p)
			if text := plain(out); !strings.HasPrefix(text, c.prefix) || !strings.HasSuffix(text, c.suffix) {
				t.Errorf("want prefix %q and suffix %q in:\n%s", c.prefix, c.suffix, text)
			}
			has(t, out, c.has...)
			lacks(t, out, c.lacks...)
		})
	}
}

func TestStatuslineGitSegment(t *testing.T) {
	t.Parallel()
	t.Run("git segment shows branch and additions/deletions across staged and unstaged changes", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		s.change("a.txt", "one", "changed")
		testutil.Put(t, s.repo, "b.txt", "two")
		testutil.Git(t, s.repo, "add", "b.txt")
		has(t, plain(s.render(payload(s.repo, 50))), "main +2 ~1")
	})

	t.Run("git segment counts untracked files repo-wide, not ignored or binary ones", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		testutil.Put(t, s.repo, ".gitignore", "*.log\n")
		testutil.Git(t, s.repo, "add", ".gitignore")
		testutil.Git(t, s.repo, "commit", "-qm", "init")
		testutil.Put(t, s.repo, "new.txt", "x\ny\nz")
		testutil.Put(t, s.repo, "skip.log", "1\n2\n")
		testutil.Put(t, s.repo, "blob.bin", "a\x00\nb\n")
		// Not read: one file past the byte budget, and a FIFO that would block.
		testutil.Put(t, s.repo, "huge.txt", strings.Repeat("x\n", 5<<20))
		if err := syscall.Mkfifo(filepath.Join(s.repo, "pipe"), 0o644); err != nil {
			t.Fatal(err)
		}
		// A symlink is one line to git, its target, whatever it points at.
		if err := os.Symlink("new.txt", filepath.Join(s.repo, "link")); err != nil {
			t.Fatal(err)
		}
		sub := filepath.Join(s.repo, "sub")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		has(t, plain(s.render(payload(sub, 50))), "main +4")
	})

	t.Run("git segment is omitted outside a git repo", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		dir := filepath.Join(t.TempDir(), "plain")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if got := firstLine(s.render(payload(dir, 50))); got != "Opus  plain" {
			t.Errorf("first line %q, want %q", got, "Opus  plain")
		}
	})
}

// The default TTL of 1s is too narrow to assert cache reuse against: the
// commits between two renders can outlast it. Expiry cases keep the
// default and age the cache file instead of racing the clock; reuse cases,
// in TestStatuslineCacheTTLFromEnv, set a wide TTL.
func TestStatuslineCacheExpiry(t *testing.T) {
	t.Parallel()
	t.Run("git status cache regenerates after the TTL expires", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		s.change("a.txt", "one", "x")
		has(t, plain(s.render(payload(s.repo, 50))), "+1 ~1")
		s.change("b.txt", "two", "y")
		time.Sleep(2 * time.Second)
		has(t, plain(s.render(payload(s.repo, 50))), "+2 ~2")
	})

	t.Run("git status cache regenerates exactly at the TTL boundary (age == CACHE_TTL)", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		s.change("a.txt", "one", "x")
		has(t, plain(s.render(payload(s.repo, 50))), "+1 ~1")
		s.change("b.txt", "two", "y")
		testutil.Age(t, time.Second, s.cacheFile())
		has(t, plain(s.render(payload(s.repo, 50))), "+2 ~2")
	})
}

// Not parallel: each case sets STATUSLINE_CACHE_TTL.
func TestStatuslineCacheTTLFromEnv(t *testing.T) {
	t.Run("git status cache is reused within the TTL", func(t *testing.T) {
		t.Setenv("STATUSLINE_CACHE_TTL", "3600")
		s := newSandbox(t)
		s.change("a.txt", "one", "x")
		has(t, plain(s.render(payload(s.repo, 50))), "+1 ~1")
		s.change("b.txt", "two", "y")
		has(t, plain(s.render(payload(s.repo, 50))), "+1 ~1")
	})

	t.Run("git status cache is reused comfortably under the TTL boundary", func(t *testing.T) {
		t.Setenv("STATUSLINE_CACHE_TTL", "3600")
		s := newSandbox(t)
		s.change("a.txt", "one", "x")
		has(t, plain(s.render(payload(s.repo, 50))), "+1 ~1")
		s.change("b.txt", "two", "y")
		testutil.Age(t, time.Minute, s.cacheFile())
		has(t, plain(s.render(payload(s.repo, 50))), "+1 ~1")
	})

	t.Run("a non-numeric STATUSLINE_CACHE_TTL falls back instead of blanking the line", func(t *testing.T) {
		t.Setenv("STATUSLINE_CACHE_TTL", "notanumber")
		s := newSandbox(t)
		s.change("a.txt", "one", "x")
		has(t, plain(s.render(payload(s.repo, 50))), "+1 ~1")
	})
}

// The payload's model.display_name is always "Opus", so the session
// model's short name is "opus" throughout. Timestamps compare as strings,
// so their string order must match chronological order.
func TestStatuslineModelDivergence(t *testing.T) {
	t.Parallel()
	user := func(text, ts string) any {
		return map[string]any{"type": "user", "isMeta": false, "message": map[string]any{"content": text}, "timestamp": ts}
	}
	// The array-of-content-blocks shape real transcripts overwhelmingly use,
	// exercising the other branch of the since-last-prompt filter.
	userArray := func(text, ts string) any {
		return map[string]any{"type": "user", "isMeta": false, "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}, "timestamp": ts}
	}
	assistant := func(model, ts string, sidechain bool) any {
		return map[string]any{"type": "assistant", "isSidechain": sidechain, "message": map[string]any{"model": model}, "timestamp": ts}
	}
	// A command_permissions attachment records a skill's frontmatter model
	// override at invocation time.
	declared := func(model, ts string) any {
		return map[string]any{"type": "attachment", "attachment": map[string]any{"type": "command_permissions", "model": model}, "timestamp": ts}
	}
	for _, c := range []struct {
		name       string
		lines      []any
		match      string
		has, lacks []string
	}{
		{
			name: "actual model differs from the session model renders the yellow divergence arrow",
			lines: []any{
				user("hi", "2026-01-01T00:00:01Z"),
				assistant("claude-sonnet-5", "2026-01-01T00:00:02Z", false),
			},
			match: `(?s)\x1b\[33m.*->.*Sonnet`,
		},
		{
			name: "actual model differs from the session model renders the yellow divergence arrow (array-shaped message content)",
			lines: []any{
				userArray("hi", "2026-01-01T00:00:01Z"),
				assistant("claude-sonnet-5", "2026-01-01T00:00:02Z", false),
			},
			match: `(?s)\x1b\[33m.*->.*Sonnet`,
		},
		{
			name: "a sidechain (subagent) assistant entry is excluded from the actual model",
			lines: []any{
				user("hi", "2026-01-01T00:00:01Z"),
				assistant("claude-sonnet-5", "2026-01-01T00:00:02Z", false),
				assistant("claude-haiku-4-5", "2026-01-01T00:00:03Z", true),
			},
			has:   []string{"Sonnet"},
			lacks: []string{"Haiku"},
		},
		{
			name: "a declared override that silently falls back to the session model shows only the declared marker",
			lines: []any{
				user("hi", "2026-01-01T00:00:01Z"),
				assistant("claude-opus-5", "2026-01-01T00:00:02Z", false),
				declared("claude-sonnet-5", "2026-01-01T00:00:03Z"),
			},
			has:   []string{red + "!Sonnet"},
			lacks: []string{"->"},
		},
		{
			name: "a declared override that diverges from both the session and actual model shows the three-way arrow",
			lines: []any{
				user("hi", "2026-01-01T00:00:01Z"),
				assistant("claude-haiku-4-5", "2026-01-01T00:00:02Z", false),
				declared("claude-sonnet-5", "2026-01-01T00:00:03Z"),
			},
			match: `(?s)Sonnet.*->.*Haiku`,
		},
		{
			name: "a declared attachment older than the last user prompt is ignored as stale",
			lines: []any{
				declared("claude-sonnet-5", "2026-01-01T00:00:01Z"),
				user("hi", "2026-01-01T00:00:02Z"),
				assistant("claude-opus-5", "2026-01-01T00:00:03Z", false),
			},
			lacks: []string{"!", "->"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := newSandbox(t)
			path := filepath.Join(t.TempDir(), "transcript.jsonl")
			testutil.AppendJSONL(t, path, c.lines...)
			p := payload(s.repo, 50)
			p["transcript_path"] = path
			out := s.render(p)
			if !regexp.MustCompile(c.match).MatchString(out) {
				t.Errorf("output does not match %q:\n%q", c.match, out)
			}
			has(t, out, c.has...)
			lacks(t, out, c.lacks...)
		})
	}

	t.Run("a missing transcript file renders the plain row with no divergence segment", func(t *testing.T) {
		t.Parallel()
		s := newSandbox(t)
		p := payload(s.repo, 50)
		p["transcript_path"] = filepath.Join(t.TempDir(), "does-not-exist.jsonl")
		out := s.render(p)
		if first := firstLine(out); !strings.HasPrefix(first, "Opus  ") {
			t.Errorf("first line %q lacks prefix %q", first, "Opus  ")
		}
		lacks(t, out, "!", "->")
	})
}
