package bashguard

import (
	"slices"
	"strings"

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
