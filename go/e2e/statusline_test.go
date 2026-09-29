package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// kit statusline reads the statusLine JSON payload from stdin and renders a
// two-row status. Each test fakes $HOME so the git-status cache never lands
// in the real ~/.claude/cache, and builds a throwaway git repo for the
// branch/counts assertions so real repo state never leaks in either.
func TestStatusline(t *testing.T) {
	const (
		green  = "\033[32m"
		yellow = "\033[33m"
		red    = "\033[31m"
	)
	setup := func(t *testing.T) (*Kit, string) {
		k := New(t)
		repo := filepath.Join(Physical(t, t.TempDir()), "repo")
		Mkdir(t, repo)
		k.Git(repo, "init", "-q", "-b", "main")
		k.Git(repo, "config", "user.email", "test@example.com")
		k.Git(repo, "config", "user.name", "Test")
		return k, repo
	}
	render := func(k *Kit, p map[string]any) Result {
		raw, _ := json.Marshal(p) // maps of strings and numbers always marshal
		return k.Run(string(raw), "statusline")
	}
	// A dirty repo: a.txt committed then modified, so the git segment reads +1 ~1.
	dirty := func(t *testing.T, k *Kit, repo string) {
		Write(t, filepath.Join(repo, "a.txt"), "one")
		k.Git(repo, "add", "a.txt")
		k.Git(repo, "commit", "-qm", "init")
		statuslineAppend(t, filepath.Join(repo, "a.txt"), "x")
	}
	// A second commit plus another unstaged change: a fresh read is +2 ~2.
	dirtier := func(t *testing.T, k *Kit, repo string) {
		Write(t, filepath.Join(repo, "b.txt"), "two")
		k.Git(repo, "add", "b.txt")
		k.Git(repo, "commit", "-qm", "second")
		statuslineAppend(t, filepath.Join(repo, "b.txt"), "y")
	}
	// Sets file's mtime to now minus secs, so TTL-boundary tests hit the exact
	// edge deterministically instead of sleeping past it. Truncated to the
	// second, as touch -t does.
	backdate := func(t *testing.T, path string, secs int) {
		ts := time.Now().Add(-time.Duration(secs) * time.Second).Truncate(time.Second)
		if err := os.Chtimes(path, ts, ts); err != nil {
			t.Fatal(err)
		}
	}
	cacheFile := func(k *Kit, session string) string {
		return filepath.Join(k.Claude, "cache", "statusline", "git-"+session)
	}

	t.Run("renders model name, dir basename, and context percentage", func(t *testing.T) {
		k, repo := setup(t)
		r := render(k, statuslinePayload(repo, "t1", 50))
		r.Want(t, 0)
		r.Has(t, "Opus", filepath.Base(repo), "50%")
	})

	for _, tc := range []struct {
		name  string
		ctx   float64
		color string
	}{
		{"context bar renders green below the yellow threshold", 50, green},
		{"context bar renders yellow at the yellow threshold", 75, yellow},
		{"context bar renders red at the red threshold", 95, red},
		{"context bar stays green just below the yellow threshold", 69, green},
		{"context bar turns yellow exactly at the yellow threshold", 70, yellow},
		{"context bar stays yellow just below the red threshold", 89, yellow},
		{"context bar turns red exactly at the red threshold", 90, red},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, repo := setup(t)
			render(k, statuslinePayload(repo, "tctx", tc.ctx)).Has(t, tc.color)
		})
	}

	t.Run("negative context percentage clamps to zero and renders green", func(t *testing.T) {
		k, repo := setup(t)
		render(k, statuslinePayload(repo, "t4e", -15)).Has(t, "0%", green)
	})

	t.Run("context percentage above 100 clamps to 100 and renders red", func(t *testing.T) {
		k, repo := setup(t)
		render(k, statuslinePayload(repo, "t4f", 150)).Has(t, "100%", red)
	})

	// make_payload omits agent keys because a plain interactive session has
	// none; only a main thread running as a named agent gets them.
	t.Run("agent name renders beside the model when present", func(t *testing.T) {
		k, repo := setup(t)
		p := statuslinePayload(repo, "t4g", 50)
		p["agent"] = map[string]any{"name": "scout"}
		r := render(k, p)
		r.Want(t, 0)
		statuslinePrefix(t, r, "Opus (scout)  "+filepath.Base(repo))
	})

	t.Run("agent name falls back to agent_type when the agent key is absent", func(t *testing.T) {
		k, repo := setup(t)
		p := statuslinePayload(repo, "t4h", 50)
		p["agent_type"] = "reviewer"
		statuslinePrefix(t, render(k, p), "Opus (reviewer)")
	})

	t.Run("agent key wins over agent_type when both are present", func(t *testing.T) {
		k, repo := setup(t)
		p := statuslinePayload(repo, "t4i", 50)
		p["agent"] = map[string]any{"name": "scout"}
		p["agent_type"] = "reviewer"
		r := render(k, p)
		statuslinePrefix(t, r, "Opus (scout)")
		r.Lacks(t, "reviewer")
	})

	t.Run("agent segment is omitted for a plain interactive session", func(t *testing.T) {
		k, repo := setup(t)
		r := render(k, statuslinePayload(repo, "t4j", 50))
		statuslinePrefix(t, r, "Opus  "+filepath.Base(repo))
		r.Lacks(t, "(")
	})

	t.Run("effort segment renders when present", func(t *testing.T) {
		k, repo := setup(t)
		p := statuslinePayload(repo, "t5", 50)
		p["effort"] = map[string]any{"level": "high"}
		render(k, p).Has(t, "effort:high")
	})

	t.Run("effort segment is omitted when absent", func(t *testing.T) {
		k, repo := setup(t)
		render(k, statuslinePayload(repo, "t6", 50)).Lacks(t, "effort:")
	})

	t.Run("5h segment renders when present", func(t *testing.T) {
		k, repo := setup(t)
		p := statuslinePayload(repo, "t7", 50)
		p["rate_limits"] = map[string]any{"five_hour": map[string]any{"used_percentage": 12.7}}
		render(k, p).Has(t, "5h:12%")
	})

	t.Run("5h segment is omitted when absent", func(t *testing.T) {
		k, repo := setup(t)
		render(k, statuslinePayload(repo, "t8", 50)).Lacks(t, "5h:")
	})

	withDuration := func(repo, session string, ms int) map[string]any {
		p := statuslinePayload(repo, session, 50)
		p["cost"] = map[string]any{"total_cost_usd": 1, "total_duration_ms": ms}
		return p
	}

	t.Run("duration under a minute renders as seconds only", func(t *testing.T) {
		k, repo := setup(t)
		out := statuslinePlain(render(k, withDuration(repo, "tdur1", 46000)).Output)
		if !strings.Contains(out, " 46s") {
			t.Errorf("want \" 46s\" in:\n%s", out)
		}
	})

	t.Run("duration under an hour renders as minutes only, seconds dropped", func(t *testing.T) {
		k, repo := setup(t)
		out := statuslinePlain(render(k, withDuration(repo, "tdur2", 125000)).Output)
		if !strings.HasSuffix(out, " 2m") {
			t.Errorf("want suffix \" 2m\" in:\n%s", out)
		}
	})

	t.Run("duration of an hour or more rolls into space-separated hours and minutes", func(t *testing.T) {
		k, repo := setup(t)
		out := statuslinePlain(render(k, withDuration(repo, "tdur3", 16546000)).Output)
		if !strings.HasSuffix(out, " 4h 35m") {
			t.Errorf("want suffix \" 4h 35m\" in:\n%s", out)
		}
	})

	t.Run("git segment shows branch and additions/deletions across staged and unstaged changes", func(t *testing.T) {
		k, repo := setup(t)
		Write(t, filepath.Join(repo, "a.txt"), "one")
		k.Git(repo, "add", "a.txt")
		k.Git(repo, "commit", "-qm", "init")
		Write(t, filepath.Join(repo, "b.txt"), "two")
		k.Git(repo, "add", "b.txt")
		statuslineAppend(t, filepath.Join(repo, "a.txt"), "changed")
		statuslineContains(t, render(k, statuslinePayload(repo, "t9", 50)), "main +2 ~1")
	})

	t.Run("git segment is omitted outside a git repo", func(t *testing.T) {
		k, _ := setup(t)
		plain := filepath.Join(Physical(t, t.TempDir()), "plain")
		Mkdir(t, plain)
		out := statuslinePlain(render(k, statuslinePayload(plain, "t10", 50)).Output)
		if first, _, _ := strings.Cut(out, "\n"); first != "Opus  plain" {
			t.Errorf("first line %q, want %q", first, "Opus  plain")
		}
	})

	// The default CACHE_TTL is 1 second, which is far too narrow to assert cache
	// REUSE against: the git commits between two renders can easily take longer
	// than the window, so the second call would legitimately refresh and the
	// test would flake. Reuse tests therefore set a wide STATUSLINE_CACHE_TTL
	// and make no assumption about elapsed wall time; expiry tests keep the real
	// 1s default and backdate the cache mtime so the boundary is hit
	// deterministically, with no sleep.

	t.Run("git status cache is reused within the TTL", func(t *testing.T) {
		k, repo := setup(t)
		k.Setenv("STATUSLINE_CACHE_TTL", "3600")
		dirty(t, k, repo)
		statuslineContains(t, render(k, statuslinePayload(repo, "tcache1", 50)), "+1 ~1")
		dirtier(t, k, repo)
		statuslineContains(t, render(k, statuslinePayload(repo, "tcache1", 50)), "+1 ~1")
	})

	t.Run("git status cache regenerates after the TTL expires", func(t *testing.T) {
		k, repo := setup(t)
		dirty(t, k, repo)
		statuslineContains(t, render(k, statuslinePayload(repo, "tcache2", 50)), "+1 ~1")
		dirtier(t, k, repo)
		time.Sleep(2 * time.Second)
		statuslineContains(t, render(k, statuslinePayload(repo, "tcache2", 50)), "+2 ~2")
	})

	t.Run("git status cache regenerates exactly at the TTL boundary (age == CACHE_TTL)", func(t *testing.T) {
		k, repo := setup(t)
		dirty(t, k, repo)
		statuslineContains(t, render(k, statuslinePayload(repo, "tboundary1", 50)), "+1 ~1")
		dirtier(t, k, repo)
		// Default CACHE_TTL is 1; backdating the cache file's mtime by exactly 1
		// second hits the `>=` boundary deterministically, no sleep.
		backdate(t, cacheFile(k, "tboundary1"), 1)
		statuslineContains(t, render(k, statuslinePayload(repo, "tboundary1", 50)), "+2 ~2")
	})

	t.Run("git status cache is reused comfortably under the TTL boundary", func(t *testing.T) {
		// A fresh (not backdated) cache file under a wide TTL - the only way to
		// assert the reuse side without the elapsed-time race described above.
		k, repo := setup(t)
		k.Setenv("STATUSLINE_CACHE_TTL", "3600")
		dirty(t, k, repo)
		statuslineContains(t, render(k, statuslinePayload(repo, "tboundary2", 50)), "+1 ~1")
		dirtier(t, k, repo)
		backdate(t, cacheFile(k, "tboundary2"), 60)
		statuslineContains(t, render(k, statuslinePayload(repo, "tboundary2", 50)), "+1 ~1")
	})

	t.Run("a non-numeric STATUSLINE_CACHE_TTL falls back instead of blanking the line", func(t *testing.T) {
		k, repo := setup(t)
		k.Setenv("STATUSLINE_CACHE_TTL", "notanumber")
		dirty(t, k, repo)
		r := render(k, statuslinePayload(repo, "tbadttl", 50))
		r.Want(t, 0)
		statuslineContains(t, r, "+1 ~1")
	})

	t.Run("renders without jq on PATH", func(t *testing.T) {
		// The status line is Go: an empty PATH still renders. Runs the built
		// binary rather than the shim, which would pick the committed one.
		k, _ := setup(t)
		k.Setenv("PATH", t.TempDir())
		r := k.Run(`{"model":{"display_name":"Opus"}}`, "statusline")
		r.Want(t, 0)
		r.Has(t, "Opus", "0%")
	})

	// Transcript-derived actual/declared model divergence. The payload's
	// model.display_name is always "Opus", so the session model short-name is
	// "opus" throughout. Timestamps compare lexicographically, so their string
	// order must match chronological order.
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
	// A command_permissions attachment, the recorded signal for a skill's
	// frontmatter model: override at invocation time.
	declared := func(model, ts string) any {
		return map[string]any{"type": "attachment", "attachment": map[string]any{"type": "command_permissions", "model": model}, "timestamp": ts}
	}
	withTranscript := func(t *testing.T, repo, session string, lines ...any) map[string]any {
		path := filepath.Join(t.TempDir(), session+".jsonl")
		var b strings.Builder
		for _, l := range lines {
			raw, err := json.Marshal(l)
			if err != nil {
				t.Fatal(err)
			}
			b.Write(raw)
			b.WriteByte('\n')
		}
		Write(t, path, b.String())
		p := statuslinePayload(repo, session, 50)
		p["transcript_path"] = path
		return p
	}
	matches := func(t *testing.T, r Result, pattern string) {
		t.Helper()
		if !regexp.MustCompile(pattern).MatchString(r.Output) {
			t.Errorf("output does not match %q:\n%q", pattern, r.Output)
		}
	}

	t.Run("actual model differs from the session model renders the yellow divergence arrow", func(t *testing.T) {
		k, repo := setup(t)
		r := render(k, withTranscript(t, repo, "tact",
			user("hi", "2026-01-01T00:00:01Z"),
			assistant("claude-sonnet-5", "2026-01-01T00:00:02Z", false)))
		matches(t, r, `(?s)\x1b\[33m.*->.*Sonnet`)
	})

	t.Run("actual model differs from the session model renders the yellow divergence arrow (array-shaped message content)", func(t *testing.T) {
		k, repo := setup(t)
		r := render(k, withTranscript(t, repo, "tactarr",
			userArray("hi", "2026-01-01T00:00:01Z"),
			assistant("claude-sonnet-5", "2026-01-01T00:00:02Z", false)))
		matches(t, r, `(?s)\x1b\[33m.*->.*Sonnet`)
	})

	t.Run("a sidechain (subagent) assistant entry is excluded from the actual model", func(t *testing.T) {
		k, repo := setup(t)
		r := render(k, withTranscript(t, repo, "tside",
			user("hi", "2026-01-01T00:00:01Z"),
			assistant("claude-sonnet-5", "2026-01-01T00:00:02Z", false),
			assistant("claude-haiku-4-5", "2026-01-01T00:00:03Z", true)))
		r.Has(t, "Sonnet")
		r.Lacks(t, "Haiku")
	})

	t.Run("a declared override that silently falls back to the session model shows only the declared marker", func(t *testing.T) {
		k, repo := setup(t)
		r := render(k, withTranscript(t, repo, "tdecl1",
			user("hi", "2026-01-01T00:00:01Z"),
			assistant("claude-opus-5", "2026-01-01T00:00:02Z", false),
			declared("claude-sonnet-5", "2026-01-01T00:00:03Z")))
		r.Has(t, red+"!Sonnet")
		r.Lacks(t, "->")
	})

	t.Run("a declared override that diverges from both the session and actual model shows the three-way arrow", func(t *testing.T) {
		k, repo := setup(t)
		r := render(k, withTranscript(t, repo, "tdecl2",
			user("hi", "2026-01-01T00:00:01Z"),
			assistant("claude-haiku-4-5", "2026-01-01T00:00:02Z", false),
			declared("claude-sonnet-5", "2026-01-01T00:00:03Z")))
		matches(t, r, `(?s)Sonnet.*->.*Haiku`)
	})

	t.Run("a declared attachment older than the last user prompt is ignored as stale", func(t *testing.T) {
		k, repo := setup(t)
		r := render(k, withTranscript(t, repo, "tstale",
			declared("claude-sonnet-5", "2026-01-01T00:00:01Z"),
			user("hi", "2026-01-01T00:00:02Z"),
			assistant("claude-opus-5", "2026-01-01T00:00:03Z", false)))
		r.Lacks(t, "!", "->")
	})

	t.Run("a missing transcript file renders the plain row with no divergence segment", func(t *testing.T) {
		k, repo := setup(t)
		p := statuslinePayload(repo, "tmiss", 50)
		p["transcript_path"] = filepath.Join(t.TempDir(), "does-not-exist.jsonl")
		r := render(k, p)
		r.Want(t, 0)
		if first, _, _ := strings.Cut(statuslinePlain(r.Output), "\n"); !strings.HasPrefix(first, "Opus  ") {
			t.Errorf("first line %q lacks prefix %q", first, "Opus  ")
		}
		r.Lacks(t, "!", "->")
	})
}

