package enforce

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Edit plans the fixers for file, a file just edited: every pre-commit
// fixer of the git-hook managers that claims it, in their order; else the
// formatter the project states by evidence (its config, a dependency, its
// language's toolchain), when one claims it. Two that claim it plan one
// skip: which is right isn't the kit's call. The classifier is never asked.
func Edit(cfg *config.Config, o Options, file string) Plan {
	p := Plan{Root: o.Root}
	files := stopFiles(o.Root, []string{file})
	if len(files) == 0 {
		return p
	}
	o.Ask = false
	b, _ := plan(cfg, o)
	b.hookFixers(files)
	if len(b.gates) == 0 {
		b.evidenceFixer(files)
	}
	b.unclassified(func(e sources.Entry) bool {
		return atPreCommit(e) && (e.Source == "pre-commit" || gitHook(e.Source)) && len(matching(files, b.root, e.Files)) > 0
	})
	b.settle(o.CacheDir)
	p.Gates, p.Unclassified = b.gates, b.skipped
	return p
}

// OnEdit is the user's on_edit policy for file, under root: the formatter
// chain of the last rule whose globs match it, so an overlay's rule wins,
// and the note of every rule that does.
func OnEdit(cfg *config.Config, root, file string) (chain []config.EditCommand, notes []string) {
	for _, rule := range cfg.OnEdit {
		if len(matching([]string{file}, root, rule.Globs)) == 0 || len(rule.Globs) == 0 {
			continue
		}
		if len(rule.Run) > 0 {
			chain = rule.Run
		}
		if rule.Note != "" {
			notes = append(notes, rule.Note)
		}
	}
	return chain, notes
}

// Claims is true when a fixer of the project's claims the file: one that
// runs, or one that can't and so leaves it alone. An entry with no verdict
// claims nothing.
func (p Plan) Claims() bool {
	return slices.ContainsFunc(p.Gates, func(g Gate) bool { return g.Verdict != "" })
}

// hookFixers plans the git-hook managers' pre-commit fixers that claim
// files.
func (b *builder) hookFixers(files []string) {
	for _, e := range b.entries {
		if v, why := b.verdict(e); why != "" || v.Role != "fixer" || !atPreCommit(e) {
			continue
		}
		g := Gate{Label: "fix (" + e.File + ": " + e.Name + ")", Dir: filepath.Join(b.root, e.Dir), Env: e.Env, Verdict: "fixer"}
		switch e.Source {
		case "pre-commit":
			g.Body, g.Files = "pre-commit run "+e.Name+" --hook-stage pre-commit --files {files}", files
		case "lint-staged", "lefthook":
			g.Body = e.Body.Text
			if e.PassFiles {
				g.Body += " {files}"
			}
			g.Files = matching(files, b.root, e.Files)
		}
		if len(g.Files) > 0 {
			b.add(g)
		}
	}
}

// evidenceFixer plans the formatter a proposal claims files with; with
// more than one claiming them, a skip naming them.
func (b *builder) evidenceFixer(files []string) {
	var claiming []Gate
	for _, p := range b.facts.Proposals {
		dir := filepath.Join(b.root, p.Dir)
		if p.Role != "fixer" || p.FileForm == "" {
			continue
		}
		if mine := matching(files, dir, p.Globs); len(mine) > 0 && len(p.Globs) > 0 {
			claiming = append(claiming, Gate{Label: "fix (evidence " + p.Evidence + ")", Dir: dir, Body: p.FileForm, Verdict: "proposed: " + p.FileForm, Files: mine})
		}
	}
	switch len(claiming) {
	case 0:
	case 1:
		b.add(claiming[0])
	default:
		var names []string
		for _, g := range claiming {
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(g.Label, "fix ("), ")"))
		}
		b.gates = append(b.gates, Gate{Label: "fix", Verdict: "conflict", Skip: "left alone: " + strings.Join(names, " and ") + " each format it"})
	}
}
