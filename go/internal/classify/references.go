package classify

import (
	"path/filepath"
	"slices"
	"strings"
)

// runnerGrammar is the command line of a runner that takes several task
// names: flags naming another file, setting the directory (dirGlued when
// glued to its value), or taking a value (numeric ones only before a
// number). make runs every target it's given; just (namesLookedUp) runs
// each word that names a recipe, the rest being arguments to the recipe
// before it.
type runnerGrammar struct {
	file, dir, numeric, valued []string
	dirGlued                   string
	namesLookedUp              bool
}

var runnerFlags = map[string]runnerGrammar{
	"make": {file: []string{"-f", "--file", "--makefile", "--justfile"}, dir: []string{"-C", "--directory"}, dirGlued: "-C",
		numeric: []string{"-j", "-l"}, valued: []string{"-o", "-W", "-I"}},
	"just": {file: []string{"-f", "--file", "--makefile", "--justfile"}, dir: []string{"-d", "--working-directory"}, namesLookedUp: true},
}

// longestReference is the reference whose words are the longest prefix of
// words, and that prefix's length in words.
func longestReference(words []string) (*Reference, int) {
	var best *Reference
	bestLen := 0
	for i, r := range Grammar.References {
		n := len(strings.Fields(r.Words))
		if n > bestLen && hasPrefix(words, strings.Fields(r.Words)) {
			best, bestLen = &Grammar.References[i], n
		}
	}
	return best, bestLen
}

// refArgs reads the words after a reference prefix into ref: its directory,
// name, and arguments, plus the further task names of a runner that takes
// several. kind is Ref, or FanOut or Other when one word makes the whole
// command that.
func (c *classifier) refArgs(ref TaskRef, rest []string) (_ TaskRef, more []string, kind Kind) {
	flags, runner := runnerFlags[ref.Provider]
	for i := 0; i < len(rest); i++ {
		w := rest[i]
		switch {
		case w == "--":
			ref.Args = rest[i+1:]
			return ref, more, Ref
		case slices.Contains(Grammar.FanOutFlags, w) || slices.ContainsFunc(Grammar.FanOutFlags, func(f string) bool { return strings.HasPrefix(w, f+"=") }):
			return ref, nil, FanOut
		}
		if runner {
			skip, kind, read := flags.option(&ref, rest, i)
			if kind != Ref {
				return ref, nil, kind
			}
			if read {
				i += skip
				continue
			}
		}
		switch {
		case strings.HasPrefix(w, "-"):
		case ref.Name == "":
			ref.Name = w
		case runner && (!flags.namesLookedUp || len(ref.Args) == 0 && c.lookup(ref.Provider, ref.Dir, w)):
			more = append(more, w)
		default:
			ref.Args = append(ref.Args, w)
		}
	}
	return ref, more, Ref
}

// option reads a runner's word rest[i] into ref when it's one of g's
// options: how many words after it the option takes, Other for another
// file's targets the kit can't look up, and whether it was one. A
// variable (make GOARCH=arm64 build) and a flag's value (make -j 4) are
// no target.
func (g runnerGrammar) option(ref *TaskRef, rest []string, i int) (skip int, kind Kind, read bool) {
	w, next := rest[i], i+1 < len(rest)
	switch {
	case slices.Contains(g.file, w):
		return 0, Other, true
	case slices.Contains(g.dir, w) && next:
		ref.Dir = rest[i+1]
		return 1, Ref, true
	case g.dirGlued != "" && strings.HasPrefix(w, g.dirGlued):
		ref.Dir = strings.TrimPrefix(w, g.dirGlued)
		return 0, Ref, true
	case slices.Contains(g.numeric, w) && next && isNumber(rest[i+1]), slices.Contains(g.valued, w) && next:
		return 1, Ref, true
	case strings.Contains(w, "="):
		return 0, Ref, true
	}
	return 0, Ref, false
}

// RefName is the task the words after r's prefix name, past option values
// (make -C web lint names lint), or "" when they name none the kit reads.
func RefName(r Reference, rest []string) string {
	c := &classifier{lookup: func(string, string, string) bool { return false }}
	ref, _, kind := c.refArgs(TaskRef{Provider: r.Provider, Name: r.Task}, rest)
	if kind != Ref {
		return ""
	}
	return ref.Name
}

// reference reads words as a task reference, by the longest matching
// reference prefix.
func (c *classifier) reference(words, env []string) (Command, bool) {
	best, bestLen := longestReference(words)
	if best == nil {
		return Command{}, false
	}
	ref, more, kind := c.refArgs(TaskRef{Provider: best.Provider, Name: best.Task}, words[bestLen:])
	if kind != Ref {
		return Command{Kind: kind, Words: words, Env: env}, true
	}
	if ref.Name == "" || filepath.IsAbs(ref.Dir) || strings.HasPrefix(ref.Dir, "~") {
		// make -C /opt/app: outside the repo, so not a task the kit can read.
		return Command{Kind: Other, Words: words, Env: env}, true
	}
	if len(more) > 0 {
		refs := []TaskRef{ref}
		for _, name := range more {
			refs = append(refs, TaskRef{Provider: ref.Provider, Dir: ref.Dir, Name: name})
		}
		return Command{Kind: Ref, Words: words, Env: env, Refs: refs}, true
	}
	if best.Shorthand && !c.lookup(ref.Provider, ref.Dir, ref.Name) {
		// pnpm eslint: no such script, so pnpm runs the eslint binary.
		return Command{Kind: Other, Words: words[bestLen:], Env: env}, true
	}
	return Command{Kind: Ref, Words: words, Env: env, Refs: []TaskRef{ref}}, true
}
