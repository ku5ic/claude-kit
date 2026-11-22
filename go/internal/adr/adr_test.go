package adr

import (
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestDir(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string // relative to root; "" for none
	}{
		{".adr-dir names the folder", map[string]string{".adr-dir": "architecture/decisions\n", "docs/adr/0001-x.md": ""}, "architecture/decisions"},
		{"an empty .adr-dir falls back", map[string]string{".adr-dir": "\n", "docs/adr/0001-x.md": ""}, "docs/adr"},
		{"adr-tools' default comes first", map[string]string{"doc/adr/0001-x.md": "", "docs/decisions/0001-x.md": ""}, "doc/adr"},
		{"MADR's folder", map[string]string{"docs/decisions/0001-x.md": ""}, "docs/decisions"},
		{"none", map[string]string{"README.md": ""}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for name, body := range tc.files {
				testutil.Put(t, root, name, body)
			}
			want := ""
			if tc.want != "" {
				want = filepath.Join(root, tc.want)
			}
			if got := Dir(root); got != want {
				t.Errorf("Dir = %q, want %q", got, want)
			}
		})
	}
}

func TestAccepted(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text      string
		accepted        bool
		title, decision string
		cites, notCites string
	}{
		{"adr-tools, accepted", "# 1. Use Go\n\nDate: 2026-01-01\n\n## Status\n\nAccepted\n\n## Context\n\nWhy.\n\n## Decision\n\nHooks live in `go/internal/hooks/`.\n\n## Consequences\n\nMore Go.\n",
			true, "1. Use Go", "Hooks live in `go/internal/hooks/`.", "go/internal/hooks/stop.go", "go/internal/hook.go"},
		{"adr-tools, accepted with a note", "# 2. Amend\n\n## Status\n\nAccepted, amends [1](0001-use-go.md)\n\n## Decision\n\nUse `kit.yml`.\n",
			true, "2. Amend", "Use `kit.yml`.", "kit.yml", "go"},
		{"adr-tools, superseded", "# 3. Old\n\n## Status\n\nSuperseded by [4](0004-new.md)\n\n## Decision\n\nUse `x.go`.\n",
			false, "", "", "", ""},
		{"MADR front matter, accepted", "---\nstatus: accepted\ndate: 2026-01-01\n---\n# Use MADR\n\n## Decision Outcome\n\nChosen option: `docs/decisions/`.\n",
			true, "Use MADR", "Chosen option: `docs/decisions/`.", "docs/decisions/0001-x.md", "docs/adr/0001-x.md"},
		{"MADR front matter, quoted status", "---\nstatus: \"accepted\"\n---\n# Quoted\n\n## Decision Outcome\n\nYes.\n",
			true, "Quoted", "Yes.", "", ""},
		{"MADR front matter, proposed", "---\nstatus: proposed\n---\n# Maybe\n\n## Decision Outcome\n\nUse `y.go`.\n",
			false, "", "", "", ""},
		{"a bare word in backticks names no folder", "# 5. Words\n\n## Status\n\nAccepted\n\n## Decision\n\nWrite it in `go`.\n",
			true, "5. Words", "Write it in `go`.", "", "go/internal/x.go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			testutil.Put(t, dir, "0001-record.md", tc.text)
			testutil.Put(t, dir, "notes.txt", tc.text)
			got := Accepted(dir)
			if !tc.accepted {
				if len(got) != 0 {
					t.Errorf("Accepted = %+v, want none", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("Accepted = %+v, want the .md record alone", got)
			}
			a := got[0]
			if a.Path != filepath.Join(dir, "0001-record.md") || a.Title != tc.title || a.Decision != tc.decision {
				t.Errorf("got path %q, title %q, decision %q", a.Path, a.Title, a.Decision)
			}
			if tc.cites != "" && !a.Cites(tc.cites) {
				t.Errorf("doesn't cite %q", tc.cites)
			}
			if tc.notCites != "" && a.Cites(tc.notCites) {
				t.Errorf("cites %q", tc.notCites)
			}
		})
	}
}
