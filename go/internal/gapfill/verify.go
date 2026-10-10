package gapfill

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/resolve"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// verify turns s's answers into a Result. Policy first: an entry the deny
// lists catch is denied, answered or not. Then the project's files: manager
// facts must cite a lockfile or field that exists, a verdict's segments
// must be copied from its body, and a form must be its body's own words
// and never fetch, or it is dropped.
func verify(cfg *config.Config, root string, entries []sources.Entry, s store, failure string) Result {
	r := Result{Verdicts: map[string]Verdict{}, Skipped: map[string]string{}}
	var prefixes []string
	for _, m := range s.Managers {
		m.Manager, _, _ = strings.Cut(m.Manager, "@") // yarn@4.5.0 names yarn
		if err := checkManager(cfg, root, m); err != nil {
			r.Dropped = append(r.Dropped, fmt.Sprintf("manager for %s: %v", m.Dir, err))
			continue
		}
		r.Managers = append(r.Managers, m)
		prefixes = append(prefixes, m.RunPrefix)
	}
	resolver := resolve.New(root, nil)
	at := func(dir string) forms {
		return forms{root: root, dir: filepath.Join(root, dir), prefixes: prefixes, resolver: resolver}
	}
	unclassified := "unclassified"
	if failure != "" {
		unclassified += " (" + failure + ")"
	}
	for _, e := range entries {
		key := Key(e)
		if token := denied(cfg, e); token != "" {
			r.Skipped[key] = "denied (" + token + ")"
			continue
		}
		a := s.Entries[key]
		if a == nil {
			r.Skipped[key] = unclassified
			continue
		}
		v, dropped, err := at(e.Dir).verdict(e, a.Verdict)
		if err != nil {
			r.Skipped[key] = "rejected (" + err.Error() + ")"
			continue
		}
		for _, d := range dropped {
			r.Dropped = append(r.Dropped, e.Source+" "+e.Name+": "+d)
		}
		r.Verdicts[key] = v
	}
	for _, p := range s.Proposals {
		if err := at(p.Dir).proposal(cfg, p); err != nil {
			r.Dropped = append(r.Dropped, fmt.Sprintf("proposal %q: %v", p.Command, err))
			continue
		}
		if p.Role != "check" {
			p.Kind = ""
		}
		r.Proposals = append(r.Proposals, p)
	}
	return r
}

// denied is the deny_commands token e's body holds, "" when none.
func denied(cfg *config.Config, e sources.Entry) string {
	return cfg.GateDiscovery.DeniedCommand(strings.Fields(e.Body.Text))
}

// checkRole is an error for a role the schema doesn't list, or a check
// kind it doesn't. A check of several kinds, an aggregate, has none.
func checkRole(role, kind string) error {
	if !slices.Contains(roles, role) {
		return fmt.Errorf("role %q", role)
	}
	if role == "check" && kind != "" && !slices.Contains(kinds, kind) {
		return fmt.Errorf("check kind %q", kind)
	}
	return nil
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
	if !lockfile(cfg, m.Cites) || !fsx.IsFile(filepath.Join(dir, m.Cites)) {
		return fmt.Errorf("no lockfile %s", m.Cites)
	}
	return nil
}

// forms checks commands an answer gives against the place they run.
type forms struct {
	root, dir string
	prefixes  []string // the verified managers' run prefixes
	resolver  *resolve.Resolver
}

// verdict is v once it holds up against e: its role and kind, and its
// segments copied from the body, or an error. A form that doesn't hold up
// is dropped, with why; the rest of the verdict stands.
func (f forms) verdict(e sources.Entry, v Verdict) (Verdict, []string, error) {
	if err := checkRole(v.Role, v.Kind); err != nil {
		return v, nil, err
	}
	if v.Role != "check" {
		v.Kind = ""
	}
	body := strings.Join(strings.Fields(e.Body.Text), " ")
	bodies := []string{e.Body.Text}
	for i, s := range v.Segments {
		if err := checkRole(s.Role, s.Kind); err != nil {
			return v, nil, fmt.Errorf("segment %q: %w", s.Text, err)
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
	if why := f.form(v.FileForm, bodies, "{files}", "{dirs}"); v.FileForm != "" && why != "" {
		dropped = append(dropped, fmt.Sprintf("file form %q: %s", v.FileForm, why))
		v.FileForm, v.Globs = "", nil
	}
	if why := f.form(v.Affected, bodies, "{base}"); v.Affected != "" && why != "" {
		dropped = append(dropped, fmt.Sprintf("affected form %q: %s", v.Affected, why))
		v.Affected = ""
	}
	return v, dropped, nil
}

// proposal is an error unless p is a check of a known kind, or a fixer,
// whose evidence exists, which the deny list allows, and which never
// fetches. Whether the project already enforces its kind where it runs is
// the plan's call.
func (f forms) proposal(cfg *config.Config, p Proposal) error {
	words := strings.Fields(p.Command)
	switch {
	case len(words) == 0:
		return fmt.Errorf("no command")
	case p.Role != "check" && p.Role != "fixer":
		return fmt.Errorf("role %q", p.Role)
	case p.Role == "check" && !slices.Contains(kinds, p.Kind):
		return fmt.Errorf("check kind %q", p.Kind)
	case p.Evidence == "" || !fsx.IsFile(filepath.Join(f.root, p.Evidence)) && !fsx.IsFile(filepath.Join(f.dir, p.Evidence)):
		return fmt.Errorf("no evidence file %q", p.Evidence)
	}
	if token := cfg.GateDiscovery.DeniedCommand(words); token != "" {
		return fmt.Errorf("denied (%s)", token)
	}
	if why := f.resolver.Fetches(f.dir, words); why != "" {
		return fmt.Errorf("fetches (%s)", why)
	}
	if why := f.form(p.FileForm, []string{p.Command}, "{files}", "{dirs}"); p.FileForm != "" && why != "" {
		return fmt.Errorf("file form %q: %s", p.FileForm, why)
	}
	return nil
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
		case placeholder(w) && i < len(source) && f.pathLike(source[i]):
			for i < len(source) && f.pathLike(source[i]) {
				i++
			}
		case strings.HasPrefix(w, "-") || placeholder(w):
		default:
			return false
		}
	}
	return i == len(source)
}

func placeholder(w string) bool {
	return strings.Contains(w, "{files}") || strings.Contains(w, "{dirs}") || strings.Contains(w, "{base}")
}

// pathLike is true for a word naming files: ".", a ./... package pattern,
// a glob, or a path that exists where the command runs.
func (f forms) pathLike(w string) bool {
	return w == "." || strings.HasSuffix(w, "/...") || strings.ContainsAny(w, "*?[") || fsx.IsFile(filepath.Join(f.dir, w)) || fsx.IsDir(filepath.Join(f.dir, w))
}