// statuslinePayload is the bats make_payload: dir, session and context
// percentage, with a 1s duration.
func statuslinePayload(dir, session string, ctx float64) map[string]any {
	return map[string]any{
		"model":          map[string]any{"display_name": "Opus"},
		"workspace":      map[string]any{"current_dir": dir},
		"session_id":     session,
		"context_window": map[string]any{"used_percentage": ctx},
		"cost":           map[string]any{"total_cost_usd": 1, "total_duration_ms": 1000},
	}
}

var statuslineANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// statuslinePlain strips color codes and trailing newlines (as bats' $output
// does), so tests assert on segment text without hardcoding what is colored.
func statuslinePlain(s string) string {
	return strings.TrimRight(statuslineANSI.ReplaceAllString(s, ""), "\n")
}

func statuslinePrefix(t *testing.T, r Result, prefix string) {
	t.Helper()
	if out := statuslinePlain(r.Output); !strings.HasPrefix(out, prefix) {
		t.Errorf("want prefix %q in:\n%s", prefix, out)
	}
}

func statuslineContains(t *testing.T, r Result, sub string) {
	t.Helper()
	if out := statuslinePlain(r.Output); !strings.Contains(out, sub) {
		t.Errorf("want %q in:\n%s", sub, out)
	}
}

func statuslineAppend(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}
