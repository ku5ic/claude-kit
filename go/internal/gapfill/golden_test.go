package gapfill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// golden is one fixture's verified result, read back by name.
type golden struct {
	t       *testing.T
	entries []sources.Entry
	r       Result
}

func (g golden) verdict(source, name string) Verdict {
	g.t.Helper()
	for _, e := range g.entries {
		if e.Source == source && e.Name == name {
			if v, ok := g.r.Verdicts[Key(e)]; ok {
				return v
			}
			g.t.Fatalf("%s %s: %s", source, name, g.r.Skipped[Key(e)])
		}
	}
	g.t.Fatalf("no entry %s %s", source, name)
	return Verdict{}
}

func (g golden) manager() Manager {
	g.t.Helper()
	if len(g.r.Managers) != 1 {
		g.t.Fatalf("managers: %+v", g.r.Managers)
	}
	return g.r.Managers[0]
}

// proposed is the kinds (fixer for a formatter) the proposals cover.
func (g golden) proposed() []string {
	var kinds []string
	for _, p := range g.r.Proposals {
		kinds = append(kinds, map[bool]string{true: p.Kind, false: "fixer"}[p.Role == "check"])
	}
	slices.Sort(kinds)
	return slices.Compact(kinds)
}

// TestGoldenVerdicts replays each fixture's recorded live answer through
// the verifier and checks what it says. KIT_RECORD_GOLDEN=1 asks the live
// classifier instead, rewriting the recordings: review the diff.
func TestGoldenVerdicts(t *testing.T) {
	t.Parallel()
	cfg := testutil.KitConfig(t)
	record := os.Getenv("KIT_RECORD_GOLDEN") == "1"
	checks := map[string]func(g golden){
		"turbo": func(g golden) {
			if v := g.verdict("turbo", "lint"); v.Role != "check" || v.Kind != "lint" || !strings.Contains(v.Affected, "{base}") {
				g.t.Errorf("turbo lint: %+v", v)
			}
			if v := g.verdict("turbo", "build"); v.Role != "build" {
				g.t.Errorf("turbo build: %+v", v)
			}
			if m := g.manager(); m.Manager != "pnpm" || m.Cites != "pnpm-lock.yaml" {
				g.t.Errorf("manager: %+v", m)
			}
		},
		"nx": func(g golden) {
			if v := g.verdict("nx", "test"); v.Role != "check" || v.Kind != "test" {
				g.t.Errorf("nx test: %+v", v)
			}
			if m := g.manager(); m.Manager != "npm" {
				g.t.Errorf("manager: %+v", m)
			}
		},
		"poetry": func(g golden) {
			if m := g.manager(); m.Manager != "poetry" || m.RunPrefix != "poetry run" {
				g.t.Errorf("manager: %+v", m)
			}
			for _, kind := range []string{"lint", "test", "typecheck"} {
				if !slices.Contains(g.proposed(), kind) {
					g.t.Errorf("no %s proposal: %+v", kind, g.r.Proposals)
				}
			}
			for _, p := range g.r.Proposals {
				if !strings.HasPrefix(p.Command, "poetry run ") {
					g.t.Errorf("a declared dependency runs through poetry run: %+v", p)
				}
			}
		},
		"yarn-pnp": func(g golden) {
			if m := g.manager(); m.Manager != "yarn" || m.Cites != "yarn.lock" && m.Cites != "package.json#packageManager" {
				g.t.Errorf("manager: %+v", m)
			}
			if v := g.verdict("package-scripts", "lint"); v.Role != "check" || v.Kind != "lint" {
				g.t.Errorf("lint: %+v", v)
			}
			// The command beside the reference gets its kind as a segment.
			v := g.verdict("package-scripts", "ci")
			if !slices.ContainsFunc(v.Segments, func(s Segment) bool { return s.Text == "prettier --check ." && s.Kind == "format-check" }) {
				g.t.Errorf("ci: %+v", v)
			}
		},
		"makefile": func(g golden) {
			if v := g.verdict("make", "lint"); v.Role != "check" || v.Kind != "lint" {
				g.t.Errorf("lint: %+v", v)
			}
			if v := g.verdict("make", "deploy"); v.Role != "deploy" {
				g.t.Errorf("deploy: %+v", v)
			}
			if v := g.verdict("make", "check"); v.Role != "check" {
				g.t.Errorf("check aggregates checks: %+v", v)
			}
		},
		"pre-commit": func(g golden) {
			if v := g.verdict("pre-commit", "ruff-format"); v.Role != "fixer" || !v.Mutates {
				g.t.Errorf("ruff-format: %+v", v)
			}
			if v := g.verdict("pre-commit", "mypy"); v.Role != "check" || v.Kind != "typecheck" {
				g.t.Errorf("mypy: %+v", v)
			}
		},
		"lint-staged": func(g golden) {
			for _, e := range g.entries {
				v := g.r.Verdicts[Key(e)]
				if e.Source == "lint-staged" && strings.Contains(e.Body.Text, "--check") != (v.Role == "check") {
					g.t.Errorf("%s %q: %+v", e.Name, e.Body.Text, v)
				}
			}
		},
		"go-noci": func(g golden) {
			for _, kind := range []string{"lint", "test", "fixer"} {
				if !slices.Contains(g.proposed(), kind) {
					g.t.Errorf("no %s proposal: %+v", kind, g.r.Proposals)
				}
			}
			for _, p := range g.r.Proposals {
				if p.Evidence != "go.mod" {
					g.t.Errorf("go.mod is the evidence: %+v", p)
				}
			}
		},
		"rust": func(g golden) {
			for _, kind := range []string{"lint", "test", "format-check"} {
				if !slices.Contains(g.proposed(), kind) {
					g.t.Errorf("no %s proposal: %+v", kind, g.r.Proposals)
				}
			}
		},
	}
	for name := range testutil.Fixtures(t, "testdata") {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := testutil.Fixture(t, "testdata", name)
			recording, _ := filepath.Abs(filepath.Join("testdata", name+".golden.json"))
			c := *cfg
			c.Classifier = testutil.Replayer(t, recording)
			replies := t.TempDir()
			if record {
				stub := filepath.Join(t.TempDir(), "classifier")
				testutil.FakeTool(t, stub, filepath.Join(t.TempDir(), "calls"), fmt.Sprintf("\"$@\" | tee %q/$$.json", replies))
				c.Classifier = append([]string{stub}, claude()...)
			}
			entries := sources.Entries(&c, root, project.Subprojects(root))
			r := Run(&c, Options{Root: root, CacheDir: t.TempDir(), Entries: entries, Ask: true, Timeout: 2 * time.Minute})
			if record {
				merge(t, replies, recording)
			}
			if r.Unclassified() != 0 {
				t.Errorf("unclassified: %v", r.Skipped)
			}
			for _, d := range r.Dropped {
				t.Logf("dropped: %s", d)
			}
			checks[name](golden{t, entries, r})
		})
	}
}

