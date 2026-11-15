package gapfill

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

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
	unclassified, now := unclassifiedReason, time.Now()
	if failure != "" {
		unclassified += " (" + failure + ")"
	}
	for _, e := range entries {
		key := Key(e)
		if token := denied(cfg, e); token != "" {
			r.Skipped[key] = deniedReason + " (" + token + ")"
			continue
		}
		a := s.Entries[key]
		if a == nil {
			r.Skipped[key] = unclassified
			if retry := s.retryAt(cfg, key, now); failure == "" && !retry.IsZero() {
				r.Skipped[key] = coolingReason + retry.Format("15:04") + ")"
			}
			continue
		}
		v, dropped, err := at(e.Dir).verdict(e, a.Verdict)
		if err != nil {
			r.Skipped[key] = "rejected (" + err.Error() + ")"
			continue
		}
		v.Globs = fromDir(e.Dir, v.Globs)
		for _, d := range dropped {
			r.Dropped = append(r.Dropped, e.Source+" "+e.Name+": "+d)
		}
		r.Verdicts[key] = v
	}
	r.Dropped = append(r.Dropped, coverage(entries, r.Verdicts)...)
	for _, p := range s.Proposals {
		p.Evidence = at(p.Dir).evidencePath(p.Evidence)
		p.Globs = fromDir(p.Dir, p.Globs)
		p, dropped, err := at(p.Dir).proposal(p)
		if err != nil {
			r.Dropped = append(r.Dropped, fmt.Sprintf("proposal %q: %v", p.Command, err))
			continue
		}
		for _, d := range dropped {
			r.Dropped = append(r.Dropped, fmt.Sprintf("proposal %q: %s", p.Command, d))
		}
		r.Proposals = append(r.Proposals, p)
	}
	return r
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
	body := strings.Join(strings.Fields(e.Body.Text), " ")
	bodies := []string{e.Body.Text}
	for i, s := range v.Segments {
		// The model saw e.Command(), the env in front; the body has none.
		if words := strings.Fields(s.Text); len(withoutEnv(words, e.Env)) < len(words) {
			s.Text = strings.Join(withoutEnv(words, e.Env), " ")
			v.Segments[i].Text = s.Text
		}
		if err := f.checkCommand(s.Role, s.Kind, strings.Fields(s.Text), roles, true); err != nil {
			return v, nil, fmt.Errorf("segment %q: %w", s.Text, err)
		}
		if strings.TrimSpace(s.Text) == "" {
			return v, nil, fmt.Errorf("segment %q is empty", s.Text)
		}
		if !strings.Contains(body, strings.Join(strings.Fields(s.Text), " ")) {
			return v, nil, fmt.Errorf("segment %q isn't in the body", s.Text)
		}
		if s.Role != "check" {
			v.Segments[i].Kind = ""
		}
		bodies = append(bodies, s.Text)
	}
	var dropped []string
	if why := f.fileForm(v.Role, v.FileForm, bodies); why != "" {
		dropped = append(dropped, fmt.Sprintf("file form %q: %s", v.FileForm, why))
		v.FileForm, v.Globs = "", nil
	}
	if why := f.checkForm(v.Role, v.Affected, bodies, "{base}"); why != "" {
		dropped = append(dropped, fmt.Sprintf("affected form %q: %s", v.Affected, why))
		v.Affected = ""
	}
	return v, dropped, nil
}

// fileForm is why form, the file form of a command in role whose own
// words are bodies, fails checkForm or drops the project's config; "" when
// it holds.
func (f forms) fileForm(role, form string, bodies []string) string {
	why := f.checkForm(role, form, bodies, "{files}", "{dirs}")
	if why == "" && form != "" && f.ignoresConfigOnFiles(form) {
		why = "tsc ignores tsconfig.json once given files"
	}
	return why
}

