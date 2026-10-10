package enforce

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Stop plans the checks the Stop hook runs on files, the turn's edits: the
// project's pre-commit, lint-staged, and lefthook checks, then, for each
// file and kind none of those checks, the file form of the full gate's
// check of that kind whose globs match it. Fixers never run here: they ran
// on each edit. A file the repo ignores, a deleted one, and one outside the
// root don't count. The classifier is never asked: the cache answers.
func Stop(cfg *config.Config, o Options, files []string) Plan {
	p := Plan{Root: o.Root}
	files = stopFiles(o.Root, files)
	if len(files) == 0 {
		return p
	}
	o.Ask = false
	b, _ := fullGate(cfg, o)
	full := b.gates
	b.gates, b.planned = nil, map[string]bool{}
	checked := b.hookChecks(files)
	for _, g := range full {
		b.fileForm(g, files, checked)
	}
	b.unclassified(func(e sources.Entry) bool {
		return (e.Stage == "" || e.Stage == "pre-commit") && e.Source != "husky" && e.Source != "commitlint"
	})
	b.resolve()
	skipMarked(b.gates, o.CacheDir, o.Root)
	p.Gates, p.Unclassified = b.gates, b.skipped
	return p
}

// stopFiles is files, physical and absolute, that exist under root and that
// the repo doesn't ignore, each once.
func stopFiles(root string, files []string) []string {
	var kept []string
	for _, f := range files {
		if f = fsx.PhysicalPath(f); strings.HasPrefix(f, root+"/") && fsx.IsFile(f) && !slices.Contains(kept, f) {
			kept = append(kept, f)
		}
	}
	ignored := git.Ignored(root, kept)
	return slices.DeleteFunc(kept, func(f string) bool { return ignored[f] })
}

// hookChecks plans the git-hook managers' pre-commit checks on files, and
// returns, per file, the kinds they check it for. pre-commit filters files
// itself, so its hooks check every file.
func (b *builder) hookChecks(files []string) map[string][]string {
	checked := map[string][]string{}
	for _, e := range b.entries {
		v, why := b.verdict(e)
		kinds := kindsOf(v)
		if why != "" || v.Role != "check" || !atPreCommit(e) {
			continue
		}
		g := Gate{Label: label(kinds, e.File+": "+e.Name, "."), Kinds: kinds, Dir: filepath.Join(b.root, e.Dir), Env: e.Env, Verdict: v.String()}
		switch e.Source {
		case "pre-commit":
			g.Body, g.Files = "pre-commit run "+e.Name+" --hook-stage pre-commit --files {files}", files
		case "lint-staged", "lefthook":
			g.Body = e.Body.Text
			if e.PassFiles {
				g.Body += " {files}"
			}
			g.Files = matching(files, b.root, e.Files)
		default:
			continue
		}
		if len(g.Files) == 0 {
			continue
		}
		b.add(g)
		for _, f := range g.Files {
			checked[f] = append(checked[f], kinds...)
		}
	}
	return checked
}

// atPreCommit is true for an entry git's pre-commit hook runs: one staged
// there, or a pre-commit hook whose repo's manifest sets its stages.
func atPreCommit(e sources.Entry) bool {
	return e.Stage == "pre-commit" || e.Source == "pre-commit" && e.Stage == ""
}

// fileForm plans full gate g's file form on the files under its directory
// its globs match, each file only for the kinds no hook checks it for.
func (b *builder) fileForm(g Gate, files []string, checked map[string][]string) {
	if g.Skip != "" || g.FileForm == "" || len(g.Globs) == 0 {
		return
	}
	var mine []string
	for _, f := range matching(files, g.Dir, g.Globs) {
		if slices.ContainsFunc(g.Kinds, func(k string) bool { return !slices.Contains(checked[f], k) }) {
			mine = append(mine, f)
		}
	}
	if len(mine) > 0 {
		b.add(Gate{Label: g.Label, Kinds: g.Kinds, Dir: g.Dir, Env: g.Env, Body: g.FileForm, Verdict: g.Verdict, Files: mine})
	}
}

// matching is the files under dir that globs match, as git-hook managers
// match them: relative to dir, a glob without a slash matching the base
// name, with {a,b} alternatives. No globs match every file under dir.
func matching(files []string, dir string, globs []string) []string {
	var out []string
	for _, f := range files {
		if !strings.HasPrefix(f, dir+"/") {
			continue
		}
		rel := fsx.Rel(dir, f)
		if len(globs) == 0 || slices.ContainsFunc(globs, func(glob string) bool { return globMatch(glob, rel) }) {
			out = append(out, f)
		}
	}
	return out
}

func globMatch(glob, rel string) bool {
	for _, pattern := range braces(strings.TrimPrefix(glob, "./")) {
		switch {
		case !strings.Contains(pattern, "/"):
			if guard.Glob(pattern, filepath.Base(rel)) {
				return true
			}
		case guard.Glob(pattern, rel):
			return true
		case strings.HasPrefix(pattern, "**/") && guard.Glob(strings.TrimPrefix(pattern, "**/"), rel):
			return true
		}
	}
	return false
}

// braces expands a glob's {a,b} alternatives, innermost first.
func braces(glob string) []string {
	end := strings.Index(glob, "}")
	start := strings.LastIndex(glob[:max(end, 0)], "{")
	if end < 0 || start < 0 {
		return []string{glob}
	}
	var out []string
	for alt := range strings.SplitSeq(glob[start+1:end], ",") {
		out = append(out, braces(glob[:start]+alt+glob[end+1:])...)
	}
	return out
}

// placeholderWord is a word holding {files} or {dirs}.
var placeholderWord = regexp.MustCompile(`\S*\{(?:files|dirs)\}\S*`)

// Command is g's body with its files in place: a word holding {files}
// repeated once per file, and one holding {dirs} once per file's directory,
// each relative to where it runs and shell-quoted.
func (g Gate) Command() string {
	if len(g.Files) == 0 {
		return g.Body
	}
	var files, dirs []string
	for _, f := range g.Files {
		rel := fsx.Rel(g.Dir, f)
		files = append(files, rel)
		if dir := filepath.Dir(rel); dir == "." {
			dirs = append(dirs, ".")
		} else {
			dirs = append(dirs, "./"+dir)
		}
	}
	slices.Sort(dirs)
	dirs = slices.Compact(dirs)
	return placeholderWord.ReplaceAllStringFunc(g.Body, func(word string) string {
		placeholder, values := "{files}", files
		if strings.Contains(word, "{dirs}") {
			placeholder, values = "{dirs}", dirs
		}
		out := make([]string, len(values))
		for i, v := range values {
			out[i] = strings.ReplaceAll(word, placeholder, quote(v))
		}
		return strings.Join(out, " ")
	})
}

// quote is s as one shell word.
func quote(s string) string {
	if q, err := syntax.Quote(s, syntax.LangBash); err == nil {
		return q
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
