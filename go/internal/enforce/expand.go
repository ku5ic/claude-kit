package enforce

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// maxDepth bounds how deep task references are followed; a cycle ends there.
const maxDepth = 8

// part is one top-level statement of a body: a reference to a task, or a
// command of its own, with the check kinds it runs.
type part struct {
	text  string // the statement as written
	dir   string // where it runs, after the cds before it
	env   []string
	task  *sources.Entry
	kinds []string
}

// expand plans entry e, a check, run in dir with env: an aggregate whose
// parts all check runs as its parts, each on its own line; anything else
// runs whole.
func (b *builder) expand(e sources.Entry, v gapfill.Verdict, dir string, env []string) {
	b.expandAt(e, v, dir, env, 0)
}

func (b *builder) expandAt(e sources.Entry, v gapfill.Verdict, dir string, env []string, depth int) {
	parts, ok := b.split(e.Body.Text, dir, env, v, depth)
	switch {
	case ok && len(parts) == 1 && parts[0].task != nil:
		b.reference(parts[0], depth)
	case ok && len(parts) > 1:
		for _, p := range parts {
			if p.task != nil {
				b.reference(p, depth)
			} else {
				b.add(Gate{Label: label(p.kinds, e.File+": "+e.Name+": "+firstWords(p.text), b.rel(p.dir)), Kinds: p.kinds, Dir: p.dir, Env: p.env, Body: p.text, Verdict: v.String()})
			}
		}
	default:
		b.add(b.entryGate(e, v, dir, env))
	}
}

// reference plans the task p runs: its own parts when it splits into more
// than one, else p as written, or the task's affected form when it has one.
func (b *builder) reference(p part, depth int) {
	t := *p.task
	v, _ := b.verdict(t)
	if depth < maxDepth {
		if parts, ok := b.split(t.Body.Text, p.dir, p.env, v, depth+1); ok && (len(parts) > 1 || len(parts) == 1 && parts[0].task != nil) {
			b.expandAt(t, v, p.dir, p.env, depth+1)
			return
		}
	}
	g := Gate{Label: label(p.kinds, t.File+": "+t.Name, b.rel(p.dir)), Kinds: p.kinds, Dir: p.dir, Env: p.env, Body: p.text, Verdict: v.String(), FileForm: v.FileForm, Globs: v.Globs}
	if v.Affected != "" {
		g.Dir, g.Body = filepath.Join(b.root, t.Dir), v.Affected
	}
	g.FanOut = b.fansOut(g.Body)
	b.add(g)
}

// entryGate is e's body run whole in dir: its affected form when it has
// one, labeled with every kind it checks.
func (b *builder) entryGate(e sources.Entry, v gapfill.Verdict, dir string, env []string) Gate {
	kinds := b.taskKinds(e, v, 0)
	g := Gate{Label: label(kinds, e.File+": "+e.Name, b.rel(dir)), Kinds: kinds, Dir: dir, Env: env, Body: cmp.Or(v.Affected, e.Body.Text), Verdict: v.String(), FileForm: v.FileForm, Globs: v.Globs}
	g.FanOut = b.fansOut(g.Body)
	return g
}

// split reads body, run in dir with env, as its top-level statements: a cd
// moves the rest, an export joins their env, a reference takes its task's
// kinds, and any other statement takes the kind of the verdict's segment
// holding it (the verdict's own for a body of one statement). ok is false
// when a statement isn't a check or the body can't be followed.
func (b *builder) split(body, dir string, env []string, v gapfill.Verdict, depth int) (parts []part, ok bool) {
	stmts, parsed := classify.Statements(body)
	if !parsed || depth > maxDepth {
		return nil, false
	}
	ok = true
	env = slices.Clone(env)
	for _, stmt := range stmts {
		r := classify.Body(b.nocat, stmt, b.lookup(dir))
		var cmd classify.Command
		if r.Opaque == "" && len(r.Commands) == 1 {
			cmd = r.Commands[0]
		}
		switch {
		case cmd.Kind == classify.Cd && r.Opaque == "" && len(r.Commands) == 1:
			next, can := b.cd(dir, cmd.Words)
			if !can {
				return nil, false
			}
			dir = next
		case cmd.Kind == classify.Export && r.Opaque == "" && len(r.Commands) == 1:
			env = append(env, cmd.Env...)
		case cmd.Kind == classify.Ref && len(cmd.Refs) == 1:
			ref := cmd.Refs[0]
			t, found := b.tasks[taskKey{ref.Provider, filepath.Join(dir, ref.Dir), ref.Name}]
			if !found {
				return nil, false
			}
			tv, why := b.verdict(t)
			kinds := b.taskKinds(t, tv, depth+1)
			ok = ok && why == "" && len(kinds) > 0
			parts = append(parts, part{text: stmt, dir: dir, env: append(slices.Clone(env), cmd.Env...), task: &t, kinds: kinds})
		default:
			kind, isCheck := segmentKind(v, stmt, len(stmts))
			ok = ok && isCheck
			parts = append(parts, part{text: stmt, dir: dir, env: slices.Clone(env), kinds: kindList(kind)})
		}
	}
	return parts, ok && len(parts) > 0
}