// configOnlyTools drop the project config once given files: the TypeScript
// docs say "When input files are specified on the command line,
// tsconfig.json files are ignored", and vue-tsc takes tsc's arguments.
var configOnlyTools = []string{"tsc", "vue-tsc"}

// ignoresConfigOnFiles is true when body runs one of configOnlyTools,
// behind a run prefix or a tool runner, or through a package script, up
// to three scripts deep.
func (f forms) ignoresConfigOnFiles(body string) bool {
	tp, ok := sources.Provider("package-scripts")
	return f.runsConfigOnly(body, 0, func(rel string) map[string]sources.Body {
		if !ok {
			return nil
		}
		return tp.Bodies(filepath.Join(f.dir, rel, "package.json"))
	})
}

// runsConfigOnly is ignoresConfigOnFiles at depth scripts down, reading
// each directory's package scripts with scripts.
func (f forms) runsConfigOnly(body string, depth int, scripts func(rel string) map[string]sources.Body) bool {
	if depth > 3 {
		return false
	}
	lookup := func(provider, rel, name string) bool {
		_, ok := scripts(rel)[name]
		return provider == "package-scripts" && ok
	}
	// Only a command given the files counts. A script's arguments land at
	// the end of its body, so its body is read with them there.
	runs := func(c shellcalls.Command) bool {
		switch {
		case c.Kind == shellcalls.Other && len(c.Words) > 0:
			return slices.Contains(configOnlyTools, filepath.Base(c.Words[0])) && slices.ContainsFunc(c.Words, holdsFiles)
		case c.Kind == shellcalls.Ref:
			return slices.ContainsFunc(c.Refs, func(ref shellcalls.TaskRef) bool {
				return ref.Provider == "package-scripts" && slices.ContainsFunc(ref.Args, holdsFiles) &&
					f.runsConfigOnly(scripts(ref.Dir)[ref.Name].Text+" {files}", depth+1, scripts)
			})
		}
		return false
	}
	// As written for pnpm exec tsc and pnpm run x; unprefixed for a run
	// prefix that's neither a tool runner nor a reference.
	words := strings.Fields(body)
	return slices.ContainsFunc([]string{body, strings.Join(f.unprefixed(words), " ")}, func(b string) bool {
		return slices.ContainsFunc(shellcalls.Body(b, lookup).Commands, runs)
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

// checkForm is why form, of an entry in role, fails form, with a check's
// form also turned away for setting a deny_flags flag; "" for no form.
func (f forms) checkForm(role, form string, bodies []string, placeholders ...string) string {
	if form == "" {
		return ""
	}
	if flag := f.cfg.DeniedFlag(strings.Fields(form)); role == "check" && flag != "" {
		return "sets " + flag
	}
	if why := f.form(form, bodies, placeholders...); why != "" {
		return why
	}
	// sameWords lets any added word starting with "-" through as a flag.
	if !shellcalls.Single(form) {
		return "not one plain command"
	}
	return ""
}

// proposal is p once it holds up: a check of a known kind, or a fixer,
// whose evidence exists, which the deny lists allow, and which never
// fetches; or an error. A file form that doesn't hold up is dropped, with
// why, as a verdict's is. Whether the project already enforces its kind
// where it runs is the plan's call.
func (f forms) proposal(p Proposal) (Proposal, []string, error) {
	words := strings.Fields(p.Command)
	if len(words) == 0 {
		return p, nil, fmt.Errorf("no command")
	}
	if err := f.checkCommand(p.Role, p.Kind, words, proposalRoles, false); err != nil {
		return p, nil, err
	}
	switch {
	// Model output: a second command, a redirect, or eval could hide in it.
	case !shellcalls.Single(p.Command):
		return p, nil, fmt.Errorf("not one plain command")
	case p.Evidence == "" || !f.isFile(p.Evidence):
		return p, nil, fmt.Errorf("no evidence file %q", p.Evidence)
	}
	if token := f.cfg.GateDiscovery.DeniedCommand(words); token != "" {
		return p, nil, fmt.Errorf("denied (%s)", token)
	}
	if why := f.resolver.Fetches(f.dir, words); why != "" {
		return p, nil, fmt.Errorf("fetches (%s)", why)
	}
	if p.Role != "check" {
		p.Kind = ""
	}
	var dropped []string
	if why := f.fileForm(p.Role, p.FileForm, []string{p.Command}); why != "" {
		dropped = append(dropped, fmt.Sprintf("file form %q: %s", p.FileForm, why))
		p.FileForm, p.Globs = "", nil
	}
	return p, dropped, nil
}

// form is why form isn't one of bodies' own words, behind at most a
// verified run prefix, with path arguments replaced by placeholders and
// only flags and placeholders added; or why it fetches, or lacks every one
// of placeholders. "" when it holds up.
func (f forms) form(form string, bodies []string, placeholders ...string) string {
	if !slices.ContainsFunc(placeholders, func(p string) bool { return strings.Contains(form, p) }) {
		return "no " + strings.Join(placeholders, " or ")
	}
	words := strings.Fields(form)
	if why := f.resolver.Fetches(f.dir, words); why != "" {
		return "fetches (" + why + ")"
	}
	for _, b := range bodies {
		source := strings.Fields(b)
		if f.sameWords(words, source) || f.sameWords(f.unprefixed(words), source) {
			return ""
		}
	}
	return "not the body's own words"
}

// unprefixed is words without a verified run prefix in front.
func (f forms) unprefixed(words []string) []string {
	for _, p := range f.prefixes {
		if prefix := strings.Fields(p); len(prefix) > 0 && len(words) > len(prefix) && slices.Equal(words[:len(prefix)], prefix) {
			return words[len(prefix):]
		}
	}
	return words
}

// sameWords is true when form is source, in order, with runs of path
// arguments replaced by placeholder words and flags or placeholders added.
func (f forms) sameWords(form, source []string) bool {
	i := 0
	for _, w := range form {
		switch {
		case i < len(source) && w == source[i]:
			i++
		case placeholder(w) && f.pathRun(w, source[i:]) > 0:
			i += f.pathRun(w, source[i:])
		case strings.HasPrefix(w, "-") || placeholder(w):
		default:
			return false
		}
	}
	return i == len(source)
}

// pathRun is how many of words, from the first, the placeholder word w
// stands for: the longest run of path arguments that starts with the text
// before w's placeholder and ends with the text after it, as `.)"` is to
// `{files})"`.
func (f forms) pathRun(w string, words []string) int {
	before, after := around(w)
	run := 0
	for n, word := range words {
		if n == 0 {
			var ok bool
			if word, ok = strings.CutPrefix(word, before); !ok {
				return 0
			}
		}
		if path, ok := strings.CutSuffix(word, after); ok && f.pathLike(path) {
			run = n + 1
		}
		if !f.pathLike(word) {
			return run
		}
	}
	return run
}

var placeholders = []string{"{files}", "{dirs}", "{base}"}

func placeholder(w string) bool {
	return slices.ContainsFunc(placeholders, func(p string) bool { return strings.Contains(w, p) })
}

// around is the text before and after w's placeholder.
func around(w string) (before, after string) {
	for _, p := range placeholders {
		if before, after, ok := strings.Cut(w, p); ok {
			return before, after
		}
	}
	return "", ""
}

// pathLike is true for a word naming files: ".", a ./... package pattern,
// a glob, or a path that exists where the command runs.
func (f forms) pathLike(w string) bool {
	return w == "." || strings.HasSuffix(w, "/...") || strings.ContainsAny(w, "*?[") || fsx.IsFile(filepath.Join(f.dir, w)) || fsx.IsDir(filepath.Join(f.dir, w))
}

// holdsFiles is true when word takes a file form's files.
func holdsFiles(word string) bool {
	return strings.Contains(word, "{files}") || strings.Contains(word, "{dirs}")
}
