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
		`{"ts":"2026-11-15T00:00:00Z","event":"claims","session_id":"a","cost_usd":"0.0500"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"critic-skip","session_id":"c","rule":"plan-critic","reason":"renames one flag"}`,
		`not json`)
	write(claims,
		`{"ts":"2026-11-15T00:00:00Z","event":"claim","source":"agent","model":"sonnet","outcome":"dropped"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"claim","source":"agent","model":"sonnet","outcome":"kept"}`)
	var out strings.Builder
	if err := Report(guards, claims, Options{Since: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)}, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"rule            blocks  shadow  disabled  skipped  sessions", "plan-critic     0       0       0         1        1", "sensitive-read  1       1       0         0        2", "agent   sonnet  2        1        1", "hook runs  1  $0.05"} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "weaken") {
		t.Errorf("report counts a line before Since:\n%s", got)
	}
}
