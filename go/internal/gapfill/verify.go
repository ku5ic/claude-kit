package gapfill

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	// Aliased: gapfill has a classify function of its own.
	shellcalls "github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/resolve"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// verify turns s's answers into a Result. Policy first: an entry the deny
// lists catch is denied, answered or not. Then the project's files: manager
// facts must cite a lockfile or field that exists, a verdict's segments
// must be copied from its body, and a form must be its body's own words
// and never fetch, or it is dropped.
func verify(cfg *config.Config, root string, entries []sources.Entry, s store, failure string) Result {
	r := Result{Verdicts: map[string]Verdict{}, Skipped: map[string]string{}, Failure: failure}
	r.Managers, r.Dropped = managers(cfg, root, s.Managers)
	var prefixes []string
	for _, m := range r.Managers {
		prefixes = append(prefixes, m.RunPrefix)
	}
	resolver := resolve.New(root, "", nil, nil) // only Fetches, which reads neither home nor pins
	at := func(dir string) forms {
		return forms{cfg: cfg, root: root, dir: filepath.Join(root, dir), prefixes: prefixes, resolver: resolver}
	}
	for _, e := range entries {
		r.entry(cfg, e, s, at, failure)
	}
	r.Dropped = append(r.Dropped, coverage(entries, r.Verdicts)...)
	for _, p := range s.Proposals {
		r.proposal(p, at(p.Dir))
	}
	return r
}

// entry adds e's verdict to r, or why it's skipped: denied by policy,
// unanswered (cooling down after a failed call, when it is), or rejected.
func (r *Result) entry(cfg *config.Config, e sources.Entry, s store, at func(string) forms, failure string) {
	key := Key(e)
	if token := denied(cfg, e); token != "" {
		r.Skipped[key] = deniedReason + " (" + token + ")"
		return
	}
	a := s.Entries[key]
	if a == nil {
		r.Skipped[key] = unclassifiedReason
		if failure != "" {
			r.Skipped[key] += " (" + failure + ")"
		} else if f, cooling := s.cool.Cooling(failKey(cfg, key), time.Now()); cooling {
			r.Skipped[key] = coolingReason + f.Why + "; asked again after " + f.Until().Format("15:04") + ")"
		}
		return
	}
	v, dropped, err := at(e.Dir).verdict(e, a.Verdict)
	if err != nil {
		r.Skipped[key] = "rejected (" + err.Error() + ")"
		return
	}
	v.Globs = fromDir(e.Dir, v.Globs)
	for _, d := range dropped {
		r.Dropped = append(r.Dropped, e.Source+" "+e.Name+": "+d)
	}
	r.Verdicts[key] = v
}

// proposal adds p to r once it holds up against f, its directory's forms,
// or says why it was dropped.
func (r *Result) proposal(p Proposal, f forms) {
	p.Evidence = f.evidencePath(p.Evidence)
	p.Globs = fromDir(p.Dir, p.Globs)
	p, dropped, err := f.proposal(p)
	if err != nil {
		r.Dropped = append(r.Dropped, fmt.Sprintf("proposal %q: %v", p.Command, err))
		return
	}
	for _, d := range dropped {
		r.Dropped = append(r.Dropped, fmt.Sprintf("proposal %q: %s", p.Command, d))
	}
	r.Proposals = append(r.Proposals, p)
}

// fromDir is globs as they match from dir, which the plan reads them
// from. The classifier sometimes writes them from the root (go/**/*.go
// for dir go), so dir's own prefix goes, from each alternative of a
// leading brace group too ({go/a.go,go/b.go}).
func fromDir(dir string, globs []string) []string {
	dir = filepath.ToSlash(filepath.Clean(dir))
	trim := func(g string) string {
		g = strings.TrimPrefix(g, "./")
		if dir != "." {
			g = strings.TrimPrefix(g, dir+"/")
		}
		return g
	}
	out := make([]string, 0, len(globs))
	for _, g := range globs {
		if alts, rest, ok := leadingGroup(g); ok {
			for i := range alts {
				alts[i] = trim(alts[i])
			}
			g = "{" + strings.Join(alts, ",") + "}" + rest
		}
		out = append(out, trim(g))
	}
	return out
}

