package bashguard

import (
	"maps"
	"regexp"
	"slices"
	"strings"
)

// shellVars is what a command's own assignments set, as state checks it
// one segment at a time: what a variable holds, whether the shell surely
// holds it there, and what pins a path to one.
type shellVars struct {
	// segs is the whole command parsed, inner substitutions included.
	segs []Segment
	// vars is what the command's own bare assignments set (R=rm; $R -rf ~),
	// as of the segment being checked.
	vars map[string]string
	// loopVars is every value each loop's assignments set, by loop id.
	loopVars map[int]map[string][]string
	// sure is vars as the current shell surely has them: set by a lone
	// command outside any subshell and any part that may not run.
	sure map[string]string
	// sureAt is the segment each sure value was set in, and at the segment
	// being checked.
	sureAt map[string]int
	// chained is true for a sure value set on the right of && (track).
	chained map[string]bool
	at      int
	// exported is every name an export in the command names.
	exported map[string]bool
	// cmd is the whole command, as the Bash call gives it.
	cmd string
}

// begin readies st to check segs, cmd parsed, one segment at a time.
func (st *shellVars) begin(cmd string, segs []Segment) {
	st.cmd = cmd
	st.vars, st.sure, st.sureAt, st.chained, st.loopVars, st.segs = map[string]string{}, map[string]string{}, map[string]int{}, map[string]bool{}, loopAssigned(segs), segs
	st.exported = exportedNames(segs)
}

// track records what seg, the i-th segment, assigns, once it's checked. An
// assignment on the right of && counts when every pipeline from its list's
// head to it follows an && (ls && S=x): a use later in that chain runs only
// if it ran (sureValue). One after || or in a compound doesn't.
func (st *shellVars) track(i int, seg Segment) {
	vars := assigned(seg)
	maps.Copy(st.vars, vars)
	chained := seg.conditional && st.chainedFromHead(i)
	for name, value := range vars {
		switch {
		case seg.subshell || len(seg.Calls) > 1 || seg.conditional && !chained:
			delete(st.sure, name)
		default:
			st.sure[name], st.sureAt[name], st.chained[name] = value, i, chained
		}
	}
}

// chainedFromHead is true when segment i runs only if every pipeline
// before it in its list succeeded: an && joins each pair from the nearest
// earlier pipeline that always runs.
func (st *shellVars) chainedFromHead(i int) bool {
	for head := i - 1; head >= 0; head-- {
		if seg := st.segs[head]; !seg.subshell && !seg.conditional {
			return andChain(st.segs, head, i)
		}
	}
	return false
}

// sureValue is name's sure value at segment use: one set on the right of
// && counts only where use runs in the same && chain after it.
func (st *shellVars) sureValue(name string, use int) (string, bool) {
	value, ok := st.sure[name]
	if ok && st.chained[name] && !andChain(st.segs, st.sureAt[name], use) {
		return "", false
	}
	return value, ok
}

// setVars is vars as NAME=value words, in no set order, split into those
// the command exports and the bare rest, which reach a child process only
// if the environment already exported them.
func (st *shellVars) setVars() (exported, bare []string) {
	for name, value := range st.vars {
		if st.exported[name] {
			exported = append(exported, name+"="+value)
		} else {
			bare = append(bare, name+"="+value)
		}
	}
	return exported, bare
}

// pinRefs is each $NAME or ${NAME} in a string.
var pinRefs = regexp.MustCompile(nameRef)

// pinBase is a value that starts with one $NAME or ${NAME} and has plain
// text after it: its groups are the name (twice) and the text.
var pinBase = regexp.MustCompile(`^` + nameRef + "([^$`]*)$")

// pinned is the plain path name surely holds at the segment being checked:
// set once, outside any loop, to a plainWord (nothing the shell splits,
// globs, or expands, ~ included), or to an earlier pinned variable with
// plain text after it, up to depth levels. Nothing is pinned that could be
// set unseen (setOnly), that a for or select sets, or when the command
// names IFS, which would split it.
func (st *shellVars) pinned(name string, depth int) (string, bool) {
	value, ok := st.sureValue(name, st.at)
	if !ok || st.inLoop(name) || !st.setOnly(name) {
		return "", false
	}
	cmd := st.cmd
	q := regexp.QuoteMeta(name)
	if regexp.MustCompile(`\bIFS\b|\b(?:for|select)\s+` + q + `\b`).MatchString(cmd) {
		return "", false
	}
	if m := pinBase.FindStringSubmatch(value); m != nil {
		inner := m[1] + m[2]
		if depth == 0 || !st.setFromEarlier(name, inner) {
			return "", false
		}
		base, ok := st.pinned(inner, depth-1)
		if !ok {
			return "", false
		}
		value = base + m[3]
	}
	return value, plainWord.MatchString(value)
}