// merge writes the envelopes recorded in replies, one per call, as one
// envelope holding every call's answers, indented: replayed to every call,
// each keeps what it asked for.
func merge(t *testing.T, replies, recording string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(replies, "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no replies recorded in %s: %v", replies, err)
	}
	merged := response{Entries: []entryVerdict{}, Managers: []Manager{}, Proposals: []Proposal{}}
	for _, p := range paths {
		var envelope struct {
			IsError    bool     `json:"is_error"`
			Subtype    string   `json:"subtype"`
			Structured response `json:"structured_output"`
		}
		if err := json.Unmarshal([]byte(testutil.Read(t, p)), &envelope); err != nil {
			t.Fatalf("recording %s: %v", p, err)
		}
		if envelope.IsError {
			t.Fatalf("recording %s: no answer (%s)", p, envelope.Subtype)
		}
		merged.Entries = append(merged.Entries, envelope.Structured.Entries...)
		merged.Managers = append(merged.Managers, envelope.Structured.Managers...)
		merged.Proposals = append(merged.Proposals, envelope.Structured.Proposals...)
	}
	// Calls finish in any order: sorting keeps a re-recording's diff to what changed.
	slices.SortFunc(merged.Entries, func(a, b entryVerdict) int { return strings.Compare(a.ID, b.ID) })
	data, err := json.MarshalIndent(map[string]any{"is_error": false, "subtype": "success", "structured_output": merged}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	testutil.Put(t, filepath.Dir(recording), filepath.Base(recording), string(data)+"\n")
}
