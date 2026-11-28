package bashguard

import (
	"regexp"
	"slices"
	"strings"
)

// plainWord is a word with no shell syntax in it.
var plainWord = regexp.MustCompile(`^[A-Za-z0-9_./%:=+@,-]+$`)

// mktempDir is true for a whole $(mktemp ...) call with -d or --directory
// and only plain words, so nothing else runs in it.
func mktempDir(value string) bool {
	if !strings.HasPrefix(value, "$(") || !strings.HasSuffix(value, ")") {
		return false
	}
	inner := value[2 : len(value)-1]
	words := strings.Fields(inner)
	if len(words) < 2 || words[0] != "mktemp" || slices.ContainsFunc(words, func(w string) bool { return !plainWord.MatchString(w) }) {
		return false
	}
	return slices.ContainsFunc(words[1:], func(w string) bool {
		return w == "--directory" || strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.Contains(w, "d")
	})
}

// allScratch is true when ops is non-empty and each one is a scratch
// operand.
func (st *state) allScratch(ops []Word) bool {
	return len(ops) > 0 && !slices.ContainsFunc(ops, func(w Word) bool { return !st.scratchOperand(w) })
}

// quotedVar is a whole word that is one variable in double quotes, "$S",
// "${S}", or "${S:?}", and maybe a plain path after it, "$S/x": unquoted,
// IFS could split it into other paths. Its groups are the name (twice),
// the :?, and the path.
var quotedVar = regexp.MustCompile(`^"\$(?:(` + varName + `)|\{(` + varName + `)(:\?)?\})(/[^"$` + "`" + `\\]*)?"$`)

// bashBuiltins is every bash builtin. Only a builtin can set a variable of
// the shell running the command; any other command runs in a child.
var bashBuiltins = nameSet(".", ":", "[", "alias", "bg", "bind", "break", "builtin", "caller", "cd", "command",
	"compgen", "complete", "compopt", "continue", "declare", "dirs", "disown", "echo", "enable", "eval", "exec",
	"exit", "export", "false", "fc", "fg", "getopts", "hash", "help", "history", "jobs", "kill", "let", "local",
	"logout", "mapfile", "popd", "printf", "pushd", "pwd", "read", "readarray", "readonly", "return", "set",
	"shift", "shopt", "source", "suspend", "test", "times", "trap", "true", "type", "typeset", "ulimit", "umask",
	"unalias", "unset", "wait")

// inertBuiltins are the builtins that set no variable a caller names and
// run no code from a string; every other builtin can (printf -v "$N",
// trap "$C" DEBUG, wait -p "$N").
var inertBuiltins = nameSet(":", "[", "break", "cd", "continue", "dirs", "echo", "exit", "false", "popd", "pushd",
	"pwd", "return", "set", "shift", "test", "true", "type")

// scratchOperand is true for a scratch target, quoted when it expands
// anything, or a quoted scratch variable, with a path after it with no ..
// only when it can't be empty there, since an empty S makes "$S/x" /x.
func (st *state) scratchOperand(w Word) bool { return st.scratchWord(w, true) }

// scratchWord is scratchOperand, and when bareEmpty is false, a bare "$S"
// must not be empty either, as an empty git -C is the cwd.
func (st *state) scratchWord(w Word, bareEmpty bool) bool {
	quoted := strings.HasPrefix(w.Raw, `"`) && strings.HasSuffix(w.Raw, `"`)
	if m := quotedVar.FindStringSubmatch(w.Raw); m != nil && !strings.Contains(m[4], "..") && st.scratchVar(m[1]+m[2], 1) &&
		(m[4] == "" && bareEmpty || st.nonEmpty(m[1]+m[2], m[3] != "", st.at)) {
		path, fresh := st.varPath(m[1]+m[2], 1)
		return fresh || path != "" && st.onDisk(path+m[4], false)
	}
	v := st.pinWords([]Word{w})[0].Value
	return !varRef.MatchString(v) && (quoted || !strings.Contains(v, "$")) && st.scratchTarget(v, false)
}

// cdPwd is a value that prints a variable's directory with its links
// resolved, $(cd "$T" && pwd -P), or pwd alone; its groups are the name,
// once per form.
var cdPwd = regexp.MustCompile(`^\$\(cd "\$(?:(` + varName + `)|\{(` + varName + `)\})" && pwd(?: -P)?\)$`)

// varPath is the absolute path scratch variable name holds, so the path
// below it gets the symlink check: its value as scratchPath reads it, or,
// up to depth levels, an earlier scratch variable's path and what follows
// it. "" when it can't be read. fresh is true for a mktemp -d dir, which
// holds no link but one the command makes, and rm asks beside ln.
func (st *state) varPath(name string, depth int) (path string, fresh bool) {
	value := st.sure[name]
	if mktempDir(value) {
		return "", true
	}
	if m := cdPwd.FindStringSubmatch(value); m != nil {
		if depth == 0 {
			return "", false
		}
		// cd follows a final link, so its target must be in scratch too.
		path, fresh := st.varPath(m[1]+m[2], depth-1)
		if fresh || path != "" && st.onDisk(path, true) {
			return path, fresh
		}
		return "", false
	}
	if m := quotedVar.FindStringSubmatch(`"` + value + `"`); m != nil {
		if depth == 0 {
			return "", false
		}
		base, fresh := st.varPath(m[1]+m[2], depth-1)
		if fresh || base == "" {
			return "", fresh
		}
		return base + m[4], false
	}
	if p, ok := st.scratchPath(value); ok {
		return p, false
	}
	return "", false
}