// setFromEarlier is true when name's sure value was set from $inner, which
// the command surely set before it.
func (st *shellVars) setFromEarlier(name, inner string) bool {
	at, set := st.sureAt[inner]
	return set && at < st.sureAt[name] && setFrom(st.cmd, name, inner)
}

// setFrom is true when cmd sets name to $inner, bare or in double quotes:
// '$S' and \$S set the text $S.
func setFrom(cmd, name, inner string) bool {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `="?\$\{?` + regexp.QuoteMeta(inner) + `\b`).MatchString(cmd)
}

// inLoop is true when a loop in the command assigns name.
func (st *shellVars) inLoop(name string) bool {
	return slices.ContainsFunc(slices.Collect(maps.Values(st.loopVars)), func(loop map[string][]string) bool {
		_, ok := loop[name]
		return ok
	})
}

// expandPins is s with each reference to a pinned variable replaced by its
// path; any other reference stays.
func (st *shellVars) expandPins(s string) string {
	if !strings.Contains(s, "$") {
		return s
	}
	return pinRefs.ReplaceAllStringFunc(s, func(ref string) string {
		m := pinRefs.FindStringSubmatch(ref)
		if path, ok := st.pinned(m[1]+m[2], 2); ok {
			return path
		}
		return ref
	})
}

// pinWords is words with expandPins applied to each that has no single
// quote or backslash, which would keep a $ as text.
func (st *shellVars) pinWords(words []Word) []Word {
	out := slices.Clone(words)
	for i, w := range out {
		if !strings.ContainsAny(w.Raw, `'\`) {
			out[i].Value = st.expandPins(w.Value)
		}
	}
	return out
}

// nonEmpty is true when scratch variable name can't be empty at segment
// use: used as ${name:?}, which stops the shell, set to a literal path or
// to another scratch variable (scratchVar checked that one), or set by a
// bare assignment whose $(...) failing stops the command first, under an
// earlier set -e when it stands alone (set -e spares a failure left of &&
// or ||), or by an && chain through to use; never after !.
func (st *shellVars) nonEmpty(name string, colonQ bool, use int) bool {
	value := st.sure[name]
	switch {
	case colonQ, !strings.ContainsAny(value, "$`"), quotedVar.MatchString(`"` + value + `"`):
		return true
	case st.exported[name]:
		// export's own status is 0 whatever its $(...) did.
		return false
	}
	set := st.sureAt[name]
	assign := st.segs[set]
	// ! turns a failed $(...) into success, for set -e and && alike.
	if len(assign.Calls) == 0 || inverted(assign) {
		return false
	}
	if standalone(assign) && errexit(st.segs[:set]) {
		return true
	}
	// A failed $(...) fails the assignment, and every && after it skips.
	return use > set && andChain(st.segs, set, use)
}

// setOnly is true when the command names name only in one assignment and
// in $name references, outside the words and values it passes as written,
// and runs nothing that could set it unseen: no builtin outside
// inertBuiltins but printf without -v, no declaration that sets a name it
// doesn't spell, no command word it computes, and no arithmetic.
func (st *shellVars) setOnly(name string) bool {
	if arithAssigns(st.cmd) {
		return false
	}
	for _, seg := range st.segs {
		for _, call := range seg.Calls {
			if call.declOpaque {
				return false
			}
			i := lead(call.Words)
			if i >= len(call.Words) {
				continue
			}
			// A bare [ is the test builtin, though its name reads as a glob.
			if !call.Words[i].literal && call.Words[i].Raw != "[" {
				if !st.fixedProgram(call.Words[i]) {
					return false
				}
				continue
			}
			name := baseName(call.Words[i].Value)
			// A trap's action is checked as commands of its own (trapAction).
			if name != "trap" && bashBuiltins[name] && !inertBuiltins[name] && !plainPrintf(name, call.Words[i+1:]) {
				return false
			}
		}
	}
	cmd := blankWritten(st.cmd, st.segs)
	q := regexp.QuoteMeta(name)
	count := func(pattern string) int { return len(regexp.MustCompile(pattern).FindAllStringIndex(cmd, -1)) }
	sets := count(`\b` + q + `=`)
	return sets == 1 && count(`\b`+q+`\b`) == sets+count(`\$`+q+`\b|\$\{`+q+`(?::\?)?\}`)
}

// fixedProgram is true for a command word that is one variable the command
// sets once, to a plainWord naming no builtin ($KIT run-checks), or a path
// under a variable ("$V/x", or $V/x with V set so). It runs as a child
// process, which can't set the shell's variables.
func (st *shellVars) fixedProgram(w Word) bool {
	// A path one word long runs a file, never a builtin.
	if m := quotedVar.FindStringSubmatch(w.Raw); m != nil && m[4] != "" {
		return true
	}
	m := programRef.FindStringSubmatch(w.Value)
	if m == nil {
		return false
	}
	name, path := m[1]+m[2], m[3] != ""
	var values []string
	for _, seg := range st.segs {
		if value, ok := assigned(seg)[name]; ok {
			values = append(values, value)
		}
	}
	count := len(regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`=`).FindAllStringIndex(st.cmd, -1))
	return len(values) == 1 && count == 1 && plainWord.MatchString(values[0]) && (path || !bashBuiltins[baseName(values[0])])
}

