package hooks

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// A block shown once is guarded by its marker. A marker that can't be
// saved would bring the block back on every retry, so each such hook lets
// the call through and says so. Each case first shows the call blocks.
func TestAnUnsavableMarkerLetsTheCallThrough(t *testing.T) {
	t.Parallel()
	blocked := func(r result) bool {
		return r.Status == 2 || strings.Contains(r.Stdout, `"permissionDecision":"deny"`) || strings.Contains(r.Stdout, `"decision":"block"`)
	}
	project := func(t *testing.T) string {
		dir := repo(t, t.TempDir())
		testutil.Put(t, dir, "src/stop.go", "a\nb\nc\n")
		return dir
	}
	const badCite = "- src/stop.go:40 returns early"
	gatherer := func(p map[string]any, dir string) map[string]any {
		p["cwd"], p["agent_type"] = dir, "kit:gatherer"
		return p
	}
	for _, tc := range []struct {
		name, hook, kind, rule string
		setup                  func(k *sandbox)
		call                   func(t *testing.T, k *sandbox) map[string]any
	}{
		{"a gatherer handback's citations", "check-citations", cache.Cites, "citations", nil, func(t *testing.T, _ *sandbox) map[string]any {
			return gatherer(handback("a1", badCite), project(t))
		}},
		{"a gatherer stop's citations", "check-citations", cache.Cites, "citations", nil, func(t *testing.T, _ *sandbox) map[string]any {
			return gatherer(subagentStop("a1", badCite, false), project(t))
		}},
		{"a plan's citations", "check-citations", cache.Cites, "citations", nil, func(t *testing.T, _ *sandbox) map[string]any {
			dir := project(t)
			p := toolCall("ExitPlanMode", map[string]any{"plan": "# Plan\n\n" + badCite, "planFilePath": filepath.Join(dir, ".claude/plans/p.md")})
			p["cwd"] = dir
			return p
		}},
		{"a handback over its size budget", "check-size", cache.Size, "size", func(k *sandbox) { k.overlay("size_budgets: {report: 2}\n") }, func(*testing.T, *sandbox) map[string]any {
			return handback("a1", "One two three four.")
		}},
		{"a handback's AI tells", "guard-handback", cache.Slop, "slop", nil, func(*testing.T, *sandbox) map[string]any {
			return handback("a1", "## Findings\n\n"+tell)
		}},
		{"a handback's claims", "verify-claims", cache.Claims, "claims", func(k *sandbox) { k.verifier("", claim) }, func(*testing.T, *sandbox) map[string]any {
			return handback("a1", claim)
		}},
		{"a subagent stop's claims", "verify-claims", cache.Claims, "claims", func(k *sandbox) { k.verifier("", claim) }, func(*testing.T, *sandbox) map[string]any {
			return subagentStop("a1", claim, false)
		}},
		{"a subagent's report claims", "verify-claims", cache.Claims, "claims", func(k *sandbox) { k.verifier("", claim) }, func(_ *testing.T, k *sandbox) map[string]any {
			p := toolCall("Write", map[string]any{"file_path": filepath.Join(k.home, "proj", ".claude", "scratch", "explore-x-20261110-1200.md"), "content": "# Explore\n\n" + claim})
			p["agent_id"] = "a1"
			return p
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, unsavable := range []bool{false, true} {
				k := newSandbox(t)
				if tc.setup != nil {
					tc.setup(k)
				}
				if unsavable {
					// A file sits where the marker's directory goes, so the write fails.
					testutil.Write(t, filepath.Join(k.paths.CacheDir(), tc.kind), "")
				}
				r := k.run(tc.hook, tc.call(t, k))
				switch {
				case !unsavable && !blocked(r):
					t.Fatalf("the call isn't blocked to begin with, so this case proves nothing:\n%s", r.Output)
				case unsavable && blocked(r):
					t.Errorf("blocked with no marker to end it:\n%s", r.Output)
				case unsavable:
					r.Has(t, "kit "+tc.rule+": let through unchecked")
				}
			}
		})
	}
}
