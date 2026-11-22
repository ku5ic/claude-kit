// Package bashguard is guard-bash: it blocks genuinely destructive shell
// commands that permission rules can't express, on a real bash syntax tree
// (mvdan.cc/sh) instead of hand-rolled splitting.
package bashguard

import (
	"errors"
	"github.com/ku5ic/claude-kit/go/internal/classify"
	"regexp"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

// Word is one shell word: Value has quotes removed and escapes resolved,
// with expansions left as their source text ("$HOME/x", "$(kit scratch-dir)").
// Raw is the source.
type Word struct {
	Value string
	Raw   string
	end   int // source offset just past the word
	// literal is whether the shell passes the word as written: no
	// expansion, brace list, or glob character in any part.
	literal bool
}

// values is each word's Value.
func values(words []Word) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		out = append(out, w.Value)
	}
	return out
}

// Redir is an output redirect: its operator and target word.
type Redir struct {
	Op     string
	Target Word
}

// Call is one simple command of a pipeline.
type Call struct {
	Assigns []string // leading VAR=value words, the value's quotes removed
	Words   []Word
	Redirs  []Redir
	Inputs  []string // < redirect sources, quotes removed
	// Heredocs are the raw bodies of this command's here-documents and
	// here-strings.
	Heredocs []string
	start    int // source offset of the first word
}

// Segment is one pipeline: the commands between &&, ||, ;, &, or a newline.
type Segment struct {
	Calls []Call
	end   int // source offset where the pipeline ends
	src   string
	// subshell is a pipeline inside ( ), $( ), backticks, or <( ): a cd
	// there moves nothing after it.
	subshell bool
	// conditional is a pipeline that may not run: on the right of && or
	// ||, or inside a compound command.
	conditional bool
	// loop is the outermost for, while, or until loop holding the
	// pipeline, 0 for none: its body reruns, so an assignment anywhere in
	// it may be in effect.
	loop int
}

// Rest is the normalized source text after call's word, to the end of the
// pipeline: what the per-command regexes match on. A word index past the
// end means the text after the whole call.
func (s Segment) Rest(call, word int) string {
	c := s.Calls[call]
	start := c.start
	switch {
	case word < len(c.Words):
		start = c.Words[word].end
	case len(c.Words) > 0:
		start = c.Words[len(c.Words)-1].end
	}
	if start >= s.end || start > len(s.src) {
		return ""
	}
	return normalize(s.src[start:s.end])
}

// normalize turns tabs into spaces and squeezes runs of spaces, so every
// rule matches one spacing.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

// shells are the POSIX-family shells: a heredoc body fed to one is a
// script, and -c runs its string.
var shells = func() map[string]bool {
	m := map[string]bool{}
	for _, name := range strings.Fields("sh bash rbash zsh dash ash hush ksh mksh yash") {
		m[name] = true
	}
	return m
}()

// runsC is true when name's -c runs a string: every shell, and fish, which
// isn't in shells because its heredoc bodies don't parse as POSIX.
func runsC(name string) bool { return shells[name] || name == "fish" }

// Segments parses src and returns every pipeline in it, substitution and
// process-substitution bodies included (inner ones first, as they run
// first), and heredoc bodies fed to a shell. On a parse error, statements
// completed before it are kept, the rest of its line is parsed again from
// the next separator, and the lines after it are parsed again as a script
// of their own, so an unterminated heredoc's body is checked as commands.
func Segments(src string) []Segment {
	segs, _ := parseAll(src)
	return segs
}

// parseAll is Segments, also reporting whether a line failed to parse short
// of the end of input. mvdan rejects a few constructs bash runs ($((cmd) ),
// an unquoted associative index), and every statement on that line goes
// unchecked, so the caller fails closed.
func parseAll(src string) ([]Segment, bool) {
	c := &collector{}
	c.parse(src, 0)
	return c.segs, c.unparsed
}

