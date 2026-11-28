package enforce

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/resolve"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// resolve finds each gate's binaries; the first that can't run skips it.
func (b *builder) resolve() {
	for i := range b.gates {
		g := &b.gates[i]
		if g.Skip != "" {
			continue
		}
		calls, ok := classify.Calls(g.Body)
		if !ok {
			g.Skip = "can't read the command"
			continue
		}
		for _, words := range calls {
			if words[0] == "$" {
				g.Skip = "a variable names the command it runs"
				break
			}
			res := b.resolver.Command(g.Dir, words)
			if res.Skip != "" {
				g.Skip, g.Bins = res.Skip, nil
				break
			}
			if !slices.ContainsFunc(g.Bins, func(r resolve.Resolution) bool { return r.Path == res.Path }) {
				g.Bins = append(g.Bins, res)
			}
		}
	}
}

// subOf is the deepest subproject holding dir.
func (b *builder) subOf(dir string, subs []string) string {
	best := "."
	for _, sub := range subs {
		abs := filepath.Join(b.root, sub)
		if fsx.Within(abs, dir) && len(sub) > len(best) {
			best = sub
		}
	}
	return best
}

// provider is the task runner a source names, when its entries are tasks.
func provider(source string) (sources.TaskProvider, bool) {
	tp, ok := sources.Provider(source)
	return tp, ok && tp.Tasks != nil
}

// runCommand is how tp runs task in sub: through the package manager the
// nearest verified lockfile names, when tp runs by it, else tp.PM.
func (b *builder) runCommand(tp sources.TaskProvider, sub, task string) string {
	pm := ""
	for _, m := range gapfill.Nearest(b.root, filepath.Join(b.root, sub), b.facts.Managers) {
		_, override := tp.RunByPM[m.Manager]
		// {pm} is tp.PM, or a manager that rivals it.
		if override || tp.PM != "" && (m.Manager == tp.PM || slices.Contains(m.Rivals, tp.PM)) {
			pm = m.Manager
			break
		}
	}
	run := tp.Run
	if override, ok := tp.RunByPM[pm]; ok {
		run = override
	}
	pm = cmp.Or(pm, tp.PM)
	return strings.NewReplacer("{pm}", pm, "{task}", task).Replace(run)
}

// label is a gate's line: the kinds it checks ("check" for an aggregate the
// verdict gives none), where it comes from, and its subproject.
func label(kinds []string, origin, sub string) string {
	what := strings.Join(kinds, "+")
	if what == "" {
		what = "check"
	}
	return what + " (" + origin + ")" + project.SubLabel(sub)
}

// rel is dir relative to the root, for a label.
func (b *builder) rel(dir string) string { return fsx.Rel(b.root, dir) }