// leadingGroup splits a glob that opens with a brace group into the
// group's top-level alternatives and what follows its closing brace.
func leadingGroup(glob string) (alts []string, rest string, ok bool) {
	if !strings.HasPrefix(glob, "{") {
		return nil, "", false
	}
	depth, start := 0, 1
	for i, r := range glob {
		switch {
		case r == '{':
			depth++
		case r == '}':
			if depth--; depth == 0 {
				return append(alts, glob[start:i]), glob[i+1:], true
			}
		case r == ',' && depth == 1:
			alts, start = append(alts, glob[start:i]), i+1
		}
	}
	return nil, "", false
}

// coverage drops each covered_by claim but one naming another verified check
// of the same kind in the same directory that claims no coverer of its own:
// a wrong claim drops a check from the gate, so anything less goes. It
// returns what it dropped.
func coverage(entries []sources.Entry, verdicts map[string]Verdict) []string {
	byKey := map[string]sources.Entry{}
	for _, e := range entries {
		byKey[Key(e)] = e
	}
	// Judged against the claims as answered, so the order verdicts are
	// visited in never changes the outcome.
	claims := map[string]string{}
	for key, v := range verdicts {
		if v.CoveredBy != "" {
			claims[key] = v.CoveredBy
		}
	}
	var dropped []string
	for _, key := range slices.Sorted(maps.Keys(claims)) {
		v, e := verdicts[key], byKey[key]
		coverer, verified := verdicts[v.CoveredBy]
		var why string
		switch {
		case !verified:
			why = "no verified entry has that id"
		case v.Role != "check" || v.Kind == "" || coverer.Role != "check" || coverer.Kind != v.Kind:
			why = "not a check of the same kind"
		case byKey[v.CoveredBy].Dir != e.Dir:
			why = "in another directory"
		case claims[v.CoveredBy] != "":
			why = "its coverer claims a coverer too"
		default:
			continue
		}
		dropped = append(dropped, e.Source+" "+e.Name+": covered_by: "+why)
		v.CoveredBy = ""
		verdicts[key] = v
	}
	return dropped
}

// denied is the deny_commands token e's body holds, "" when none.
func denied(cfg *config.Config, e sources.Entry) string {
	return cfg.GateDiscovery.DeniedCommand(strings.Fields(e.Body.Text))
}

// proposalRoles are the roles a proposal may take.
var proposalRoles = []string{"check", "fixer"}

// checkCommand is an error unless role is one of allowed, a check's kind is
// one the schema lists, and a check's words set no deny_flags flag. With
// aggregates, a check of several kinds has none.
func (f forms) checkCommand(role, kind string, words, allowed []string, aggregates bool) error {
	if !slices.Contains(allowed, role) {
		return fmt.Errorf("role %q", role)
	}
	if role != "check" {
		return nil
	}
	if (!aggregates || kind != "") && !slices.Contains(kinds, kind) {
		return fmt.Errorf("check kind %q", kind)
	}
	if flag := f.cfg.DeniedFlag(words); flag != "" {
		return fmt.Errorf("a check that sets %s", flag)
	}
	return nil
}

// managers are the facts of ms that verify, each manager named alone;
// dropped says why the rest don't.
func managers(cfg *config.Config, root string, ms []Manager) (kept []Manager, dropped []string) {
	for _, m := range ms {
		m.Manager, _, _ = strings.Cut(m.Manager, "@") // yarn@4.5.0 names yarn
		if err := checkManager(cfg, root, m); err != nil {
			dropped = append(dropped, fmt.Sprintf("manager for %s: %v", m.Dir, err))
			continue
		}
		kept = append(kept, m)
	}
	return kept, dropped
}

// checkManager is an error unless m cites a lockfile in its directory, or
// that directory's package.json packageManager naming m's manager.
func checkManager(cfg *config.Config, root string, m Manager) error {
	dir := filepath.Join(root, m.Dir)
	if m.Cites == "package.json#packageManager" {
		pm := sources.JSONValue(filepath.Join(dir, "package.json"), ".packageManager")
		if name, _, _ := strings.Cut(pm, "@"); m.Manager == "" || name != m.Manager {
			return fmt.Errorf("packageManager is %q, not %s", pm, m.Manager)
		}
		return nil
	}
	if !guard.IsGuardedLockfile(cfg, m.Cites) || !fsx.IsFile(filepath.Join(dir, m.Cites)) {
		return fmt.Errorf("no lockfile %s", m.Cites)
	}
	return nil
}

// forms checks commands an answer gives against the place they run.
type forms struct {
	cfg       *config.Config
	root, dir string
	prefixes  []string // the verified managers' run prefixes
	resolver  *resolve.Resolver
}

