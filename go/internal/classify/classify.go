// Package classify reads a task body or a CI step as shell and says what
// each command in it is: a quality gate (a tool kit.yml's checks name), a
// reference to another task, a fan-out across workspace packages, or
// something else. Nothing it can't read with certainty counts as a gate.
package classify

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

// Kind is what one command is.
type Kind int

const (
	Other  Kind = iota // runs, but isn't a gate (build, codegen, echo)
	Gate               // a tool a check's pattern names
	Ref                // runs other tasks
	FanOut             // runs a task across workspace packages
	Cd                 // a literal cd, which later commands depend on
	Export             // export K=V, or a bare K=V
)

// TaskRef is a task a command runs: its provider, the directory relative to
// the body's (make -C web), its name, and the arguments passed after --.
type TaskRef struct {
	Provider string
	Dir      string
	Name     string
	Args     []string
}

// Command is one simple command of a body.
type Command struct {
	Kind    Kind
	Words   []string            // after wrappers and tool runners
	ToolAt  int                 // Gate: Tool's word in Words (find -exec runs it later)
	Env     []string            // K=V assignments in front of it, or in a wrapper
	Slot    string              // Gate: the check it counts as
	Tool    string              // Gate: the pattern's bin
	Pattern *config.ToolPattern // Gate: the pattern it matched
	Refs    []TaskRef
	// Expansion: a word held a variable; Words ends at it with "$".
	Expansion bool
	Line      uint     // the body line its statement starts on
	Globs     []string // words with an unquoted glob, which a shell expands
	// Other: a check's tool run with a flag the check forbids ("eslint
	// --fix"), which no gate may run.
	Forbidden string
}

// Result is a whole body. Opaque says why it can't be read (a pipe, ||,
// a subshell, command substitution, a parse error); its Commands are then
// empty.
type Result struct {
	Commands []Command
	Opaque   string
}

// Lookup reports whether provider has a task called name in dir (relative
// to the body's directory).
type Lookup func(provider, dir, name string) bool

// Body classifies body.
func Body(cfg *config.Config, body string, lookup Lookup) Result {
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(body), "")
	if err != nil {
		return Result{Opaque: "parse error"}
	}
	c := &classifier{cfg: cfg, lookup: lookup}
	for _, stmt := range file.Stmts {
		n := len(c.out)
		c.stmt(stmt)
		if c.opaque != "" {
			return Result{Opaque: c.opaque}
		}
		for i := n; i < len(c.out); i++ {
			c.out[i].Line = stmt.Pos().Line()
		}
	}
	return Result{Commands: c.out}
}

// SingleGate is the body's one gate when, apart from cd and exports, it is
// exactly one gate command.
func (r Result) SingleGate() (Command, bool) {
	var gate Command
	n := 0
	for _, cmd := range r.Commands {
		switch cmd.Kind {
		case Cd, Export:
		case Gate:
			gate = cmd
			n++
		default:
			return Command{}, false
		}
	}
	return gate, n == 1
}

type classifier struct {
	cfg    *config.Config
	lookup Lookup
	out    []Command
	opaque string
}

func (c *classifier) stmt(s *syntax.Stmt) {
	switch {
	case s.Background || s.Coprocess:
		c.opaque = "background job"
		return
	case s.Negated:
		c.opaque = "negated command"
		return
	}
	for _, r := range s.Redirs {
		if !harmlessRedirect(r) {
			c.opaque = "redirect"
			return
		}
	}
	switch cmd := s.Cmd.(type) {
	case *syntax.CallExpr:
		c.call(cmd)
	case *syntax.BinaryCmd:
		if cmd.Op != syntax.AndStmt {
			c.opaque = cmd.Op.String()
			return
		}
		c.stmt(cmd.X)
		if c.opaque == "" {
			c.stmt(cmd.Y)
		}
	case *syntax.DeclClause:
		if cmd.Variant.Value != "export" {
			c.opaque = cmd.Variant.Value
			return
		}
		var env []string
		for _, a := range cmd.Args {
			// export FOO re-exports FOO's current value; it sets nothing.
			if a.Name == nil || a.Naked {
				continue
			}
			kv, ok := assignment(a)
			if !ok {
				c.opaque = "export with an expansion"
				return
			}
			env = append(env, kv)
		}
		c.out = append(c.out, Command{Kind: Export, Env: env})
	default:
		c.opaque = "compound command"
	}
}

