package stats

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestFlaggedReadsEachClaimLine(t *testing.T) {
	t.Parallel()
	reply := "- `Foo` writes the cache -> rg -n 'func Foo'\n2. the test passes -> run go test ./x\nok\nagentId: a1\n"
	want := []string{"`Foo` writes the cache", "the test passes"}
	if got := Flagged(reply); !slices.Equal(got, want) {
		t.Errorf("Flagged = %q, want %q", got, want)
	}
	if got := Flagged("ok\n"); got != nil {
		t.Errorf("Flagged(ok) = %q, want none", got)
	}
}

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
		`not json`)
	write(claims,
		`{"ts":"2026-11-15T00:00:00Z","event":"claim","source":"agent","model":"sonnet","outcome":"dropped"}`,
		`{"ts":"2026-11-15T00:00:00Z","event":"claim","source":"agent","model":"sonnet","outcome":"kept"}`)
	var out strings.Builder
	if err := Report(guards, claims, Options{Since: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)}, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"sensitive-read  1       1       0         2", "agent   sonnet  2        1        1", "hook runs  1  $0.05"} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "weaken") {
		t.Errorf("report counts a line before Since:\n%s", got)
	}
}
