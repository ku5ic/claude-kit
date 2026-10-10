package hooks

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
)

func TestToolingBlock(t *testing.T) {
	t.Parallel()
	const pointer = "`kit run-checks --plan` lists them, with where each comes from, without running them\n"
	gates := []enforce.Gate{
		{Label: "lint (ci.yml: go/vet)", Dir: "/r/go", Body: "go vet ./..."},
		{Label: "lint (package.json: lint)", Dir: "/r", Body: "pnpm run lint"},
		{Label: "deadcode (ci.yml: go/deadcode)", Dir: "/r/go", Body: "out=\"$(deadcode ./...)\"\n[ -z \"$out\" ]\n"},
		{Label: "test (ci.yml: shell/bats)", Dir: "/r", Body: "bats test", Skip: "bats only on PATH"},
	}
	for _, c := range []struct {
		name    string
		plan    enforce.Plan
		managed bool
		cli     []string
		want    string
	}{
		{"what run-checks runs, root first, a multi-line body by its label", enforce.Plan{Root: "/r", Gates: gates}, false, nil,
			"run-checks runs:\n  pnpm run lint\nrun-checks runs [go]:\n  go vet ./...\n  deadcode (ci.yml: go/deadcode)\n" + pointer},
		{"unclassified entries", enforce.Plan{Root: "/r", Unclassified: 2}, false, []string{"available: rg"},
			"unclassified: 2 (config entries gap-fill hasn't answered; it runs in the background, and Stop skips them until it answers)\n" + pointer + "available: rg\n"},
		{"nothing stated", enforce.Plan{Root: "/r", Discovery: enforce.Discovery}, false, nil, "discovery: " + enforce.Discovery + "\n"},
		{"guidance only with a verified package manager", enforce.Plan{Root: "/r", Gates: gates[1:2]}, true, nil,
			"run-checks runs:\n  pnpm run lint\n" + pointer + "\nguidance: " + guidance + "\n"},
		{"tools only", enforce.Plan{Root: "/r"}, false, []string{"missing: sg"}, "missing: sg\n"},
		{"nothing at all", enforce.Plan{Root: "/r"}, false, nil, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			want := c.want
			if want != "" {
				want = "\n<tooling>\n" + want + "</tooling>\n"
			}
			if got := toolingBlock(c.plan, c.managed, c.cli); got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
	t.Run("subprojects past 20 are capped with a note", func(t *testing.T) {
		t.Parallel()
		var many []enforce.Gate
		for i := range 22 {
			many = append(many, enforce.Gate{Dir: fmt.Sprintf("/r/p%02d", i), Body: "make test"})
		}
		got := toolingBlock(enforce.Plan{Root: "/r", Gates: many}, false, nil)
		if strings.Count(got, "run-checks runs [") != 20 || !strings.Contains(got, "\n(subprojects capped at 20; kit run-checks covers all)\n") {
			t.Errorf("got:\n%s", got)
		}
	})
}