type collector struct {
	segs     []Segment
	depth    int // of the parse in progress: nested heredocs and env -S strings
	subshell int // how many subshells the walk is inside
	// conditional is how many places the walk is inside that may not run:
	// the right of && or ||, and any compound (if, case, a function body).
	conditional int
	loop        int // the outermost loop the walk is inside, 0 for none
	loops       int // loop ids handed out
	unparsed    bool
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
	file, err := syntax.NewParser().Parse(strings.NewReader(src), "")
	if err == nil {
		c.stmts(src, file.Stmts)
		return
	}
	// An incomplete command (an unclosed quote at the end) doesn't run in
	// bash either.
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

	seg := Segment{src: src, end: segmentEnd(parts[len(parts)-1]), subshell: c.subshell > 0, conditional: c.conditional > 0, loop: c.loop}
	for _, p := range parts {
		call, ok := p.Cmd.(*syntax.CallExpr)
		if p.Cmd == nil {
			// A redirect alone (> f, true | > f) is a call with no words,
			// as x=1 > f is, so its target gets every redirect check.
			call, ok = &syntax.CallExpr{}, true
		}
		if decl, isDecl := p.Cmd.(*syntax.DeclClause); isDecl {
			call, ok = c.declCall(src, decl), true
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
		seg.Calls = append(seg.Calls, c.call(src, p, call))
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
	c.segs = append(c.segs, Segment{src: src, end: segmentEnd(s), subshell: c.subshell > 0, loop: c.loop,
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
func (c *collector) shellHeredocs(seg Segment) {
	if !slices.ContainsFunc(seg.Calls, func(cl Call) bool {
		i := lead(cl.Words)
		return i < len(cl.Words) && shells[baseName(cl.Words[i].Value)] || startsRootShell(cl.Words)
	}) {
		return
	}
	for _, cl := range seg.Calls {
		for _, body := range cl.Heredocs {
			c.parse(body, c.depth+1)
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
	for _, a := range call.Assigns {
		if a.Name == nil {
			continue
		}
		value := ""
		if a.Value != nil {
			value = wordValue(src, a.Value)
		}
		cl.Assigns = append(cl.Assigns, a.Name.Value+"="+value)
	}
	if len(call.Args) > 0 {
		cl.start = int(call.Args[0].Pos().Offset())
	} else {
		cl.start = int(p.Pos().Offset())
	}
	for _, w := range call.Args {
		cl.Words = append(cl.Words, Word{wordValue(src, w), src[w.Pos().Offset():w.End().Offset()], int(w.End().Offset()), literal(w.Parts)})
	}
	if cmd, ok := envSplitString(cl.Words); ok {
		c.parse(cmd, c.depth+1)
	}
	if cmd, ok := watchString(cl.Words); ok {
		c.parse(cmd, c.depth+1)
	}
	if cmd, ok := flockString(cl.Words); ok {
		c.parse(cmd, c.depth+1)
	}
	for _, cmd := range rebaseExecs(cl.Words) {
		c.parse(cmd, c.depth+1)
	}
	cl.Redirs = append(cl.Redirs, timeOutputs(cl.Words)...)
	for _, r := range p.Redirs {
		c.redirect(src, r, &cl)
	}
	return cl
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

// rebaseExecs is each command string a git rebase among words runs after
// each commit: -x cmd, -xcmd, either in a cluster such as -ix, --exec cmd,
// or --exec=cmd.
func rebaseExecs(words []Word) []string {
	i := lead(words)
	if i >= len(words) || baseName(words[i].Value) != "git" {
		return nil
	}
	sub, args, _ := classify.GitSubcommand(values(words[i+1:]))
	if sub != "rebase" {
		return nil
	}
	// git takes any unambiguous prefix of a long option: --ex is --exec.
	isExec := func(flag string) bool { return len(flag) >= len("--ex") && abbrevOf(flag, "--exec") }
	var out []string
	for j := 0; j < len(args); j++ {
		a := args[j]
		flag, value, attached := strings.Cut(a, "=")
		switch {
		case attached && isExec(flag):
			out = append(out, value)
		case isExec(a) && j+1 < len(args):
			out = append(out, args[j+1])
			j++
		case len(a) > 1 && a[0] == '-' && a[1] != '-':
			cmd, ok := clusterExec(a[1:])
			if ok && cmd == "" && j+1 < len(args) {
				cmd = args[j+1]
				j++
			}
			if cmd != "" {
				out = append(out, cmd)
			}
		}
	}
	return out
}

// clusterExec is the exec command in letters, a short-option cluster of
// git rebase such as ix or xcmd: the rest after its x, "" when the x ends
// it and the command is the next word. A letter that takes a value (-s,
// -X, -C) holds the rest of the cluster, so an x after it is data.
func clusterExec(letters string) (string, bool) {
	for i, c := range letters {
		switch c {
		case 'x':
			return letters[i+1:], true
		case 's', 'X', 'C':
			return "", false
		}
	}
	return "", false
}

// fdTarget is a >& target that names a descriptor: 2, 2-, or -.
var fdTarget = regexp.MustCompile(`^([0-9]+-?|-)$`)

// flockString is the command string a flock among words' wrappers runs
// with -c or --command, before its lock file or right after it, since flock
// permutes its options: flock -c "rm -rf ~" /tmp/l and flock /tmp/l -c
// "rm -rf ~" both run rm -rf ~ in a shell.
func flockString(words []Word) (string, bool) {
	runs, n := wrappedBy(words)
	for _, w := range runs {
		if w.name != "flock" {
			continue
		}
		for k := w.at + 1; k < w.end && words[k].Value != "--"; k++ {
			if cmd, ok := flockCommand(words, k); ok {
				return cmd, true
			}
		}
		if w.end == n && n < len(words) {
			return flockCommand(words, n)
		}
	}
	return "", false
}

// flockCommand reads words[k] as flock's -c or --command, in a short
// cluster too (-nc), and returns the string it runs: the rest of the word
// or the next one.
func flockCommand(words []Word, k int) (string, bool) {
	opt := words[k].Value
	next := func() (string, bool) {
		if k+1 < len(words) {
			return words[k+1].Value, true
		}
		return "", false
	}
	if name, value, eq := strings.Cut(opt, "="); strings.HasPrefix(opt, "--") {
		if !abbrevOf(name, "--command") {
			return "", false
		}
		if eq {
			return value, true
		}
		return next()
	}
	if !strings.HasPrefix(opt, "-") {
		return "", false
	}
	for i := 1; i < len(opt); i++ {
		switch {
		case opt[i] == 'c' && i+1 < len(opt):
			return opt[i+1:], true
		case opt[i] == 'c':
			return next()
		case wrappers["flock"].takes("-" + opt[i:i+1]):
			return "", false
		}
	}
	return "", false
}

// watchString is the command a watch among words' wrappers runs: its
// operands joined, which watch hands to sh -c unless -x execs them.
func watchString(words []Word) (string, bool) {
	runs, _ := wrappedBy(words)
	for _, w := range runs {
		if w.name != "watch" {
			continue
		}
		i := w.at
		j := i + 1
		for ; j < len(words) && strings.HasPrefix(words[j].Value, "-"); j++ {
			opt := words[j].Value
			if opt == "--" {
				j++
				break
			}
			if abbrevOf(opt, "--exec") || shortCluster(opt, "x") {
				return "", false
			}
			if takesNextWord(wrappers["watch"], opt) {
				j++
			}
		}
		if j >= len(words) {
			return "", false
		}
		var parts []string
		for _, w := range words[j:] {
			parts = append(parts, w.Value)
		}
		return strings.Join(parts, " "), true
	}
	return "", false
}

// timeOutputs are the files a time among words' wrappers writes its report
// to with -o, as > redirects, so they get a redirect's checks.
func timeOutputs(words []Word) []Redir {
	var out []Redir
	runs, _ := wrappedBy(words)
	for _, w := range runs {
		if w.name != "time" {
			continue
		}
		found, done := timeOptions(words, w.at)
		out = append(out, found...)
		if done {
			return out
		}
	}
	return out
}

// timeOptions reads the options of the time at words[at]. done is true at
// --, which ends the scan for every wrapper after it too.
func timeOptions(words []Word, at int) (out []Redir, done bool) {
	for j := at + 1; j < len(words) && strings.HasPrefix(words[j].Value, "-"); j++ {
		opt := words[j].Value
		switch {
		case opt == "--":
			return out, true
		case abbrevOf(opt, "--output"):
			// getopt_long takes any unambiguous prefix: --out file.
			if _, v, ok := strings.Cut(opt, "="); ok {
				out = append(out, Redir{Op: ">", Target: Word{Value: v, Raw: v}})
			} else if j+1 < len(words) {
				j++
				out = append(out, Redir{Op: ">", Target: words[j]})
			}
		case strings.HasPrefix(opt, "--"):
			if takesNextWord(wrappers["time"], opt) {
				j++
			}
		default:
			// A short cluster: its first f or o takes the rest of the
			// word, or the next word when it's last (-ao file).
			k := strings.IndexAny(opt[1:], "fo") + 1
			if k == 0 {
				continue
			}
			v := opt[k+1:]
			if v == "" && j+1 < len(words) {
				j++
				v = words[j].Value
			}
			if opt[k] == 'o' && v != "" {
				out = append(out, Redir{Op: ">", Target: Word{Value: v, Raw: v}})
			}
		}
	}
	return out, false
}

// envSplitString returns the command env -S/--split-string runs, the string
// followed by env's remaining words, from a wrapper before the command:
// env -S "rm -rf" ~ runs rm -rf ~.
func envSplitString(words []Word) (string, bool) {
	runs, _ := wrappedBy(words)
	for _, w := range runs {
		if w.name != "env" {
			continue
		}
		i := w.at
		for j := i + 1; j < len(words) && strings.HasPrefix(words[j].Value, "-"); j++ {
			opt, cmd := words[j].Value, ""
			name, value, eq := strings.Cut(opt, "=")
			switch {
			case opt == "--":
				return "", false
			case abbrevOf(name, "--split-string") && eq:
				cmd = value
			case abbrevOf(name, "--split-string"):
				if j+1 >= len(words) {
					return "", false
				}
				j++
				cmd = words[j].Value
			case strings.HasPrefix(opt, "--"):
				if takesNextWord(wrappers["env"], opt) {
					j++
				}
				continue
			default:
				// A short cluster (-iS, -C/tmp): its first S, u, or C takes the
				// rest of the word, or the next word when it's last. Skipping a
				// u or C value lets the scan reach a later -S.
				k := strings.IndexAny(opt[1:], "SuC") + 1
				switch {
				case k == 0:
					continue
				case opt[k] != 'S':
					if k == len(opt)-1 {
						j++
					}
					continue
				case k < len(opt)-1:
					cmd = opt[k+1:]
				case j+1 >= len(words):
					return "", false
				default:
					j++
					cmd = words[j].Value
				}
			}
			for _, w := range words[j+1:] {
				cmd += " " + w.Raw
			}
			return cmd, true
		}
	}
	return "", false
}

// assigned is the value each bare assignment in seg sets (R=rm; $R -rf ~),
// for the segments after it. A prefix assignment (R=rm $R) is left out:
// the shell expands $R before it applies.
func assigned(seg Segment) map[string]string {
	vars := map[string]string{}
	for _, call := range seg.Calls {
		assigns := call.Assigns
		switch {
		case len(call.Words) > 0 && call.Words[0].Value == "export":
			// export NAME=value sets NAME as a bare assignment does.
			assigns = slices.DeleteFunc(values(call.Words[1:]), func(w string) bool { return !assignment.MatchString(w) })
		case len(call.Words) > 0:
			continue
		}
		for _, a := range assigns {
			name, value, _ := strings.Cut(a, "=")
			vars[name] = value
		}
	}
	return vars
}

// loopAssigned is every value each loop's bare assignments set, by loop
// id: a later pass sees what an earlier one assigned.
func loopAssigned(segs []Segment) map[int]map[string][]string {
	out := map[int]map[string][]string{}
	for _, seg := range segs {
		if seg.loop == 0 {
			continue
		}
		if out[seg.loop] == nil {
			out[seg.loop] = map[string][]string{}
		}
		for name, value := range assigned(seg) {
			out[seg.loop][name] = append(out[seg.loop][name], value)
		}
	}
	return out
}

// varRef is a word that is one variable and nothing else: $R or ${R}.
var varRef = regexp.MustCompile(`^\$(?:([A-Za-z_][A-Za-z0-9_]*)|\{([A-Za-z_][A-Za-z0-9_]*)\})$`)

// varFields is the fields w may expand to in seg when it is a variable the
// command assigned itself: the value set before seg, and in a loop every
// value the loop sets. Nil for any other word, $EDITOR included.
func (st *state) varFields(seg Segment, w Word) [][]string {
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

// inner collects the statements nested anywhere under n: substitution
// bodies and compound-command bodies.
func (c *collector) inner(src string, n syntax.Node) {
	if n == nil {
		return
	}
	syntax.Walk(n, func(node syntax.Node) bool {
		var stmts []*syntax.Stmt
		switch v := node.(type) {
		case *syntax.Stmt:
			c.stmt(src, v)
			return false
		case *syntax.ForClause, *syntax.WhileClause:
			return c.inLoop(src, v)
		case *syntax.Subshell:
			stmts = v.Stmts
		case *syntax.CmdSubst:
			stmts = v.Stmts
		case *syntax.ProcSubst:
			stmts = v.Stmts
		default:
			return true
		}
		c.subshell++
		c.stmts(src, stmts)
		c.subshell--
		return false
	})
}

// inLoop collects an outermost for, while, or until loop's parts under a
// loop id of their own, and reports whether the walk should descend: a loop
// nested in another takes the outer one's id, since the outer pass reruns
// it too.
func (c *collector) inLoop(src string, loop syntax.Node) bool {
	if c.loop != 0 {
		return true
	}
	c.loops++
	c.loop = c.loops
	switch l := loop.(type) {
	case *syntax.ForClause:
		c.inner(src, l.Loop)
		c.stmts(src, l.Do)
	case *syntax.WhileClause:
		c.stmts(src, append(slices.Clone(l.Cond), l.Do...))
	}
	c.loop = 0
	return false
}

// pipeline flattens a | or |& chain (or a lone command, or a redirect
// alone) into its parts; nil for anything else.
func pipeline(s *syntax.Stmt) []*syntax.Stmt {
	switch cmd := s.Cmd.(type) {
	case *syntax.CallExpr, *syntax.DeclClause, nil:
		return []*syntax.Stmt{s}
	case *syntax.TimeClause:
		// curl x | time sh feeds sh like any stage; a bare time has no
		// statement.
		if cmd.Stmt == nil {
			return nil
		}
		return pipeline(cmd.Stmt)
	case *syntax.BinaryCmd:
		if cmd.Op != syntax.Pipe && cmd.Op != syntax.PipeAll {
			return nil
		}
		left := pipeline(cmd.X)
		if left == nil {
			left = []*syntax.Stmt{cmd.X}
		}
		right := pipeline(cmd.Y)
		if right == nil {
			right = []*syntax.Stmt{cmd.Y}
		}
		return append(left, right...)
	}
	return nil
}

// literal is whether parts are plain text, single-quoted text with no $,
// or double-quoted plain text, with no brace or glob character outside
// quotes. An allow for a path reads only such words, since any other form
// (an expansion, $'...', {a,b}, *) isn't the text the command gets.
func literal(parts []syntax.WordPart) bool {
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.Lit:
			if strings.ContainsAny(p.Value, "{}*?[") {
				return false
			}
		case *syntax.SglQuoted:
			if p.Dollar {
				return false
			}
		case *syntax.DblQuoted:
			for _, inner := range p.Parts {
				if _, ok := inner.(*syntax.Lit); !ok {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

// wordValue removes quotes and resolves escapes, leaving every expansion as
// its source text.
func wordValue(src string, w *syntax.Word) string {
	var b strings.Builder
	for _, part := range w.Parts {
		b.WriteString(partValue(src, part, false))
	}
	return b.String()
}

func partValue(src string, part syntax.WordPart, inDouble bool) string {
	switch p := part.(type) {
	case *syntax.Lit:
		return unescape(p.Value, inDouble)
	case *syntax.SglQuoted:
		if p.Dollar {
			// $'\x72m' is rm. Format reads printf verbs too, so a % stays
			// literal; the shell cuts the value at a NUL.
			v, _, err := expand.Format(nil, strings.ReplaceAll(p.Value, "%", "%%"), nil)
			if err != nil {
				return src[p.Pos().Offset():p.End().Offset()]
			}
			v, _, _ = strings.Cut(v, "\x00")
			return v
		}
		return p.Value
	case *syntax.DblQuoted:
		var b strings.Builder
		for _, inner := range p.Parts {
			b.WriteString(partValue(src, inner, true))
		}
		return b.String()
	default:
		return src[part.Pos().Offset():part.End().Offset()]
	}
}

// unescape resolves backslash escapes as bash does: anything outside
// quotes, only $ ` " \ and newline inside double quotes.
func unescape(s string, inDouble bool) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		next := s[i+1]
		switch {
		case next == '\n':
			i++
		case !inDouble || strings.IndexByte("$`\"\\", next) >= 0:
			b.WriteByte(next)
			i++
		default:
			b.WriteByte('\\')
		}
	}
	return b.String()
}

func baseName(s string) string { return s[strings.LastIndexByte(s, '/')+1:] }
