package stats

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKeptNeedsMostOfTheClaimsWords(t *testing.T) {
	t.Parallel()
	claim := "The cache is written by Foo in go/x.go"
	cases := map[string]bool{
		"The cache is written by Foo in go/x.go:12.": true,
		"Foo writes the cache, in go/x.go.":          true,
		"The cache lives in the session directory.":  false,
		"": false,
	}
	for text, want := range cases {
		if got := Kept(claim, []string{text}); got != want {
			t.Errorf("Kept(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestReportCountsRulesAndClaimsSinceADate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	guards := filepath.Join(dir, "guards.jsonl")
	claims := filepath.Join(dir, "claims.jsonl")
	write := func(path string, lines ...string) {
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(guards,
		`{"ts":"2026-11-01T00:00:00Z","event":"block","session_id":"old","rule":"weaken"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"block","session_id":"a","rule":"sensitive-read"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"shadow","session_id":"b","rule":"sensitive-read"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"ask","session_id":"a","rule":"rm-recursive"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"ask","session_id":"b","rule":"rm-recursive"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"ask","session_id":"a"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"claims","session_id":"a","cost_usd":"0.0500"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"critic-skip","session_id":"c","rule":"plan-critic","reason":"renames one flag"}`,
		`{"ts":"2026-11-01T00:00:00Z","event":"auto-denied","session_id":"old","tool":"Bash"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"auto-denied","session_id":"a","tool":"Bash"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"auto-denied","session_id":"b","tool":"Write"}`,
		`not json`)
	write(claims,
		`{"ts":"2026-11-15T00:00:00Z","event":"claim","source":"agent","model":"sonnet","outcome":"dropped"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"claim","source":"agent","model":"sonnet","outcome":"kept"}`)
	var out strings.Builder
	if err := Report(guards, claims, Options{Since: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)}, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"rule            blocks  asks  shadow  disabled  skipped  sessions", "plan-critic     0       0     0       0         1        1", "rm-recursive    0       2     0       0         0        2", "sensitive-read  1       0     1       0         0        2", "agent   sonnet  2        1        1", "hook runs  1  $0.05", "auto-mode denials  2"} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\nBash") {
		t.Errorf("a denial is counted as a rule:\n%s", got)
	}
	if strings.Contains(got, "\n  ") {
		t.Errorf("an ask with no rule is counted under an empty rule:\n%s", got)
	}
	out.Reset()
	if err := Report(guards, claims, Options{Rule: "weaken"}, &out); err != nil || strings.Contains(out.String(), "denials") {
		t.Errorf("--rule shows denials, which have no rule (%v):\n%s", err, out.String())
	}
	if strings.Contains(got, "weaken") {
		t.Errorf("report counts a line before Since:\n%s", got)
	}
}

func TestRunUnusedListsTheRulesWithNoEvent(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	all := []string{"citations", "claims", "plan-critic", "sensitive-read", "size", "weaken"}
	dir := t.TempDir()
	guards := strings.Join([]string{
		`{"ts":"2026-09-01T00:00:00Z","event":"block","session_id":"old","rule":"weaken"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"block","session_id":"a","rule":"sensitive-read"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"shadow","session_id":"b","rule":"citations"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"disabled","session_id":"b","rule":"claims"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"critic-skip","session_id":"c","rule":"plan-critic"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"fail-open","session_id":"c"}`,
		`not json`}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "guards.jsonl"), []byte(guards), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		dir  string
		args []string
		want string
	}{
		{"a rule that fired, shadowed, was disabled, or was skipped isn't listed", dir, []string{"--unused"}, "no event in 30 days\nsize\nweaken\n"},
		{"--since widens the window", dir, []string{"--unused", "--since", "365"}, "no event in 365 days\nsize\n"},
		{"flags in the other order", dir, []string{"--since", "365", "--unused"}, "no event in 365 days\nsize\n"},
		{"--rule narrows it", dir, []string{"--unused", "--rule", "size"}, "no event in 30 days\nsize\n"},
		{"--rule on one that fired leaves the header", dir, []string{"--unused", "--rule", "sensitive-read"}, "no event in 30 days\n"},
		{"no log lists every rule", t.TempDir(), []string{"--unused"}, "no event in 30 days\n" + strings.Join(all, "\n") + "\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var out, errs strings.Builder
			if code := Run(c.dir, all, now, c.args, &out, &errs); code != 0 || out.String() != c.want {
				t.Errorf("exit %d, stderr %q, got:\n%s\nwant:\n%s", code, errs.String(), out.String(), c.want)
			}
		})
	}
	t.Run("a bad --since still fails", func(t *testing.T) {
		t.Parallel()
		var out, errs strings.Builder
		if code := Run(dir, all, now, []string{"--unused", "--since", "x"}, &out, &errs); code != 2 || out.String() != "" {
			t.Errorf("exit %d, stdout %q", code, out.String())
		}
	})
}