// assignment is a as K=V; ok is false when its value holds an expansion.
func assignment(a *syntax.Assign) (string, bool) {
	if a.Value == nil {
		return a.Name.Value + "=", true
	}
	v, ok := literal(a.Value)
	return a.Name.Value + "=" + v, ok
}

// harmlessRedirect is a redirect that writes no file: to /dev/null, or a
// file descriptor duplicated onto another (2>&1).
func harmlessRedirect(r *syntax.Redirect) bool {
	if r.Op == syntax.DplOut || r.Op == syntax.DplIn {
		return true
	}
	target, ok := literal(r.Word)
	return ok && target == "/dev/null"
}

func (c *classifier) call(call *syntax.CallExpr) {
	var env []string
	for _, a := range call.Assigns {
		kv, ok := assignment(a)
		if !ok {
			// FOO=$BAR: later commands see a value the kit can't know.
			c.out = append(c.out, Command{Kind: Other, Words: []string{a.Name.Value + "=$"}, Expansion: true})
			return
		}
		env = append(env, kv)
	}
	var words, globs []string
	for _, w := range call.Args {
		if hasCmdSubst(w) {
			c.opaque = "command substitution"
			return
		}
		v, ok := literal(w)
		bare := unquoted(w)
		if ok && (strings.HasPrefix(bare, "~") || braces.MatchString(bare)) {
			ok = false // ~ and {a,b}: a shell rewrites them; exec doesn't
		}
		if !ok {
			// Keep the literal words before it, so cd "$DIR" still reads
			// as a cd the kit can't follow.
			c.out = append(c.out, Command{Kind: Other, Words: append(words, "$"), Env: env, Expansion: true})
			return
		}
		if strings.ContainsAny(bare, "*?[") {
			globs = append(globs, v)
		}
		words = append(words, v)
	}
	if len(words) == 0 {
		c.out = append(c.out, Command{Kind: Export, Env: env})
		return
	}
	cmds := c.words(words, env)
	for i := range cmds {
		cmds[i].Globs = globs
	}
	c.out = append(c.out, cmds...)
}

var braces = regexp.MustCompile(`\{[^{}]*(,|\.\.)[^{}]*\}`)

// unquoted is w's text outside quotes, where a shell expands globs, braces,
// and a leading ~.
func unquoted(w *syntax.Word) string {
	var b strings.Builder
	for _, part := range w.Parts {
		if lit, ok := part.(*syntax.Lit); ok {
			b.WriteString(lit.Value)
		} else {
			b.WriteString("\x00")
		}
	}
	return b.String()
}

// words classifies one command's literal words; concurrently can make one
// command several.
func (c *classifier) words(words, env []string) []Command {
	g := Grammar
	for {
		if prefix, ok := matchPrefix(words, g.Wrappers); ok {
			words = words[prefix:]
			for len(words) > 0 && strings.Contains(words[0], "=") && !strings.HasPrefix(words[0], "-") {
				env = append(env, words[0])
				words = words[1:]
			}
			continue
		}
		break
	}
	if len(words) == 0 {
		return []Command{{Kind: Export, Env: env}}
	}
	if words[0] == "cd" {
		return []Command{{Kind: Cd, Words: words}}
	}
	if _, ok := matchPrefix(words, g.FanOutCommands); ok {
		return []Command{{Kind: FanOut, Words: words, Env: env}}
	}
	if slices.Contains(g.ScriptRunners, words[0]) {
		var refs []TaskRef
		for _, w := range words[1:] {
			if !strings.HasPrefix(w, "-") {
				refs = append(refs, TaskRef{Provider: "package-scripts", Name: w})
			}
		}
		return []Command{{Kind: Ref, Words: words, Env: env, Refs: refs}}
	}
	if words[0] == "concurrently" {
		return c.concurrently(words[1:], env)
	}
	// Tool runners first: pnpm exec eslint isn't a reference to a script
	// called exec, and bundle exec rake lint is a reference once unwrapped.
	if prefix, ok := matchPrefix(words, g.ToolRunners); ok && len(words) > prefix {
		return c.words(words[prefix:], env)
	}
	if cmd, ok := c.reference(words, env); ok {
		return []Command{cmd}
	}
	if at, inner := execTarget(words); inner != nil {
		if cmd := c.tool(inner, env); cmd.Kind == Gate {
			cmd.Words, cmd.ToolAt = words, at
			return []Command{cmd}
		}
	}
	return []Command{c.tool(words, env)}
}

