package sources

import "strings"

// Source is how the kit runs the entries of one kind of source, beyond
// what a task provider's row says. The zero Source is a task runner's.
type Source struct {
	// CI is a CI provider: its steps are the full gate's own.
	CI bool
	// Graph is a task graph: one check fans out across workspaces.
	Graph bool
	// GitHook is a source only a git hook runs: the full gate leaves it out.
	GitHook bool
	// DefaultStage is the stage of an entry that names none.
	DefaultStage string
	// PreCommit is how entry e runs at pre-commit on {files}; all is true
	// when the tool filters files itself and takes every one. nil when it
	// has no per-file form.
	PreCommit func(e Entry) (body string, all bool)
	// CommitMsg is how entry e checks the message file {files}; nil when
	// the source has no commit-msg form.
	CommitMsg func(e Entry) string
	// WholeStage is true when CommitMsg runs every hook of the stage at
	// once, so its gate names the file, not the entry.
	WholeStage bool
	// Fallback is true when CommitMsg applies only where no hook manager
	// runs a commit-msg check.
	Fallback bool
}

var sourceKinds = map[string]Source{
	"github-actions": {CI: true},
	"gitlab-ci":      {CI: true},
	"turbo":          {Graph: true},
	"nx":             {Graph: true},
	"pre-commit": {
		DefaultStage: "pre-commit",
		PreCommit: func(e Entry) (string, bool) {
			return "pre-commit run " + e.Name + " --hook-stage pre-commit --files {files}", true
		},
		CommitMsg:  func(Entry) string { return "pre-commit run --hook-stage commit-msg --commit-msg-filename {files}" },
		WholeStage: true,
	},
	"lint-staged": {GitHook: true, PreCommit: matchedFiles},
	"lefthook": {
		GitHook:   true,
		PreCommit: matchedFiles,
		CommitMsg: func(e Entry) string { return strings.ReplaceAll(e.Body.Text, "{1}", "{files}") },
	},
	// A husky hook reads the message file as $1.
	"husky":      {GitHook: true, CommitMsg: func(e Entry) string { return "set -- {files}\n" + e.Body.Text }},
	"commitlint": {GitHook: true, CommitMsg: func(e Entry) string { return e.Body.Text + " {files}" }, Fallback: true},
}

// matchedFiles runs an entry on the files its globs match.
func matchedFiles(e Entry) (string, bool) { return e.FileBody(), false }

// SourceOf is the Source of entries from source.
func SourceOf(source string) Source { return sourceKinds[source] }