// verdict is v once it holds up against e: its role and kind, its segments
// copied from the body, and no check setting a deny_flags flag; or an
// error. A form that doesn't hold up is dropped, with why; the rest of the
// verdict stands.
func (f forms) verdict(e sources.Entry, v Verdict) (Verdict, []string, error) {
	// With segments, each is checked on its own words.
	words := strings.Fields(e.Body.Text)
	if len(v.Segments) > 0 {
		words = nil
	}
	if err := f.checkCommand(v.Role, v.Kind, words, roles, true); err != nil {
		return v, nil, err
	}
	if v.Role != "check" {
		v.Kind = ""
	}
	bodies, err := f.checkSegments(e, v.Segments)
	if err != nil {
		return v, nil, err
	}
	return v, f.dropUnsupported(&v, bodies), nil
}

// checkSegments checks each of segs, in place, against e's body: its role
// and kind, copied from the body, and the env the model saw in front of
// e.Command() taken off. It returns the body and each segment's text, the
// texts a form may come from.
func (f forms) checkSegments(e sources.Entry, segs []Segment) ([]string, error) {
	body := Squeeze(e.Body.Text)
	bodies := []string{e.Body.Text}
	for i, s := range segs {
		if words := strings.Fields(s.Text); len(withoutEnv(words, e.Env)) < len(words) {
			s.Text = strings.Join(withoutEnv(words, e.Env), " ")
			segs[i].Text = s.Text
		}
		if err := f.checkCommand(s.Role, s.Kind, strings.Fields(s.Text), roles, true); err != nil {
			return nil, fmt.Errorf("segment %q: %w", s.Text, err)
		}
		if strings.TrimSpace(s.Text) == "" {
			return nil, fmt.Errorf("segment %q is empty", s.Text)
		}
		if !strings.Contains(body, Squeeze(s.Text)) {
			return nil, fmt.Errorf("segment %q isn't in the body", s.Text)
		}
		if s.Role != "check" {
			segs[i].Kind = ""
		}
		bodies = append(bodies, s.Text)
	}
	return bodies, nil
}

// dropUnsupported drops each of v's forms and lists that don't hold up
// against bodies and the project, and says why for each.
func (f forms) dropUnsupported(v *Verdict, bodies []string) []string {
	var dropped []string
	if why := f.fileForm(v.Role, v.FileForm, bodies); why != "" {
		dropped = append(dropped, fmt.Sprintf("file form %q: %s", v.FileForm, why))
		v.FileForm, v.Globs = "", nil
	}
	v.FileForm = skipUnknownFiles(v.FileForm)
	if why := f.checkForm(v.Role, v.Affected, bodies, "{base}"); why != "" {
		dropped = append(dropped, fmt.Sprintf("affected form %q: %s", v.Affected, why))
		v.Affected = ""
	}
	v.AlsoKinds = slices.DeleteFunc(v.AlsoKinds, func(k string) bool {
		bad := v.Role != "check" || !slices.Contains(kinds, k) || k == v.Kind
		if bad {
			dropped = append(dropped, fmt.Sprintf("also kind %q: not a check kind it adds", k))
		}
		return bad
	})
	v.Reaches = slices.DeleteFunc(v.Reaches, func(rel string) bool {
		dir := filepath.Join(f.root, rel)
		bad := v.Role != "check" || !filepath.IsLocal(rel) || !fsx.IsDir(dir)
		if bad {
			dropped = append(dropped, fmt.Sprintf("reaches %q: no directory in the project", rel))
		}
		return bad
	})
	return dropped
}

// fileForm is why form, the file form of a command in role whose own
// words are bodies, fails checkForm or drops the project's config; "" when
// it holds.
func (f forms) fileForm(role, form string, bodies []string) string {
	why := f.checkForm(role, form, bodies, "{files}", "{dirs}")
	if why == "" && form != "" {
		why = wholeInputTools[f.wholeInputTool(form)]
	}
	return why
}

// wholeInputTools go wrong once given files, each with why. The TypeScript
// docs say "When input files are specified on the command line,
// tsconfig.json files are ignored", and vue-tsc takes tsc's arguments.
// deadcode over two of this repo's packages reported 94 functions that
// ./... reports none of.
var wholeInputTools = map[string]string{
	"tsc":      "tsc ignores tsconfig.json once given files",
	"vue-tsc":  "tsc ignores tsconfig.json once given files",
	"deadcode": "deadcode given some packages reports what only the others call",
}