// xargsValueFlags take the next word as their value.
var xargsValueFlags = []string{"-n", "-P", "-I", "-L", "-d", "-s", "-a", "-E", "--max-args", "--max-procs", "--replace", "--delimiter", "--arg-file", "--max-lines"}

// execTarget is the command find -exec/-execdir or xargs runs on the files
// they pick, without find's {} and terminator, and where it starts in words;
// nil when words is neither.
func execTarget(words []string) (at int, inner []string) {
	switch words[0] {
	case "find":
		i := slices.IndexFunc(words, func(w string) bool { return w == "-exec" || w == "-execdir" })
		if i < 0 || i+1 >= len(words) {
			return 0, nil
		}
		for _, w := range words[i+1:] {
			if w == ";" || w == `\;` || w == "+" {
				break
			}
			if w != "{}" {
				inner = append(inner, w)
			}
		}
		return i + 1, inner
	case "xargs":
		i := 1
		for i < len(words) && strings.HasPrefix(words[i], "-") {
			if slices.Contains(xargsValueFlags, words[i]) {
				i++
			}
			i++
		}
		if i >= len(words) {
			return 0, nil
		}
		return i, words[i:]
	}
	return 0, nil
}

// reference reads words as a task reference, by the longest matching
// reference prefix.
func (c *classifier) reference(words, env []string) (Command, bool) {
	var best *Reference
	bestLen := 0
	for i, r := range Grammar.References {
		n := len(strings.Fields(r.Prefix))
		if n > bestLen && hasPrefix(words, strings.Fields(r.Prefix)) {
			best, bestLen = &Grammar.References[i], n
		}
	}
	if best == nil {
		return Command{}, false
	}
	rest := words[bestLen:]
	ref := TaskRef{Provider: best.Provider, Name: best.Task}
	runner := best.Provider == "make" || best.Provider == "just"
	// make runs every target it's given; just runs each word that names a
	// recipe, the rest being arguments to the recipe before it.
	var more []string
	for i := 0; i < len(rest); i++ {
		w := rest[i]
		switch {
		case w == "--":
			ref.Args = rest[i+1:]
			i = len(rest)
		case slices.Contains(Grammar.FanOutFlags, w) || slices.ContainsFunc(Grammar.FanOutFlags, func(f string) bool { return strings.HasPrefix(w, f+"=") }):
			return Command{Kind: FanOut, Words: words, Env: env}, true
		case runner && slices.Contains([]string{"-f", "--file", "--makefile", "--justfile"}, w):
			// Another file's targets: the kit can't look them up.
			return Command{Kind: Other, Words: words, Env: env}, true
		case best.Provider == "make" && (w == "-C" || w == "--directory") && i+1 < len(rest),
			best.Provider == "just" && (w == "-d" || w == "--working-directory") && i+1 < len(rest):
			ref.Dir = rest[i+1]
			i++
		case best.Provider == "make" && strings.HasPrefix(w, "-C"):
			ref.Dir = strings.TrimPrefix(w, "-C")
		case best.Provider == "make" && slices.Contains([]string{"-j", "-l"}, w) && i+1 < len(rest) && isNumber(rest[i+1]),
			best.Provider == "make" && slices.Contains([]string{"-o", "-W", "-I"}, w) && i+1 < len(rest):
			i++ // a flag's value (make -j 4), not a target
		case runner && strings.Contains(w, "="):
			// make GOARCH=arm64 build, just os=linux build: a variable.
		case strings.HasPrefix(w, "-"):
		case ref.Name == "":
			ref.Name = w
		case best.Provider == "make",
			best.Provider == "just" && len(ref.Args) == 0 && c.lookup(ref.Provider, ref.Dir, w):
			more = append(more, w)
		default:
			ref.Args = append(ref.Args, w)
		}
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
		return c.tool(words[bestLen:], env), true
	}
	return Command{Kind: Ref, Words: words, Env: env, Refs: []TaskRef{ref}}, true
}