// taskKinds is the kinds task e checks: its verdict's, or, for an aggregate
// the verdict gives none, the kinds of the parts it runs.
func (b *builder) taskKinds(e sources.Entry, v gapfill.Verdict, depth int) []string {
	if kinds := kindsOf(v); len(kinds) > 0 || v.Role != "check" || depth > maxDepth {
		return kinds
	}
	var kinds []string
	parts, _ := b.split(e.Body.Text, filepath.Join(b.root, e.Dir), nil, v, depth+1)
	for _, p := range parts {
		for _, k := range p.kinds {
			if !slices.Contains(kinds, k) {
				kinds = append(kinds, k)
			}
		}
	}
	return kinds
}

// lookup answers classify's question, whether provider has task name, for
// a body run in dir.
func (b *builder) lookup(dir string) classify.Lookup {
	return func(provider, rel, name string) bool {
		_, ok := b.tasks[taskKey{provider, filepath.Join(dir, rel), name}]
		return ok
	}
}

// cd is the directory a literal cd moves dir to, false for one the kit
// can't follow: no argument, an expansion, or a way out of the root.
func (b *builder) cd(dir string, words []string) (string, bool) {
	if len(words) != 2 || strings.HasPrefix(words[1], "-") {
		return "", false
	}
	next := words[1]
	if !filepath.IsAbs(next) {
		next = filepath.Join(dir, next)
	}
	next = filepath.Clean(next)
	return next, next == b.root || strings.HasPrefix(next, b.root+"/")
}

// fansOut is true when body runs across workspace packages.
func (b *builder) fansOut(body string) bool {
	r := classify.Body(b.nocat, body, func(string, string, string) bool { return false })
	return slices.ContainsFunc(r.Commands, func(c classify.Command) bool { return c.Kind == classify.FanOut })
}

// segmentKind is the check kind of the segment of v holding stmt; for a
// body of one statement, v's own. isCheck is false for a segment that isn't
// a check, and for a statement no segment holds.
func segmentKind(v gapfill.Verdict, stmt string, statements int) (kind string, isCheck bool) {
	if len(v.Segments) == 0 && statements == 1 {
		return v.Kind, v.Role == "check"
	}
	text := strings.Join(strings.Fields(stmt), " ")
	for _, s := range v.Segments {
		seg := strings.Join(strings.Fields(s.Text), " ")
		if seg == text || strings.Contains(seg, text) || strings.Contains(text, seg) {
			return s.Kind, s.Role == "check"
		}
	}
	return "", false
}

// checks is true when v is a check, itself or in a segment.
func checks(v gapfill.Verdict) bool { return v.Role == "check" || len(kindsOf(v)) > 0 }

// kindsOf is the check kinds v states: its own, then its segments'.
func kindsOf(v gapfill.Verdict) []string {
	var kinds []string
	add := func(role, kind string) {
		if role == "check" && kind != "" && !slices.Contains(kinds, kind) {
			kinds = append(kinds, kind)
		}
	}
	add(v.Role, v.Kind)
	for _, s := range v.Segments {
		add(s.Role, s.Kind)
	}
	return kinds
}

func kindList(kind string) []string {
	if kind == "" {
		return nil
	}
	return []string{kind}
}

// firstWords is a statement's first two words, to tell parts apart.
func firstWords(stmt string) string {
	words := strings.Fields(stmt)
	return strings.Join(words[:min(2, len(words))], " ")
}
