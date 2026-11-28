// Package bashguard is guard-bash: it blocks genuinely destructive shell
// commands that permission rules can't express, on a real bash syntax tree
// (mvdan.cc/sh) instead of hand-rolled splitting.
package bashguard

import (
	"errors"
	"regexp"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Segments parses src and returns every pipeline in it, substitution and
// process-substitution bodies included (inner ones first, as they run
// first), and heredoc bodies fed to a shell. On a parse error, statements
// completed before it are kept, the rest of its line is parsed again from
// the next separator, and the lines after it are parsed again as a script
// of their own, so an unterminated heredoc's body is checked as commands.
// shell is the one the Bash tool runs src in.
func Segments(src, shell string) []Segment {
	segs, _ := parseAll(src, shell)
	return segs
}

// parseAll is Segments, also reporting whether a line failed to parse short
// of the end of input. mvdan rejects a few constructs bash runs ($((cmd) ),
// an unquoted associative index), and every statement on that line goes
// unchecked, so the caller fails closed. A trap whose action can't be
// placed counts too, and so does a zsh expansion that runs a command.
func parseAll(src, shell string) ([]Segment, bool) {
	p := parseFull(src, shell)
	return p.segs, p.unparsed || p.unplaced || p.runsCode
}

// parsed is parseAll's result with its causes apart.
type parsed struct {
	segs     []Segment
	unparsed bool // a line failed to parse
	unplaced bool // a trap whose action the guard can't place in the command
	runsCode bool // a zsh expansion that runs a command, see zshRunsCode
}

// parseFull is parseAll with its causes apart.
func parseFull(src, shell string) parsed {
	c := &collector{lang: shellLang(shell)}
	c.parse(src, 0)
	// A trap's action runs when the trap fires, after the command at the
	// latest, so its segments come last, and may not run.
	c.conditional++
	for len(c.traps) > 0 {
		t := c.traps[0]
		c.traps = c.traps[1:]
		c.parse(t.action, 1)
		// Fired mid-command, the action runs where the command is then; with
		// no directory change in either, that's where it's checked.
		c.unplaced = c.unplaced || t.midway && changesDir(c.segs)
	}
	return parsed{c.segs, c.unparsed, c.unplaced, c.runsCode}
}

// changesDir is whether any call in segs moves the shell's directory.
func changesDir(segs []Segment) bool {
	return slices.ContainsFunc(segs, func(seg Segment) bool {
		return slices.ContainsFunc(seg.Calls, func(call Call) bool {
			i := lead(call.Words)
			return i < len(call.Words) && movesDir(baseName(call.Words[i].Value))
		})
	})
}

// shellLang is the language a shell, a path or a name, parses src in: zsh
// as zsh, so its glob qualifiers and expansion flags read as written, and
// every other shell as bash.
func shellLang(shell string) syntax.LangVariant {
	if baseName(shell) == "zsh" {
		return syntax.LangZsh
	}
	return syntax.LangBash
}

// safeQualifier is a zsh glob qualifier whose every flag runs nothing: N,
// a file type (. / @), - to follow links, an o or O sort key, and [n].
// Anything else, e:cmd: and +fn among them, may run a command.
var safeQualifier = regexp.MustCompile(`^\((?:[N./@-]|[oO][nLlamcdN]|\[-?[0-9]+\])+\)$`)

// zshRunsCode is true when a word in f holds a zsh expansion that may run a
// command as it expands: a glob qualifier safeQualifier doesn't match
// (mvdan keeps each as a literal opening with "("), a ${(e)x} flag, or
// ${~x}, whose value may hold such a qualifier.
func zshRunsCode(f *syntax.File) bool {
	found := false
	syntax.Walk(f, func(n syntax.Node) bool {
		switch v := n.(type) {
		case *syntax.Word:
			found = found || slices.ContainsFunc(v.Parts, func(p syntax.WordPart) bool {
				lit, ok := p.(*syntax.Lit)
				return ok && strings.HasPrefix(lit.Value, "(") && !safeQualifier.MatchString(lit.Value)
			})
		case *syntax.ParamExp:
			found = found || v.Flags != nil && strings.Contains(v.Flags.Value, "e") || v.GlobSubst != syntax.OptUnset
		}
		return !found
	})
	return found
}

type collector struct {
	lang     syntax.LangVariant // of the parse in progress
	segs     []Segment
	depth    int // of the parse in progress: nested heredocs and env -S strings
	subshell int // how many subshells the walk is inside
	// scopes are the ids of those subshells, outermost first, and scopeIDs
	// the ids handed out.
	scopes   []int
	scopeIDs int
	// conditional is how many places the walk is inside that may not run:
	// the right of && or ||, and any compound (if, case, a function body).
	conditional int
	loop        int // the outermost loop the walk is inside, 0 for none
	loops       int // loop ids handed out
	unparsed    bool
	unplaced    bool   // a trap set whose action can't be placed
	runsCode    bool   // a zsh expansion that may run a command
	traps       []trap // the actions trap commands set, parsed last
}

// trap is an action a trap command sets, and whether a termination signal
// fires it mid-command.
type trap struct {
	action string
	midway bool
}

// parseAs is parse in another shell's language: a string or heredoc that
// shell runs.
func (c *collector) parseAs(src string, depth int, lang syntax.LangVariant) {
	saved := c.lang
	c.lang = lang
	defer func() { c.lang = saved }()
	c.parse(src, depth)
}

func (c *collector) parse(src string, depth int) {
	if strings.TrimSpace(src) == "" {
		return
	}
	if depth > 8 {
		c.unparsed = true
		return
	}
	saved := c.depth
	c.depth = depth
	defer func() { c.depth = saved }()
	file, err := syntax.NewParser(syntax.Variant(c.lang)).Parse(strings.NewReader(src), "")
	if file != nil && c.lang == syntax.LangZsh && zshRunsCode(file) {
		c.runsCode = true
	}
	if err == nil {
		c.stmts(src, file.Stmts)
		return
	}
	c.parseAround(src, depth, file, err)
}

// parseAround reads what a failed parse of src leaves runnable: the
// statements before the error, the rest of its line, and the lines after.
// An incomplete command (an unclosed quote at the end) doesn't run in bash
// either.
func (c *collector) parseAround(src string, depth int, file *syntax.File, err error) {
	if !syntax.IsIncomplete(err) {
		c.unparsed = true
	}
	line, offset := errorPos(err)
	if file != nil {
		// Every statement completed before the error, same line included:
		// rm -rf ~ in `rm -rf ~; echo $((ls) )`.
		var before []*syntax.Stmt
		for _, s := range file.Stmts {
			if int(s.End().Offset()) <= offset {
				before = append(before, s)
			}
		}
		c.stmts(src, before)
	}
	// The rest of the error line from its next separator on: rm -rf ~ in
	// `echo $((ls) ); rm -rf ~`. A separator inside quotes can only add a
	// false match, never hide a command.
	if !syntax.IsIncomplete(err) && offset < len(src) {
		tail := src[offset:]
		if nl := strings.IndexByte(tail, '\n'); nl >= 0 {
			tail = tail[:nl]
		}
		if i := strings.IndexAny(tail, ";&|"); i >= 0 {
			c.parse(tail[i+1:], depth)
		}
	}
	// A reparse is a sibling, so it keeps depth: each runs on shorter input,
	// so the recursion ends however many lines fail.
	if rest := afterLine(src, max(line, 1)); rest != "" {
		c.parse(rest, depth)
	}
}

// errorPos is the line and offset of a parse error; line 1, offset 0 when
// the error carries no position.
func errorPos(err error) (line, offset int) {
	if pe, ok := errors.AsType[syntax.ParseError](err); ok {
		return int(pe.Pos.Line()), int(pe.Pos.Offset())
	}
	return 1, 0
}

// afterLine is src from the line after line on.
func afterLine(src string, line int) string {
	for range line {
		nl := strings.IndexByte(src, '\n')
		if nl < 0 {
			return ""
		}
		src = src[nl+1:]
	}
	return src
}

func (c *collector) stmts(src string, stmts []*syntax.Stmt) {
	for _, s := range stmts {
		c.stmt(src, s)
	}
}

func (c *collector) stmt(src string, s *syntax.Stmt) {
	if s == nil {
		return
	}
	if s.Background {
		// bash runs a statement ending in & in a subshell of its own.
		c.openSubshell()
		defer c.closeSubshell()
	}
	if bin, ok := s.Cmd.(*syntax.BinaryCmd); ok && (bin.Op == syntax.AndStmt || bin.Op == syntax.OrStmt) {
		c.stmt(src, bin.X)
		c.conditional++
		c.stmt(src, bin.Y)
		c.conditional--
		return
	}
	parts := pipeline(s)
	if parts == nil {
		// A compound command: its own statements, and any substitution in
		// its words (for items, case subjects, [[ ]] operands).
		c.conditional++
		c.inner(src, s.Cmd)
		c.conditional--
		c.compoundRedirs(src, s)
		return
	}

	seg := Segment{src: src, end: segmentEnd(parts[len(parts)-1]), subshell: c.subshell > 0, scopes: slices.Clone(c.scopes), conditional: c.conditional > 0, loop: c.loop}
	for _, p := range parts {
		call, ok := p.Cmd.(*syntax.CallExpr)
		if p.Cmd == nil {
			// A redirect alone (> f, true | > f) is a call with no words,
			// as x=1 > f is, so its target gets every redirect check.
			call, ok = &syntax.CallExpr{}, true
		}
		var exports []string
		keyword, opaque := "", false
		if decl, isDecl := p.Cmd.(*syntax.DeclClause); isDecl {
			call, ok = c.declCall(src, decl), true
			exports, keyword, opaque = exportNames(decl), decl.Variant.Value, declOpaque(decl)
		}
		if !ok {
			// ( ... ) or { ... } inside a pipeline: its statements and its
			// redirects are segments of their own.
			c.conditional++
			c.inner(src, p.Cmd)
			c.conditional--
			c.compoundRedirs(src, p)
			continue
		}
		cl := c.call(src, p, call)
		cl.Exports, cl.decl, cl.declOpaque, cl.end = exports, keyword, opaque, segmentEnd(p)
		seg.Calls = append(seg.Calls, cl)
	}
	if len(seg.Calls) > 0 {
		c.segs = append(c.segs, seg)
	}
	c.shellHeredocs(seg)
}

// compoundRedirs is a compound command's own redirects ({ cat; } < .env,
// done > f) as a call with no words in a segment of its own, the shape a
// redirect alone has, so each gets the same read and write checks.
func (c *collector) compoundRedirs(src string, s *syntax.Stmt) {
	if len(s.Redirs) == 0 {
		return
	}
	c.segs = append(c.segs, Segment{src: src, end: segmentEnd(s), subshell: c.subshell > 0, scopes: slices.Clone(c.scopes), loop: c.loop,
		Calls: []Call{c.call(src, s, &syntax.CallExpr{})}})
}

// segmentEnd is where a pipeline ends: its last command's last word or
// redirect, not the statement's end, which takes in a trailing ; or &. A
// stage that is a redirect alone (true | >f) has no command.
func segmentEnd(last *syntax.Stmt) int {
	end := int(last.Pos().Offset())
	if last.Cmd != nil {
		end = int(last.Cmd.End().Offset())
	}
	for _, r := range last.Redirs {
		if r.Hdoc == nil {
			end = max(end, int(r.End().Offset()))
		}
	}
	return end
}

// shellHeredocs parses seg's heredocs as commands when a shell in it reads
// them (bash <<EOF, cat <<EOF | sh), past any wrapper (sudo bash <<EOF),
// or a root shell does (sudo -s <<EOF).
// A body parses in the language of the first shell named, bash for a root
// shell.
func (c *collector) shellHeredocs(seg Segment) {
	lang, found := syntax.LangBash, false
	for _, cl := range seg.Calls {
		if i := lead(cl.Words); i < len(cl.Words) && shells[baseName(cl.Words[i].Value)] {
			lang, found = shellLang(cl.Words[i].Value), true
			break
		}
		found = found || startsRootShell(cl.Words)
	}
	if !found {
		return
	}
	for _, cl := range seg.Calls {
		for _, body := range cl.Heredocs {
			c.parseAs(body, c.depth+1, lang)
		}
	}
}

// declCall is export R=rm (or declare, local, readonly, typeset) as the bare
// assignment R=rm it makes, so $R resolves the same. A flag or a name with
// no = assigns nothing; its substitutions are still collected.
func (c *collector) declCall(src string, decl *syntax.DeclClause) *syntax.CallExpr {
	call := &syntax.CallExpr{}
	for _, a := range decl.Args {
		if a.Naked {
			c.inner(src, a)
			continue
		}
		call.Assigns = append(call.Assigns, a)
	}
	return call
}

// call reads one simple command of a pipeline: its words, assignments, and
// redirects, parsing any string env -S runs and any substitution in them.
func (c *collector) call(src string, p *syntax.Stmt, call *syntax.CallExpr) Call {
	for _, a := range call.Assigns {
		c.inner(src, a)
	}
	for _, w := range call.Args {
		c.inner(src, w)
	}
	for _, r := range p.Redirs {
		c.inner(src, r.Word)
	}
	var cl Call
	cl.Assigns, cl.plainValues = assignsOf(src, call.Assigns)
	if len(call.Args) > 0 {
		cl.start = int(call.Args[0].Pos().Offset())
	} else {
		cl.start = int(p.Pos().Offset())
	}
	for _, w := range call.Args {
		cl.Words = append(cl.Words, Word{wordValue(src, w), src[w.Pos().Offset():w.End().Offset()], int(w.End().Offset()), literal(w.Parts)})
	}
	c.embedded(&cl)
	cl.Redirs = append(cl.Redirs, timeOutputs(cl.Words)...)
	for _, r := range p.Redirs {
		c.redirect(src, r, &cl)
	}
	return cl
}

// assignsOf is assigns as NAME=value, and the offsets of each literal value.
func assignsOf(src string, assigns []*syntax.Assign) (out []string, plain [][2]int) {
	for _, a := range assigns {
		if a.Name == nil {
			continue
		}
		value := ""
		if a.Value != nil {
			value = wordValue(src, a.Value)
			if literal(a.Value.Parts) {
				plain = append(plain, [2]int{int(a.Value.Pos().Offset()), int(a.Value.End().Offset())})
			}
		}
		out = append(out, a.Name.Value+"="+value)
	}
	return out, plain
}

// embedded parses every command string cl's words run (env -S, watch,
// flock, a shell's -c, git rebase -x) and keeps a trap's action, which
// parseFull reads after the command.
func (c *collector) embedded(cl *Call) {
	if cmd, ok := envSplitString(cl.Words); ok {
		c.parse(cmd, c.depth+1)
	}
	if cmd, ok := watchString(cl.Words); ok {
		c.parse(cmd, c.depth+1)
	}
	if cmd, ok := flockString(cl.Words); ok {
		c.parse(cmd, c.depth+1)
	}
	if cmd, ok := shellCString(cl.Words); ok {
		// A child shell: a cd in its string moves nothing after the call.
		cl.shellString = true
		c.openSubshell()
		c.parseAs(cmd, c.depth+1, shellLang(cl.Words[lead(cl.Words)].Value))
		c.closeSubshell()
	}
	if action, readable, midway, ok := trapAction(cl.Words); ok {
		c.traps = append(c.traps, trap{action, midway})
		// A trap in a subshell or a nested shell fires when that exits.
		c.unplaced = c.unplaced || !readable || c.depth > 0 || c.subshell > 0
	}
	for _, cmd := range rebaseExecs(cl.Words) {
		c.parse(cmd, c.depth+1)
	}
}

// redirect records r on cl: a heredoc's body, a < input, or a > target.
func (c *collector) redirect(src string, r *syntax.Redirect, cl *Call) {
	if r.Hdoc != nil {
		body := src[r.Hdoc.Pos().Offset():r.Hdoc.End().Offset()]
		// The slice ends with the terminator line; it isn't body.
		if nl := strings.LastIndexByte(body, '\n'); nl >= 0 && r.Word != nil &&
			strings.TrimLeft(body[nl+1:], "\t") == wordValue(src, r.Word) {
			body = body[:nl]
		}
		cl.Heredocs = append(cl.Heredocs, body)
		c.inner(src, r.Hdoc)
		return
	}
	if r.Word == nil {
		return
	}
	value := wordValue(src, r.Word)
	switch op := r.Op.String(); {
	case r.Op == syntax.WordHdoc: // bash <<< "rm -rf ~" runs its word
		cl.Heredocs = append(cl.Heredocs, value)
	case r.Op == syntax.RdrIn:
		cl.Inputs = append(cl.Inputs, value)
	case strings.Contains(op, ">"):
		// >&2 duplicates an fd and >&- closes one: the target is "&2" or
		// "&-", never a file name. >& file writes file, like &>.
		if r.Op == syntax.DplOut && fdTarget.MatchString(value) {
			value = "&" + value
		}
		cl.Redirs = append(cl.Redirs, Redir{op, Word{value, src[r.Word.Pos().Offset():r.Word.End().Offset()], int(r.Word.End().Offset()), literal(r.Word.Parts)}})
	}
}
