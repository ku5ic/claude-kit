package enforce

import (
	"path/filepath"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// CommitMsg plans the project's commit-msg enforcement on msg, a file
// holding the message a commit is about to get: pre-commit's commit-msg
// hooks, and husky's and lefthook's commit-msg hooks; commitlint's config
// alone only when no hook manager runs one. A command at commit-msg checks
// the message, so it needs no verdict; its binaries go through resolve.
func CommitMsg(cfg *config.Config, o Options, msg string) Plan {
	b := hookPlan(cfg, o)
	var fallback *Gate
	for _, e := range b.entries {
		src := sources.SourceOf(e.Source)
		if e.Stage != "commit-msg" || src.CommitMsg == nil {
			continue
		}
		name := e.File + ": " + e.Name
		if src.WholeStage {
			name = e.File
		}
		g := Gate{Label: "commit-msg (" + name + ")", Dir: filepath.Join(b.root, e.Dir), Env: e.Env, Files: []string{msg}, Verdict: "commit-msg", Body: src.CommitMsg(e)}
		if src.Fallback {
			fallback = &g
			continue
		}
		b.add(g)
	}
	if fallback != nil && len(b.gates) == 0 {
		b.add(*fallback)
	}
	b.settle(o.CacheDir)
	return Plan{Root: o.Root, Gates: b.gates}
}
