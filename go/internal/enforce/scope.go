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

// maxFileBytes bounds the paths a file form takes: the command is one
// bash -c argument, which Linux caps at 128 KiB.
const maxFileBytes = 64 << 10

// scope narrows each gate to what changed since the git base: its affected
// form when it has one; else, for a lint or format check of one kind, its
// file form on the changed files its globs match, passing unrun when none
// does; else the whole body. Git lists the changed files without anything
// its ignore files leave out, the user's global excludes included. With no
// base every gate stays whole. Gates that narrow to one command run once.
func (b *builder) scope() {
	since, ok := git.ChangedSince(b.root)
	if !ok {
		return
	}
	changed := slices.Sorted(maps.Keys(since.Files))
	for i := range b.gates {
		g := &b.gates[i]
		// Its marks stay those of the gate as planned whole.
		g.markKey = g.key(b.root)
		switch {
		case g.Skip != "":
		case g.Affected != "":
			g.Dir, g.Body = g.AffectedDir, g.Affected
		case len(g.Kinds) == 1 && slices.Contains(fileScoped, g.Kinds[0]) && g.FileForm != "" && len(g.Globs) > 0:
			switch files := matching(changed, g.Dir, g.Globs); {
			case len(files) == 0:
				g.Unchanged = since.Base
			case pathBytes(files) <= maxFileBytes:
				g.Files, g.Body = files, g.FileForm
			}
		}
	}
	seen := map[string]bool{}
	b.gates = slices.DeleteFunc(b.gates, func(g Gate) bool {
		if g.Skip != "" || g.Unchanged != "" {
			return false
		}
		key := g.Dir + "\x00" + g.Command()
		if seen[key] {
			return true
		}
		seen[key] = true
		return false
	})
}

// pathBytes is the length of files joined by spaces.
func pathBytes(files []string) int {
	n := 0
	for _, f := range files {
		n += len(f) + 1
	}
	return n
}
