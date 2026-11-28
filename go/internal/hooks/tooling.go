package hooks

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/doctor"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// guidance follows the gates when gap-fill has verified a package manager.
const guidance = "The commands above are the project's own, and each directory's scripts run through the package manager <repo-context> names for it."

// tooling is the <tooling> block: what kit run-checks runs, planned from
// the gap-fill cache (a session start never waits on the classifier), then
// kit.yml's tools split by whether PATH has them.
func tooling(cfg *config.Config, o enforce.Options) string {
	p := enforce.Build(cfg, o)
	return toolingBlock(p, p.Managed, cliLines(cfg))
}

// toolingBlock is p's gates, its unclassified entries or discovery
// instruction, and cli, with guidance when managed; "" when there's none
// of it.
func toolingBlock(p enforce.Plan, managed bool, cli []string) string {
	body := gateLines(p)
	if p.Unclassified > 0 {
		body = append(body, fmt.Sprintf("unclassified: %d (config entries gap-fill hasn't answered; it asks in the background and at Stop, and waits an hour after a failed answer)", p.Unclassified))
	}
	if len(body) > 0 {
		body = append(body, "`kit run-checks --plan` lists them, with where each comes from, without running them")
	}
	if p.Discovery != "" {
		body = append(body, "discovery: "+p.Discovery)
	}
	body = append(body, cli...)
	if len(body) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("\n<tooling>\n")
	for _, line := range body {
		out.WriteString(line + "\n")
	}
	if managed {
		out.WriteString("\nguidance: " + guidance + "\n")
	}
	out.WriteString("</tooling>\n")
	return out.String()
}

// gateLines are the commands p runs under a header per directory, the
// root's first, past 20 subprojects capped with a note. A body of several
// lines shows as its gate's label.
func gateLines(p enforce.Plan) []string {
	dirs := []string{"."}
	byDir := map[string][]string{}
	for _, g := range p.Gates {
		if g.Skip != "" {
			continue
		}
		dir := fsx.Rel(p.Root, g.Dir)
		if !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
		cmd := strings.TrimSpace(g.Body)
		if strings.Contains(cmd, "\n") {
			cmd = g.Label
		}
		byDir[dir] = append(byDir[dir], "  "+cmd)
	}
	var out []string
	shown := 0
	for _, dir := range dirs {
		if len(byDir[dir]) == 0 {
			continue
		}
		if dir != "." {
			if shown == 20 {
				return append(out, "(subprojects capped at 20; kit run-checks covers all)")
			}
			shown++
		}
		out = append(out, "run-checks runs"+project.SubLabel(dir)+":")
		out = append(out, byDir[dir]...)
	}
	return out
}

// cliLines says which of kit.yml's CLI tools are on PATH.
func cliLines(cfg *config.Config) []string {
	available, missing := doctor.Tools(cfg.Tools)
	var out []string
	if len(available) > 0 {
		out = append(out, "available: "+strings.Join(available, ", "))
	}
	if len(missing) > 0 {
		out = append(out, "missing: "+strings.Join(missing, ", "))
	}
	return out
}
