package bashguard

import (
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"path/filepath"
	"regexp"
	"strings"
)

// looseWrite asks before a write (op is > or tee) that lands loose in the
// current directory, unless git tracks the target: that file is the
// project's already. Where the shell may be elsewhere, it asks.
func (st *state) looseWrite(op, target string) {
	if (!strings.Contains(target, "/") || strings.HasPrefix(target, "./")) && looseWriteTarget(target) && (st.lost || st.inWorktree() && !st.tracked(target)) {
		st.ask("'"+op+" "+target+"' writes into the current directory; skills/kit-rules/reference/tooling.md wants "+op+" \"$(kit scratch-dir)/"+baseName(target)+"\". Confirm only if this file belongs in the project tree.", "loose-write")
	}
}

// inWorktree is true when the tracked cwd (after any cd) is in a git work
// tree and not under a scratch directory: cd /tmp or cd .claude/scratch
// makes a bare > name harmless.
func (st *state) inWorktree() bool { return st.worktreeTop() != "" }

// worktreeTop is the toplevel of the work tree cwd is in, "" outside one
// or in scratch.
func (st *state) worktreeTop() string {
	if st.cwd == "" || st.h.Paths.IsScratch(st.cwd) {
		return ""
	}
	return git.Toplevel(st.cwd)
}

// tracked is true when git tracks target, a path relative to the tracked
// cwd; a failed lookup reads as untracked, so the ask stays.
func (st *state) tracked(target string) bool {
	// literal: a glob in target ('main*') would match other tracked files.
	paths, err := git.Paths(st.cwd, "ls-files", "-z", "--cached", "--", ":(literal)"+target)
	return err == nil && len(paths) > 0
}

// looseWriteTarget is true for a relative target that would land loose in
// the repo instead of scratch. Unresolvable targets (variables,
// substitutions, fd duplications) are false: $(kit scratch-dir) is the
// sanctioned form.
func looseWriteTarget(p string) bool {
	switch {
	case p == "", p == "-", strings.HasPrefix(p, "$"), strings.Contains(p, "$("),
		strings.HasPrefix(p, `"`), strings.HasPrefix(p, "'"), strings.HasPrefix(p, "&"), strings.HasPrefix(p, "("):
		return false
	case p == "scratch", strings.HasPrefix(p, "scratch/"), strings.HasSuffix(p, "/scratch"), strings.Contains(p, "/scratch/"):
		return false
	case strings.HasPrefix(p, "/"), strings.HasPrefix(p, "~"):
		return false
	}
	return true
}

// scratchTarget is true when a download target is stdout, /dev/null, an fd,
// or inside a .claude/scratch directory. $(kit scratch-dir), the kit's own
// bin/kit by any path included, and $PWD count; any other unexpanded
// variable, and any "..", doesn't, since it can't be checked. A path whose
// symlinks lead out of scratch doesn't either; followLast says whether a
// final link counts, as it does for a write, and not for rm, which removes
// the link itself.
func (st *state) scratchTarget(p string, followLast bool) bool {
	p = unquote(p)
	switch {
	case p == "-", p == "/dev/null", len(p) == 2 && p[0] == '&' && p[1] >= '0' && p[1] <= '9':
		return true
	}
	path, ok := st.scratchPath(p)
	return ok && st.onDisk(path, followLast)
}

// scratchPath is p, unquoted, as an absolute path, with $(kit scratch-dir)
// read as the directory it prints and $PWD, $(pwd), ~, and $HOME expanded. False
// with a "..", or any other expansion left in it.
func (st *state) scratchPath(p string) (string, bool) {
	if strings.Contains(p, "..") {
		return "", false
	}
	if runsIn, rest, ok := st.kitScratchDir(p); ok {
		dir, err := project.ScratchDir(st.h.Paths, runsIn, false)
		return dir + rest, err == nil && !strings.ContainsAny(rest, "$`")
	}
	for _, pwd := range []string{"$PWD", "${PWD}", "$(pwd)", "`pwd`"} {
		if st.cwd != "" && (p == pwd || strings.HasPrefix(p, pwd+"/")) {
			if st.lost {
				return "", false
			}
			p = st.cwd + p[len(pwd):]
			break
		}
	}
	ok := st.placed(p)
	p = st.abs(p)
	return p, ok && !strings.ContainsAny(p, "$`")
}

