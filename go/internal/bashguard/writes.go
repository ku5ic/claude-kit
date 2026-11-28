package bashguard

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/layout"
)

// inPlaceEdit guards sed -i and sd, which is always in place when given a
// file: blocked on a shell rc file, asked on the overlay.
func (c *command) inPlaceEdit() error {
	inPlace := c.name == "sd"
	for _, v := range c.values() {
		if v == "--" {
			break
		}
		if strings.HasPrefix(v, "-") && strings.Contains(v, "i") {
			inPlace = true
		}
	}
	if !inPlace {
		return nil
	}
	for _, p := range c.operands() {
		if guard.IsRCFile(c.st.cfg, c.st.home, p) {
			if err := c.block("in-place edit of a shell rc file\n"+layout.KV("do", "edit it in the dotfiles repo"), "rc-inplace-edit"); err != nil {
				return err
			}
		}
		c.st.askKitSwitch(p)
	}
	return nil
}

// rcWrite blocks tee into a shell rc file and cp, mv, or install onto one;
// ln is left alone, as symlinking the dotfiles copy in is the fix.
func (c *command) rcWrite() error {
	var targets []string
	switch c.name {
	case "tee":
		targets = c.operands()
	case "cp", "mv", "install":
		if ops := c.operands(); len(ops) > 1 {
			targets = ops[len(ops)-1:]
		}
	}
	for _, p := range targets {
		if guard.IsRCFile(c.st.cfg, c.st.home, p) {
			return c.block(rcWriteReason, "rc-redirect")
		}
	}
	return nil
}

// writeFlags holds the commands that write only with one of these
// arguments; without one, they read whatever paths they name.
var writeFlags = map[string]func(string) bool{
	"yq": func(v string) bool {
		flag, _, _ := strings.Cut(v, "=")
		return shortCluster(flag, "is") || strings.HasPrefix(v, "--inplace") || strings.HasPrefix(v, "--split-exp")
	},
	"find": func(v string) bool {
		return v == "-delete" || v == "-fls" || strings.HasPrefix(v, "-exec") || strings.HasPrefix(v, "-ok") || strings.HasPrefix(v, "-fprint")
	},
	// sort takes any unambiguous prefix of a long option (--out=x).
	"sort": func(v string) bool {
		return shortCluster(v, "o") || abbrevOf(v, "--output") || abbrevOf(v, "--compress-program")
	},
	"sg":       sgWriteFlag,
	"ast-grep": sgWriteFlag,
	"fd":       func(v string) bool { return shortCluster(v, "xX") || strings.HasPrefix(v, "--exec") },
	"tree":     func(v string) bool { return shortCluster(v, "oR") },
}

// abbrevOf is true when v, up to any "=", is long or a getopt_long prefix of it.
func abbrevOf(v, long string) bool {
	flag, _, _ := strings.Cut(v, "=")
	return len(flag) > 2 && strings.HasPrefix(long, flag)
}

func sgWriteFlag(v string) bool {
	return shortCluster(v, "Ui") || v == "--update-all" || v == "--interactive"
}

// writes is false only for a writeFlags command run without its write
// arguments; sg new scaffolds files, so it writes too.
func (c *command) writes() bool {
	flag, ok := writeFlags[c.name]
	if !ok {
		return true
	}
	if ops := c.operands(); (c.name == "sg" || c.name == "ast-grep") && len(ops) > 0 && ops[0] == "new" {
		return true
	}
	return slices.ContainsFunc(c.values(), flag)
}

// fileWriters are the commands whose operands include the files they write.
var fileWriters = nameSet("cp", "mv", "tee", "install", "ln", "touch", "mkdir", "rm", "rsync", "truncate", "dd", "patch",
	"chmod", "chown", "unlink", "sponge", "ed", "ex", "vi", "vim", "nvim")

