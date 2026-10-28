package enforce

import (
	"maps"
	"slices"

	"github.com/ku5ic/claude-kit/go/internal/git"
)

// fileScoped are the kinds a file form checks as fully as the whole body:
// a finding is in the file it names. A test's file form runs only the tests
// it names, and a type check needs the whole program.
var fileScoped = []string{"lint", "format-check"}

// scope narrows each gate to what changed since the git base: its affected
// form when it has one; else, for a lint or format check of one kind, its
// file form on the changed files its globs match, passing unrun when none
// does; else the whole body. Git lists the changed files without anything
// its ignore files leave out, the user's global excludes included. With no
// base every gate stays whole.
func (b *builder) scope() {
	since, ok := git.ChangedSince(b.root)
	if !ok {
		return
	}
	changed := slices.Sorted(maps.Keys(since.Files))
	for i := range b.gates {
		g := &b.gates[i]
		switch {
		case g.Skip != "":
		case g.Affected != "":
			g.Dir, g.Body = g.AffectedDir, g.Affected
		case len(g.Kinds) == 1 && slices.Contains(fileScoped, g.Kinds[0]) && g.FileForm != "" && len(g.Globs) > 0:
			if g.Files = matching(changed, g.Dir, g.Globs); len(g.Files) > 0 {
				g.Body = g.FileForm
			} else {
				g.Unchanged = since.Base
			}
		}
	}
}
