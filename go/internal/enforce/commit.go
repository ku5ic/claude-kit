package enforce

import (
	"path/filepath"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

// CommitMsg plans the project's commit-msg enforcement on msg, a file
// holding the message a commit is about to get: pre-commit's commit-msg
// hooks, and husky's and lefthook's commit-msg hooks; commitlint's config
// alone only when no hook manager runs one. A command at commit-msg checks
// the message, so it needs no verdict; its binaries go through resolve.
func CommitMsg(cfg *config.Config, o Options, msg string) Plan {
	o.Ask = false
	b, _ := plan(cfg, o)
	var commitlint *Gate
	for _, e := range b.entries {
		if e.Stage != "commit-msg" {
			continue
		}
		g := Gate{Label: "commit-msg (" + e.File + ": " + e.Name + ")", Dir: filepath.Join(b.root, e.Dir), Env: e.Env, Files: []string{msg}, Verdict: "commit-msg"}
		switch e.Source {
		case "pre-commit":
			g.Label, g.Body = "commit-msg ("+e.File+")", "pre-commit run --hook-stage commit-msg --commit-msg-filename {files}"
		case "husky":
			// A husky hook reads the message file as $1.
			g.Body = "set -- {files}\n" + e.Body.Text
		case "lefthook":
			g.Body = strings.ReplaceAll(e.Body.Text, "{1}", "{files}")
		case "commitlint":
			g.Body = e.Body.Text + " {files}"
			commitlint = &g
			continue
		default:
			continue
		}
		b.add(g)
	}
	if commitlint != nil && len(b.gates) == 0 {
		b.add(*commitlint)
	}
	b.settle(o.CacheDir)
	return Plan{Root: o.Root, Gates: b.gates}
}