// nameSet is names as a set.
func nameSet(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// overlayWrite asks when a known writer names a kit switch
// (guard.KitSwitch): a fileWriters command, an inPlaceScript, a writeFlags command
// with its flags, or git through checkout/restore/apply/mv/rm/stash/reset.
// Any other command naming one goes through, logged as a kit-switch shadow:
// most only read, and guessing their operands asked on every log query.
func (c *command) overlayWrite() {
	if !c.writesFiles() {
		return
	}
	_, flagged := writeFlags[c.name]
	known := fileWriters[c.name] || flagged || c.name == "git" || c.inPlaceScript()
	for _, v := range c.values() {
		_, after, hasEq := strings.Cut(v, "=")
		if known {
			if c.st.askKitSwitch(v) || hasEq && c.st.askKitSwitch(after) {
				return
			}
			continue
		}
		if c.st.kitSwitch(v) != "" || hasEq && c.st.kitSwitch(after) != "" {
			if !c.st.shadowed {
				c.st.shadowed = true
				c.st.h.Shadow("kit-switch")
			}
			return
		}
	}
}

// writesFiles is whether overlayWrite reads the command's operands as
// files it may write. A cd the guard follows writes nothing, and later
// paths resolve against where it went; an untracked one would resolve them
// wrongly, so one naming a kit switch asks here.
func (c *command) writesFiles() bool {
	switch {
	case !c.writes(), readers[c.name]:
		return false
	case c.name == "git":
		all := append([]string{c.name}, c.values()...)
		return gitWriters.MatchString(" " + strings.Join(all, " ") + " ")
	case c.name == "cd" && c.alone && c.st.tracks(c.args):
		return false
	case c.name == "cd" || c.name == "pushd":
		if i := slices.IndexFunc(c.values(), func(v string) bool { return c.st.kitSwitch(v) != "" }); i >= 0 {
			what := c.st.kitSwitch(c.values()[i])
			c.st.ask("this moves into "+what+" where the guard can't follow later paths, so a write there would go unchecked; confirm the change", "kit-switch")
		}
		return false
	}
	return true
}

// inPlaceScript is true for a script runner editing its file operands in
// place: perl or ruby -i, or awk loading gawk's inplace extension.
func (c *command) inPlaceScript() bool {
	values := c.values()
	switch c.name {
	case "perl", "ruby":
		return inPlace(values)
	case "awk", "gawk":
		for i, v := range values {
			if v == "-iinplace" || v == "--include=inplace" || (v == "-i" || v == "--include") && i+1 < len(values) && values[i+1] == "inplace" {
				return true
			}
		}
	}
	return false
}

func (c *command) rm() error {
	// Whole words only: rm -rf *.log and rm -rf dist/* are asked, not blocked.
	force, recursive, broad := false, false, false
	for _, v := range c.values() {
		switch {
		case v == "--recursive":
			force, recursive = true, true
		case v == "--force":
			force = true
		case strings.HasPrefix(v, "--"):
		case strings.HasPrefix(v, "-"):
			force = force || strings.ContainsAny(v, "rRfF")
			recursive = recursive || strings.ContainsAny(v, "rR")
		case slices.Contains([]string{"/", "/*", "~", "~/", "~/*", "$HOME", "${HOME}", "$HOME/", "${HOME}/", "$HOME/*", "${HOME}/*", ".", "..", "./", "../", "*"}, v):
			broad = true
		}
	}
	if force && broad {
		return c.block("rm with recursive force against root, home, or cwd", "rm-recursive")
	}
	if recursive && (c.st.linksBefore(c.st.at) || !c.st.allScratch(nonOptionsOf(c.args, func(w Word) string { return w.Value }))) {
		c.st.ask("rm -r deletes a directory tree; confirm the path", "rm-recursive")
	}
	return nil
}

// rsync asks when it deletes outside scratch: --del and --delete* in its
// destination, --remove-source-files in its sources. An option after an
// operand asks too, since its value could pass for the destination.
func (c *command) rsync() error {
	ops, deletes, removesSources, late := rsyncArgs(c.args)
	if !deletes && !removesSources {
		return nil
	}
	if late || len(ops) < 2 ||
		deletes && !c.st.allScratch(ops[len(ops)-1:]) ||
		removesSources && !c.st.allScratch(ops[:len(ops)-1]) {
		c.st.ask("rsync deletes files outside scratch; confirm the path", "rsync-delete")
	}
	return nil
}

// rsyncArgs is rsync's operands, whether it deletes in the destination or
// removes its sources, and whether an option came after an operand.
func rsyncArgs(args []Word) (ops []Word, deletes, removesSources, late bool) {
	ended := false
	for _, w := range args {
		switch v := w.Value; {
		case !ended && v == "--":
			ended = true
		case !ended && strings.HasPrefix(v, "-"):
			late = late || len(ops) > 0
			deletes = deletes || v == "--del" || strings.HasPrefix(v, "--delete")
			removesSources = removesSources || v == "--remove-source-files" || v == "--remove-sent-files"
		default:
			ops = append(ops, w)
		}
	}
	return ops, deletes, removesSources, late
}

// linksBefore is true when an ln in the command can run before or beside
// segment at. A link it makes isn't on disk yet for scratchTarget to
// follow. A segment in a loop, a function body, a branch that may not run,
// or a subshell (one backgrounded with & included) can follow any ln.
func (st *state) linksBefore(at int) bool {
	anywhere := st.segs[at].loop != 0 || st.segs[at].conditional || st.segs[at].subshell
	for i, seg := range st.segs {
		if i > at && !anywhere {
			break
		}
		if slices.ContainsFunc(seg.Calls, func(call Call) bool {
			j := lead(call.Words)
			return j < len(call.Words) && baseName(call.Words[j].Value) == "ln"
		}) {
			return true
		}
	}
	return false
}

// WriteTargets is what cmd, run from cwd, writes, each path absolute: every
// > and >> target, a redirect alone and a compound command's included,
// tee's files, the destination of cp, mv, or install, and the files sed -i
// or sd edits. A cd moves where the writes after it land; one ending a
// pipeline moves zsh but not bash, so each write after it counts in both
// directories.
func WriteTargets(cmd, home, cwd, shell string) []string {
	segs, _ := parseAll(cmd, shell)
	st := &state{dirs: dirs{home: home, cwd: resolveDir(home, "/", cwd)}}
	st.begin(cmd, segs)
	w := &dirSet{st: st, ds: []dirs{st.dirs}}
	for at, seg := range segs {
		w.leave(seg)
		st.at = at
		for ci, call := range seg.Calls {
			w.call(seg, ci, call)
		}
		st.track(at, seg)
	}
	return w.out
}

// dirSet is WriteTargets' walk: ds are the directories the shell may be
// in, saved holds the ones from before a cd moved a subshell, for when it
// ends, and out is every write target found.
type dirSet struct {
	st    *state
	ds    []dirs
	saved []scopeDirs
	out   []string
}

// scopeDirs is the directories a subshell scope started in.
type scopeDirs struct {
	scope int
	ds    []dirs
}

// leave restores the directories of every scope seg is no longer in.
func (w *dirSet) leave(seg Segment) {
	for len(w.saved) > 0 && !slices.Contains(seg.scopes, w.saved[len(w.saved)-1].scope) {
		w.ds, w.saved = w.saved[len(w.saved)-1].ds, w.saved[:len(w.saved)-1]
	}
}

// addIn adds p, resolved in d, once.
func (w *dirSet) addIn(d dirs, p string) {
	if p := resolveDir(w.st.pinnedHome(), d.cwd, w.st.expandPins(unquote(p))); !slices.Contains(w.out, p) {
		w.out = append(w.out, p)
	}
}

// add adds each p in every directory the shell may be in.
func (w *dirSet) add(ps ...string) {
	for _, p := range ps {
		for _, d := range w.ds {
			w.addIn(d, p)
		}
	}
}

// call adds what one command of seg, the ci-th, writes, or moves the
// directories for a cd.
func (w *dirSet) call(seg Segment, ci int, call Call) {
	w.add(redirectFiles(call.Redirs)...)
	i := lead(call.Words)
	if i >= len(call.Words) {
		return
	}
	name, args := baseName(call.Words[i].Value), values(call.Words[i+1:])
	ops := nonOptions(args)
	switch {
	case name == "tee":
		w.add(ops...)
	case name == "cp" || name == "mv" || name == "install":
		for _, d := range w.ds {
			isDir := func(p string) bool { return fsx.IsDir(resolveDir(w.st.home, d.cwd, unquote(p))) }
			for _, op := range copyTargets(args, isDir) {
				w.addIn(d, op)
			}
		}
	case name == "sd" && len(ops) > 2:
		w.add(ops[2:]...)
	case name == "sed" && inPlace(args):
		w.add(sedFiles(args)...)
	case name == "cd" && movesShell(seg, ci):
		w.cd(seg, w.st.pinWords(call.Words[i+1:]))
	}
}

// redirectFiles is each redirect target that is a file, not a descriptor.
func redirectFiles(redirs []Redir) []string {
	var out []string
	for _, r := range redirs {
		if !strings.HasPrefix(r.Target.Value, "&") {
			out = append(out, r.Target.Value)
		}
	}
	return out
}

// cd moves every directory the shell may be in by a cd to args, saving
// them first when it opens a subshell's move. One in a pipeline moves zsh
// but not bash, so both stay.
func (w *dirSet) cd(seg Segment, args []Word) {
	if inner := len(seg.scopes) - 1; inner >= 0 && (len(w.saved) == 0 || w.saved[len(w.saved)-1].scope != seg.scopes[inner]) {
		w.saved = append(w.saved, scopeDirs{seg.scopes[inner], slices.Clone(w.ds)})
	}
	moved := slices.Clone(w.ds)
	for j := range moved {
		moved[j].cd(args)
	}
	if len(seg.Calls) > 1 {
		moved = append(slices.Clone(w.ds), moved...)
	}
	w.ds = moved
}

// copyTargets is what a cp, mv, or install with args writes: each source's
// name in a -t or --target-directory directory, or in a last operand that
// is a directory (ending in / or one that exists), else that last operand.
func copyTargets(args []string, isDir func(string) bool) []string {
	var dir string
	for i, a := range args {
		switch {
		case (a == "-t" || a == "--target-directory") && i+1 < len(args):
			dir = args[i+1]
		case strings.HasPrefix(a, "--target-directory="):
			dir = strings.TrimPrefix(a, "--target-directory=")
		}
	}
	ops := nonOptions(args)
	switch {
	case dir != "":
		// nonOptions keeps -t's value as an operand.
		ops = slices.DeleteFunc(ops, func(op string) bool { return op == dir })
	case len(ops) < 2:
		return nil
	default:
		dir, ops = ops[len(ops)-1], ops[:len(ops)-1]
		if !strings.HasSuffix(dir, "/") && !isDir(dir) {
			return []string{dir}
		}
	}
	out := make([]string, 0, len(ops))
	for _, src := range ops {
		out = append(out, filepath.Join(dir, filepath.Base(unquote(src))))
	}
	return out
}

// sedFiles is the files sed's args name: not a script given with -e,
// --expression, -f, or --file, nor BSD's empty suffix after -i,
// nor, with no script flag, the first operand, which is the script.
func sedFiles(args []string) []string {
	var files []string
	scripted, done := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case done:
			files = append(files, a)
		case a == "--":
			done = true
		case strings.HasPrefix(a, "-"):
			script, skip := sedOption(a, args[i+1:])
			scripted, i = scripted || script, i+skip
		default:
			files = append(files, a)
		}
	}
	if !scripted && len(files) > 0 {
		files = files[1:]
	}
	return files
}

// sedOption reads sed's option a, rest the words after it: whether it
// gives the script, and how many of rest it takes. BSD's -i takes the
// suffix as the next word: -i ”, -Ei ”.
func sedOption(a string, rest []string) (script bool, skip int) {
	switch {
	case a == "-e" || a == "--expression" || a == "-f" || a == "--file":
		return true, 1
	case strings.HasPrefix(a, "--expression=") || strings.HasPrefix(a, "--file="):
		return true, 0
	case !strings.HasPrefix(a, "--") && (strings.HasPrefix(a, "-e") || strings.HasPrefix(a, "-f")) && len(a) > 2:
		return true, 0
	case !strings.HasPrefix(a, "--") && strings.HasSuffix(a, "i") && len(rest) > 0 && rest[0] == "":
		return false, 1
	}
	return false, 0
}

// inPlace is true when sed's, perl's, or ruby's args edit the file in
// place: a flag holding i before any --.
func inPlace(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "i") || a == "--in-place" || strings.HasPrefix(a, "--in-place=") {
			return true
		}
	}
	return false
}
