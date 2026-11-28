package bashguard

import (
	"mvdan.cc/sh/v3/syntax"
	"slices"
)

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
		c.openSubshell()
		c.stmts(src, stmts)
		c.closeSubshell()
		return false
	})
}

// openSubshell enters a subshell with an id of its own, so a walk over
// the segments can tell where it ends; closeSubshell leaves it.
func (c *collector) openSubshell() {
	c.scopeIDs++
	c.scopes = append(c.scopes, c.scopeIDs)
	c.subshell++
	c.exitTraps = append(c.exitTraps, nil)
}

// closeSubshell first parses the EXIT traps set in the subshell, which may
// not run, inside it: they fire as it ends. One may set another, read in turn.
func (c *collector) closeSubshell() {
	top := len(c.exitTraps) - 1
	c.conditional++
	for i := 0; i < len(c.exitTraps[top]); i++ {
		c.parse(c.exitTraps[top][i], c.depth+1)
	}
	c.conditional--
	c.exitTraps = c.exitTraps[:top]
	c.scopes = c.scopes[:len(c.scopes)-1]
	c.subshell--
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
