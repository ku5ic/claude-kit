package bashguard

import (
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"iter"
	"path/filepath"
	"slices"
	"strings"
)

// dirs is where a command line's next command runs: cwd, which each cd
// moves as if it ran and worked, and prev, the cwd before the last cd, for
// cd -. maybe are the segments of cds that may not have; lost is true once
// a segment runs where one of them may not have, so the shell may be
// anywhere cwd passed through. A check that passes a path by where it
// lands reads cwd only while lost is false. lostIn is the subshell a cd
// moved, which the walk doesn't follow: the cwd is lost until it ends, then
// lostBefore again.
type dirs struct {
	home, cwd, prev string
	maybe           []int
	lost            bool
	lostIn          int
	lostBefore      bool
}

// enter readies d for segment j of segs: one past the subshell a cd moved
// finds the cwd again, and one that runs even when a cd in maybe didn't
// loses it.
func (d *dirs) enter(segs []Segment, j int) {
	if d.lostIn != 0 && !slices.Contains(segs[j].scopes, d.lostIn) {
		d.lost, d.lostIn = d.lostBefore, 0
	}
	if slices.ContainsFunc(d.maybe, func(k int) bool { return !andChain(segs, k, j) }) {
		d.maybe, d.lost = nil, true
	}
}

// loseIn loses the cwd for the rest of seg's innermost subshell, where a cd
// in seg moved the shell: (cd /; rm -rf x) runs rm in /. Outside any
// subshell, it's lost for good.
func (d *dirs) loseIn(seg Segment) {
	switch {
	case len(seg.scopes) == 0:
		d.maybe, d.lost = nil, true
	case d.lostIn == 0:
		d.lostIn, d.lostBefore, d.lost = seg.scopes[len(seg.scopes)-1], d.lost, true
	}
}

// movesShell is whether a cd at call ci of seg can move the shell that runs
// the rest of seg's subshell, or the command line: alone, or last in a
// pipeline, which zsh runs in the current shell (bash too, with lastpipe).
func movesShell(seg Segment, ci int) bool { return ci == len(seg.Calls)-1 }

// moveAt moves d as name, a cd, pushd, or popd standing alone in segment k
// of segs, with args, would. A cd surely moves the shell when it surely
// runs and set -e stops on its failure or its directory exists; one to an
// absolute directory then finds the cwd again. pushd, popd, and a cd the
// guard can't follow lose it.
func (d *dirs) moveAt(segs []Segment, k int, name string, args []Word) {
	if name != "cd" || !d.tracks(args) {
		d.maybe, d.lost = nil, true
		return
	}
	d.cd(args)
	seg := segs[k]
	switch {
	case seg.conditional || !fsx.IsDir(d.cwd) && (!standalone(seg) || !errexit(segs[:k])):
		d.maybe = append(d.maybe, k)
	case len(args) == 0 || filepath.IsAbs(guard.ExpandHome(d.home, args[0].Value)):
		d.maybe, d.lost = nil, false
	}
}

// movesDir is whether a call named name moves the shell's directory.
func movesDir(name string) bool { return name == "cd" || name == "pushd" || name == "popd" }

// tracks is true when cd with args moves d to a directory d can name: not
// with an option, nor cd - before any cd.
func (d *dirs) tracks(args []Word) bool {
	switch {
	case len(args) == 0:
		return true
	case args[0].Value == "-":
		return d.prev != ""
	default:
		return !strings.HasPrefix(args[0].Value, "-")
	}
}

// cd moves d as cd with args would.
func (d *dirs) cd(args []Word) {
	if !d.tracks(args) {
		return
	}
	prev := d.cwd
	switch {
	case len(args) == 0:
		d.cwd = d.home
	case args[0].Value == "-":
		d.cwd = d.prev
	default:
		d.cwd = resolveDir(d.home, d.cwd, args[0].Value)
	}
	d.prev = prev
}

// walked is one parsed call as walk yields it: its segment, the call, the
// index of its real command word, that command's base name, the directory
// it runs in, and whether it surely runs there (dirs.lost).
type walked struct {
	seg  Segment
	call Call
	i    int
	name string
	cwd  string
	sure bool
}

// walk yields each call of segs with a command word, run from cwd, a cd
// moving the directory for the calls after it.
func walk(segs []Segment, home, cwd string) iter.Seq[walked] {
	return walkPinned(segs, home, cwd, nil)
}

// pinFunc is words, in segment at, with each pinned variable read as its
// path (state.pinWords).
type pinFunc func(at int, words []Word) []Word

// walkPinned is walk with each call's words read through pin, when set.
func walkPinned(segs []Segment, home, cwd string, pin pinFunc) iter.Seq[walked] {
	return func(yield func(walked) bool) {
		d := dirs{home: home, cwd: cwd}
		for at, seg := range segs {
			d.enter(segs, at)
			for ci, call := range seg.Calls {
				if pin != nil {
					call.Words = pin(at, call.Words)
				}
				i := lead(call.Words)
				if i >= len(call.Words) {
					continue
				}
				name := baseName(call.Words[i].Value)
				// A cd before the last command of a pipeline moves nothing; one
				// that ends a pipeline, or sits in a subshell, moves a shell
				// the walk doesn't follow, so the cwd is lost.
				switch {
				case !movesDir(name) || !movesShell(seg, ci):
				case len(seg.Calls) == 1 && !seg.subshell:
					d.moveAt(segs, at, name, call.Words[i+1:])
				default:
					d.loseIn(seg)
				}
				if !yield(walked{seg, call, i, name, d.cwd, !d.lost}) {
					return
				}
			}
		}
	}
}
