package enforce

import (
	"cmp"
	"maps"
	"slices"

	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// fileScoped are the kinds a file form checks as fully as the whole body:
// a finding is in the file it names. A test's file form runs only the tests
// it names, and a type check needs the whole program.
var fileScoped = []string{"lint", "format-check"}

// maxFileBytes bounds the paths a file form takes: the command is one
// bash -c argument, which Linux caps at 128 KiB.
const maxFileBytes = 64 << 10

// verdictGate is a gate in dir carrying what v says of e: its verdict, its
// forms, and its coverer.
func verdictGate(e sources.Entry, v gapfill.Verdict, dir string) Gate {
	return Gate{Dir: dir, Verdict: v.String(), FileForm: v.FileForm, Globs: v.Globs, Affected: v.Affected, Entry: gapfill.Key(e), CoveredBy: v.CoveredBy}
}

// scopeTo scopes the gates b adds to what changed since the git base. Git
// lists the changed files without anything its ignore files leave out, the
// user's global excludes included. With no base every gate stays whole.
func (b *builder) scopeTo(root string) {
	since, ok := git.ChangedSince(root)
	if !ok {
		return
	}
	b.since, b.changed = &since, slices.Sorted(maps.Keys(since.Files))
}

// narrow limits g to what changed, in a scoped plan: its affected form when
// it has one; else, for a lint or format check of one kind, its file form
// on the changed files its globs match, passing unrun when none does; else
// the whole body. A file form whose paths would overflow its argument runs
// whole too.
func (b *builder) narrow(g *Gate) {
	switch {
	case b.since == nil || g.Skip != "":
	case g.Affected != "":
		g.Dir, g.Body = cmp.Or(g.AffectedDir, g.Dir), g.Affected
		g.FanOut = b.fansOut(g.Body)
	case len(g.Kinds) == 1 && slices.Contains(fileScoped, g.Kinds[0]) && g.FileForm != "" && len(g.Globs) > 0:
		switch files := matching(b.changed, g.Dir, g.Globs); {
		case len(files) == 0:
			g.Unchanged = b.since.Base
		case pathBytes(files) <= maxFileBytes:
			g.Files, g.Body = files, g.FileForm
		}
	}
}

// pathBytes is the length of files joined by spaces.
func pathBytes(files []string) int {
	n := 0
	for _, f := range files {
		n += len(f) + 1
	}
	return n
}
