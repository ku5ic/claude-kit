// Package classify reads a task body or a CI step as shell and says what
// each command in it is: a quality gate (a tool kit.yml's checks name), a
// reference to another task, a fan-out across workspace packages, or
// something else. Nothing it can't read with certainty counts as a gate.
package classify

import (
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
	Env     []string            // K=V assignments in front of it, or in a wrapper
	Slot    string              // Gate: the check it counts as
	Tool    string              // Gate: the pattern's bin
	Pattern *config.ToolPattern // Gate: the pattern it matched
	Refs    []TaskRef
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
		c.stmt(stmt)
		if c.opaque != "" {
			return Result{Opaque: c.opaque}
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
			if a.Name == nil {
				continue
			}
			value := ""
			if a.Value != nil {
				v, ok := literal(a.Value)
				if !ok {
					c.opaque = "export with an expansion"
					return
				}
				value = v
			}
			env = append(env, a.Name.Value+"="+value)
		}
		c.out = append(c.out, Command{Kind: Export, Env: env})
	default:
		c.opaque = "compound command"
	}
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
		value := ""
		if a.Value != nil {
			v, ok := literal(a.Value)
			if !ok {
				c.out = append(c.out, Command{Kind: Other})
				return
			}
			value = v
		}
		env = append(env, a.Name.Value+"="+value)
	}
	var words []string
	for _, w := range call.Args {
		if hasCmdSubst(w) {
			c.opaque = "command substitution"
			return
		}
		v, ok := literal(w)
		if !ok {
			c.out = append(c.out, Command{Kind: Other, Env: env})
			return
		}
		words = append(words, v)
	}
	if len(words) == 0 {
		c.out = append(c.out, Command{Kind: Export, Env: env})
		return
	}
	c.out = append(c.out, c.words(words, env)...)
}

// words classifies one command's literal words; concurrently can make one
// command several.
func (c *classifier) words(words, env []string) []Command {
	g := c.cfg.GateDiscovery
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
	return []Command{c.tool(words, env)}
}

// reference reads words as a task reference, by the longest matching
// reference prefix.
func (c *classifier) reference(words, env []string) (Command, bool) {
	var best *config.Reference
	bestLen := 0
	for i, r := range c.cfg.GateDiscovery.References {
		n := len(strings.Fields(r.Prefix))
		if n > bestLen && hasPrefix(words, strings.Fields(r.Prefix)) {
			best, bestLen = &c.cfg.GateDiscovery.References[i], n
		}
	}
	if best == nil {
		return Command{}, false
	}
	rest := words[bestLen:]
	ref := TaskRef{Provider: best.Provider, Name: best.Task}
	for i := 0; i < len(rest); i++ {
		w := rest[i]
		switch {
		case w == "--":
			ref.Args = rest[i+1:]
			i = len(rest)
		case slices.Contains(c.cfg.GateDiscovery.FanOutFlags, w) || slices.ContainsFunc(c.cfg.GateDiscovery.FanOutFlags, func(f string) bool { return strings.HasPrefix(w, f+"=") }):
			return Command{Kind: FanOut, Words: words, Env: env}, true
		case best.Provider == "make" && w == "-C" && i+1 < len(rest):
			ref.Dir = rest[i+1]
			i++
		case best.Provider == "make" && strings.HasPrefix(w, "-C"):
			ref.Dir = strings.TrimPrefix(w, "-C")
		case strings.HasPrefix(w, "-"):
		case ref.Name == "":
			ref.Name = w
		default:
			ref.Args = append(ref.Args, w)
		}
	}
	if ref.Name == "" {
		return Command{Kind: Other, Words: words, Env: env}, true
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
	for ci := range c.cfg.Checks {
		check := &c.cfg.Checks[ci]
		for pi := range check.Tools {
			p := &check.Tools[pi]
			if score := match(words, *p); score > bestScore {
				best, bestScore = Command{Kind: Gate, Words: words, Env: env, Slot: check.Name, Tool: p.Bin, Pattern: p}, score
			}
		}
	}
	return best
}

// match scores words against p: -1 when they don't match, else the number
// of required flags (all of which are present).
func match(words []string, p config.ToolPattern) int {
	bin := words[0]
	if i := strings.LastIndex(bin, "/"); i >= 0 {
		bin = bin[i+1:]
	}
	if bin != p.Bin {
		return -1
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
			return -1
		}
	}
	has := func(flag string) bool {
		return slices.ContainsFunc(args, func(a string) bool { return a == flag || strings.HasPrefix(a, flag+"=") })
	}
	for _, f := range p.Forbid {
		if has(f) {
			return -1
		}
	}
	for _, f := range p.Require {
		if !has(f) {
			return -1
		}
	}
	return len(p.Require)
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
