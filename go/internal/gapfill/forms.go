package gapfill

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	// Aliased: gapfill has a classify function of its own.
	shellcalls "github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

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
	p.FileForm = skipUnknownFiles(p.FileForm)
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