// skipUnknownFiles is form with prettier told to skip a changed file it has
// no parser for (.prettierignore, a lock file), which otherwise fails the
// whole check.
func skipUnknownFiles(form string) string {
	words := strings.Fields(form)
	if !slices.Contains(words, "prettier") || slices.Contains(words, "--ignore-unknown") || slices.Contains(words, "-u") {
		return form
	}
	return strings.Replace(form, "{files}", "--ignore-unknown {files}", 1)
}

// toolOf is the tool words run: the first word, or the tool go tool runs.
func toolOf(words []string) string {
	if len(words) > 2 && filepath.Base(words[0]) == "go" && words[1] == "tool" {
		return filepath.Base(words[2])
	}
	return filepath.Base(words[0])
}

// wholeInputTool is the one of wholeInputTools body gives files to,
// behind a run prefix or a tool runner, or through a package script, up
// to three scripts deep; "" for none.
func (f forms) wholeInputTool(body string) string {
	tp, ok := sources.Provider("package-scripts")
	return f.runsWholeInput(body, 0, func(rel string) map[string]sources.Body {
		if !ok {
			return nil
		}
		return tp.Bodies(filepath.Join(f.dir, rel, "package.json"))
	})
}

// runsWholeInput is wholeInputTool at depth scripts down, reading each
// directory's package scripts with scripts.
func (f forms) runsWholeInput(body string, depth int, scripts func(rel string) map[string]sources.Body) string {
	if depth > 3 {
		return ""
	}
	lookup := func(provider, rel, name string) bool {
		_, ok := scripts(rel)[name]
		return provider == "package-scripts" && ok
	}
	// As written for pnpm exec tsc and pnpm run x; unprefixed for a run
	// prefix that's neither a tool runner nor a reference.
	words := strings.Fields(body)
	for _, b := range []string{body, strings.Join(f.unprefixed(words), " ")} {
		for _, c := range bodyCommands(b, lookup) {
			if tool := f.commandRuns(c, depth, scripts); tool != "" {
				return tool
			}
		}
	}
	return ""
}

// bodyCommands is b's commands. Unparsed, a command may start at any of
// its words: a wrong match only drops a file form, and Stop runs the whole
// command.
func bodyCommands(b string, lookup shellcalls.Lookup) []shellcalls.Command {
	parsed := shellcalls.Body(b, lookup)
	if parsed.Opaque != "" {
		words := shellWords(b)
		for i := range words {
			parsed.Commands = append(parsed.Commands, shellcalls.Command{Kind: shellcalls.Other, Words: words[i:]})
		}
	}
	return parsed.Commands
}

// commandRuns is the whole-input tool c gives files to, "" for none. Only
// a command given the files counts. A script's arguments land at the end
// of its body, so its body is read with them there.
func (f forms) commandRuns(c shellcalls.Command, depth int, scripts func(rel string) map[string]sources.Body) string {
	switch {
	case c.Kind == shellcalls.Other && len(c.Words) > 0:
		if tool := toolOf(c.Words); wholeInputTools[tool] != "" && slices.ContainsFunc(c.Words, holdsFiles) {
			return tool
		}
	case c.Kind == shellcalls.Ref:
		for _, ref := range c.Refs {
			if ref.Provider == "package-scripts" && slices.ContainsFunc(ref.Args, holdsFiles) {
				if tool := f.runsWholeInput(scripts(ref.Dir)[ref.Name].Text+" {files}", depth+1, scripts); tool != "" {
					return tool
				}
			}
		}
	}
	return ""
}

// shellWords is body split at blanks, quotes, and the shell's operators.
func shellWords(body string) []string {
	return strings.FieldsFunc(body, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(`"'$()|;&`+"`", r)
	})
}

// evidencePath is evidence when it's a file, else its first word when that
// is one: the model annotates paths ("knip.json (knip in devDependencies)").
func (f forms) evidencePath(evidence string) string {
	first, _, annotated := strings.Cut(evidence, " ")
	if annotated && !f.isFile(evidence) && f.isFile(first) {
		return first
	}
	return evidence
}

// isFile is true for a file under the root or the directory the command runs in.
func (f forms) isFile(rel string) bool {
	return fsx.IsFile(filepath.Join(f.root, rel)) || fsx.IsFile(filepath.Join(f.dir, rel))
}

// withoutEnv is words without the leading words that are env's own K=V.
func withoutEnv(words, env []string) []string {
	for len(words) > 0 && slices.Contains(env, words[0]) {
		words = words[1:]
	}
	return words
}