// concurrently's arguments are quoted commands, each classified on its own.
func (c *classifier) concurrently(args, env []string) []Command {
	valued := []string{"-n", "--names", "-c", "--prefix-colors", "-p", "--prefix", "-s", "--success"}
	var out []Command
	for i := 0; i < len(args); i++ {
		switch {
		case slices.Contains(valued, args[i]):
			i++
		case strings.HasPrefix(args[i], "-"):
		default:
			r := Body(c.cfg, args[i], c.lookup)
			if r.Opaque != "" {
				c.opaque = r.Opaque
				return nil
			}
			out = append(out, r.Commands...)
		}
	}
	if len(env) > 0 {
		for i := range out {
			out[i].Env = append(slices.Clone(env), out[i].Env...)
		}
	}
	return out
}

// tool matches words against every check's tool patterns; the pattern with
// the most required flags present wins (tflint --only=terraform_unused_...
// is dead code, plain tflint is lint), the first on a tie.
func (c *classifier) tool(words, env []string) Command {
	best, bestScore := Command{Kind: Other, Words: words, Env: env}, -1
	forbidden := ""
	for ci := range c.cfg.Checks {
		check := &c.cfg.Checks[ci]
		for pi := range check.Tools {
			p := &check.Tools[pi]
			score, flag := match(words, *p)
			if score > bestScore {
				best, bestScore = Command{Kind: Gate, Words: words, Env: env, Slot: check.Name, Tool: p.Bin, Pattern: p}, score
			}
			if forbidden == "" && flag != "" {
				forbidden = p.Bin + " " + flag
			}
		}
	}
	if best.Kind == Other {
		best.Forbidden = forbidden
	}
	return best
}

// match scores words against p: -1 when they don't match, else the number
// of required flags (all of which are present). flag is a forbidden flag
// words set, whatever the required ones; one set off (--watch=false) isn't.
func match(words []string, p config.ToolPattern) (score int, flag string) {
	bin := words[0]
	if i := strings.LastIndex(bin, "/"); i >= 0 {
		bin = bin[i+1:]
	}
	if bin != p.Bin {
		return -1, ""
	}
	args := words[1:]
	if len(p.Sub) > 0 {
		sub := ""
		for _, a := range args {
			if !strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "+") {
				sub = a
				break
			}
		}
		if !slices.Contains(p.Sub, sub) {
			return -1, ""
		}
	}
	// set: the flag alone, or flag=value with a value that doesn't turn it off.
	set := func(flag string) bool {
		return slices.ContainsFunc(args, func(a string) bool {
			value, ok := strings.CutPrefix(a, flag+"=")
			return a == flag || ok && !off(value)
		})
	}
	for _, f := range p.Forbid {
		if set(f) {
			return -1, f
		}
	}
	for _, f := range p.Require {
		if !set(f) {
			return -1, ""
		}
	}
	return len(p.Require), ""
}

// off is true for a flag value that turns the flag off (--watch=false).
func off(value string) bool {
	return slices.Contains([]string{"false", "0", "no", "off"}, strings.ToLower(value))
}

// matchPrefix is the length of the first of prefixes (each one or more
// space-separated words) that words start with.
func matchPrefix(words, prefixes []string) (int, bool) {
	for _, p := range prefixes {
		if f := strings.Fields(p); hasPrefix(words, f) {
			return len(f), true
		}
	}
	return 0, false
}

func isNumber(w string) bool {
	return w != "" && strings.Trim(w, "0123456789") == ""
}

func hasPrefix(words, prefix []string) bool {
	return len(prefix) > 0 && len(words) >= len(prefix) && slices.Equal(words[:len(prefix)], prefix)
}

// literal is w's value when it has no expansions at all.
func literal(w *syntax.Word) (string, bool) {
	var b strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, q := range p.Parts {
				lit, ok := q.(*syntax.Lit)
				if !ok {
					return "", false
				}
				b.WriteString(lit.Value)
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

func hasCmdSubst(w *syntax.Word) bool {
	found := false
	syntax.Walk(w, func(n syntax.Node) bool {
		if _, ok := n.(*syntax.CmdSubst); ok {
			found = true
		}
		return !found
	})
	return found
}