// scratchVar is true for a variable the current shell surely sets
// (st.sure) once, outside any loop, and names nowhere else in the command
// (read S, printf -v S, declare S=/ all set it too), to a scratch target,
// a fresh mktemp -d dir, or, up to depth levels, a quoted scratch variable
// set before it, and not empty there, with a path after it (H="$S/x").
func (st *state) scratchVar(name string, depth int) bool {
	if st.inLoop(name) {
		return false
	}
	value, ok := st.sureValue(name, st.at)
	if !ok || !st.setOnly(name) {
		return false
	}
	if mktempDir(value) || st.scratchTarget(value, false) {
		return true
	}
	if m := cdPwd.FindStringSubmatch(value); m != nil {
		return st.cdPwdScratch(name, m[1]+m[2], depth)
	}
	// value has its quotes removed: H="$S/x", H=$S/x, and H='$S/x' all
	// read $S/x. An assignment doesn't split its value, so the first two
	// are safe; the command's one H= (setOnly) shows which this was.
	m := quotedVar.FindStringSubmatch(`"` + value + `"`)
	if depth == 0 || m == nil || strings.Contains(m[4], "..") {
		return false
	}
	inner := m[1] + m[2]
	return st.setFromEarlier(name, inner) && st.scratchVar(inner, depth-1) && st.nonEmpty(inner, m[3] != "", st.sureAt[name])
}

// cdPwdScratch is scratchVar for name set to $(cd "$inner" && pwd -P):
// inner's directory, links resolved, when inner can't be empty there (cd ""
// stays put, and pwd prints the cwd) and stays in scratch with its last
// link followed, as cd follows it.
func (st *state) cdPwdScratch(name, inner string, depth int) bool {
	at, set := st.sureAt[inner]
	if depth == 0 || !set || at >= st.sureAt[name] || !st.scratchVar(inner, depth-1) || !st.nonEmpty(inner, false, st.sureAt[name]) {
		return false
	}
	path, fresh := st.varPath(name, depth)
	return fresh || path != ""
}

// errexit is whether segs leave set -e on: the last set that turns it on
// or off, outside any subshell and any part that may not run, turns it on.
func errexit(segs []Segment) bool {
	on := false
	for _, seg := range segs {
		if seg.subshell || seg.conditional {
			continue
		}
		for _, call := range seg.Calls {
			if len(call.Words) > 0 && call.Words[0].Value == "set" && call.Words[0].literal {
				on = setsErrexit(values(call.Words[1:]), on)
			}
		}
	}
	return on
}

// setsErrexit is whether set's words leave set -e on, given on before them.
func setsErrexit(words []string, on bool) bool {
	for i, w := range words {
		if w == "--" {
			break // the rest are positional parameters: set -- -e sets $1
		}
		switch {
		case (w == "-o" || w == "+o") && i+1 < len(words) && words[i+1] == "errexit":
			on = w == "-o"
		case strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.Contains(w, "e"):
			on = true
		case strings.HasPrefix(w, "+") && strings.Contains(w, "e"):
			on = false
		}
	}
	return on
}

// findsOnlyScratch is true when every place find starts from is scratch, as
// rm -r's operands must be, and nothing makes it follow a link out (-L,
// -H, -follow): find -delete removes only what it walks. It fails closed:
// a leading option other than -P, -D x, or -O<n> (BSD's -f, -x, -s, -E,
// -X, -d), a start list read from a file (-files0-from), or no start path
// at all (BSD needs one, GNU takes the cwd) counts as unknown.
func (c *command) findsOnlyScratch() bool {
	args := c.args
	for len(args) > 0 && strings.HasPrefix(args[0].Value, "-") {
		switch v := args[0].Value; {
		case v == "-P", strings.HasPrefix(v, "-O") && len(v) > 2:
		case v == "-D" && len(args) > 1:
			args = args[1:] // its debug options
		default:
			return false
		}
		args = args[1:]
	}
	starts := slices.IndexFunc(args, func(w Word) bool { return findExpression(w.Value) })
	if starts < 0 {
		starts = len(args)
	}
	roots := args[:starts]
	return len(roots) > 0 && !slices.ContainsFunc(c.values(), func(v string) bool {
		return v == "-L" || v == "-H" || v == "-follow" || v == "-f" || v == "-files0-from"
	}) && c.st.allScratch(roots)
}

// findExpression is whether a find word starts its expression: a test,
// action, or operator, such as -name, (, or !.
func findExpression(v string) bool {
	return v == "(" || v == "!" || v == "," || strings.HasPrefix(v, "-")
}