// varFields is the fields w may expand to in seg when it is a variable the
// command assigned itself: the value set before seg, and in a loop every
// value the loop sets. Nil for any other word, $EDITOR included.
func (st *shellVars) varFields(seg Segment, w Word) [][]string {
	m := varRef.FindStringSubmatch(w.Value)
	if w.literal || m == nil {
		return nil
	}
	name := m[1] + m[2]
	var values []string
	if value, ok := st.vars[name]; ok {
		values = append(values, value)
	}
	values = append(values, st.loopVars[seg.loop][name]...)
	var out [][]string
	for _, v := range values {
		if fields := strings.Fields(v); len(fields) > 0 {
			out = append(out, fields)
		}
	}
	return out
}

// arithAssign is an assignment operator in an arithmetic expression: =
// alone or after an operator (+=, <<=), ++, or --, but not ==, !=, <=, >=.
var arithAssign = regexp.MustCompile(`(?:^|[^=!<>])=(?:[^=]|$)|<<=|>>=|\+\+|--`)

// arithAssigns is true when an arithmetic expression in cmd, (( )) or
// $(( )), can assign a variable, as (( $N = 0 )) does to a name the command
// never spells; a counter like $((i+1)) assigns nothing. An expression
// with no closing )) runs to the end of cmd.
func arithAssigns(cmd string) bool {
	for i := strings.Index(cmd, "(("); i >= 0; {
		if arithAssign.MatchString(cmd[i+2 : closingParen(cmd, i)]) {
			return true
		}
		next := strings.Index(cmd[i+2:], "((")
		if next < 0 {
			break
		}
		i += 2 + next
	}
	return false
}

// closingParen is the index of the ")" that closes the "(" at s[i], or
// len(s) when none does.
func closingParen(s string, i int) int {
	depth := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return j
			}
		}
	}
	return len(s)
}

// blankWritten is cmd with spaces over each call word and assigned value
// of segs that the shell passes as written: a name there is text, and no
// call setOnly allows can make it set a variable.
func blankWritten(cmd string, segs []Segment) string {
	b := []byte(cmd)
	blank := func(from, to int) {
		for i := from; i < to; i++ {
			b[i] = ' '
		}
	}
	for _, seg := range segs {
		// A heredoc's commands sit at offsets into its body.
		if seg.src != cmd {
			continue
		}
		for _, call := range seg.Calls {
			for _, w := range call.Words {
				if w.literal {
					blank(w.end-len(w.Raw), w.end)
				}
			}
			for _, span := range call.plainValues {
				blank(span[0], span[1])
			}
		}
	}
	return string(b)
}

// programRef is a command word that is one variable, $V or ${V}, alone or
// followed by a plainWord path.
var programRef = regexp.MustCompile(`^` + nameRef + `(/[A-Za-z0-9_./%:=+@,-]*)?$`)

// plainPrintf is true for printf with no -v, attached (-vNAME) or not,
// which would set a variable, and a first word with no expansion, which
// could turn into one.
func plainPrintf(name string, args []Word) bool {
	return name == "printf" && (len(args) == 0 || !strings.ContainsAny(args[0].Value, "$`")) &&
		!slices.ContainsFunc(args, func(w Word) bool { return strings.HasPrefix(w.Value, "-v") })
}