// onDisk is true when p, absolute, is in scratch as written and on disk:
// resolving its symlinks keeps it under its scratch root, resolved too. A
// trailing slash follows a final link, as rm -r link/ does.
func (st *state) onDisk(p string, followLast bool) bool {
	root, ok := st.h.Paths.ScratchRoot(p)
	if !ok {
		return false
	}
	if root == filepath.Clean(p) && !followLast {
		return true
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		// No root, so nothing under it to follow.
		return true
	}
	if followLast || strings.HasSuffix(p, "/") {
		p = fsx.PhysicalPath(filepath.Clean(p))
	}
	return fsx.Within(real, fsx.ResolvedParent(filepath.Clean(p)))
}

// kitScratchSub matches a path that starts with a kit scratch-dir call,
// $(K scratch-dir) or `K scratch-dir`, past a leading `cd DIR &&` and a
// KIT_DEV=1, which picks how bin/kit builds, not what it prints. Its groups
// are DIR and K, once per form.
var kitScratchSub = regexp.MustCompile("^(?:\\$\\((?:cd ([^\\s$`()&;|]+) && )?(?:KIT_DEV=1 )?([^\\s$`()=]+) scratch-dir\\)" +
	"|`(?:cd ([^\\s$`()&;|]+) && )?(?:KIT_DEV=1 )?([^\\s$`()=]+) scratch-dir`)(?:/|$)")

// kitScratchDir reads p's leading scratch-dir call to kit on PATH, or to
// this kit's bin/kit by any path: the directory the call runs in, past its
// cd, and the rest of p. ok is false for any other call, since a kit
// anywhere else could print any directory.
func (st *state) kitScratchDir(p string) (runsIn, rest string, ok bool) {
	m := kitScratchSub.FindStringSubmatch(p)
	if m == nil {
		return "", "", false
	}
	runsIn, kit, cd := st.cwd, m[2]+m[4], m[1]+m[3]
	if cd != "" {
		if !st.placed(cd) {
			return "", "", false
		}
		runsIn = resolveDir(st.pinnedHome(), st.cwd, st.expandPins(cd))
	}
	rest = p[len(strings.TrimSuffix(m[0], "/")):]
	if kit == "kit" {
		return runsIn, rest, true
	}
	path := guard.ExpandHome(st.pinnedHome(), st.expandPins(kit))
	if !filepath.IsAbs(path) {
		if cd == "" && !st.placed(kit) {
			return "", "", false
		}
		path = filepath.Join(runsIn, path)
	}
	return runsIn, rest, st.h.Paths.Root != "" && filepath.Clean(path) == filepath.Join(st.h.Paths.Root, "bin", "kit")
}

// unquote strips one pair of surrounding quotes, single or double.
func unquote(p string) string {
	p = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(p, `"`), `'`), `"`)
	return strings.TrimSuffix(p, `'`)
}

// abs is p with its pinned variables, ~, and $HOME expanded, ~ by a HOME
// the command pinned to an absolute path, joined to the segment's cwd when
// relative.
func (st *state) abs(p string) string {
	p = guard.ExpandHome(st.pinnedHome(), st.expandPins(p))
	if !strings.HasPrefix(p, "/") {
		p = st.cwd + "/" + p
	}
	return p
}

// placed is true when p, as abs reads it, lands where abs puts it: it's
// absolute, or the shell surely runs in the tracked cwd.
func (st *state) placed(p string) bool {
	return !st.lost || strings.HasPrefix(guard.ExpandHome(st.pinnedHome(), st.expandPins(p)), "/")
}

// askKitSwitch asks, and is true, when a word (quoted, ~- or
// $HOME-prefixed, or relative to the segment's cwd) names a path
// guard.KitSwitch protects, through a symlink or not. Only a writer's
// operands and a redirect's target get here, so resolving each is cheap.
func (st *state) askKitSwitch(arg string) bool {
	what := st.kitSwitch(arg)
	if what != "" {
		st.ask(guard.KitSwitchAsk(what), "kit-switch")
	}
	return what != ""
}

// kitSwitch is what guard.KitSwitch names for arg, resolved as
// askKitSwitch resolves it, or "".
func (st *state) kitSwitch(arg string) string {
	return guard.KitSwitch(st.h.Paths, st.abs(unquote(arg)))
}
