package hooks

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/adr"
	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// ADRNudge names each accepted ADR that cites a Read's or an edit's file,
// or a folder holding it, with its title and Decision, once per ADR per
// session, and lets the call through.
func ADRNudge(h *hook.Hook) error {
	p := h.Payload
	if p.Err != nil {
		return p.Err
	}
	if h.RuleDisabled("adr-nudge") {
		return nil
	}
	tool := p.String(hook.KeyToolName)
	file := p.FilePath()
	if file == "" || (tool != "Read" && !slices.Contains(filePathTools, tool)) || h.Paths.IsScratch(file) {
		return nil
	}
	// Root names the repo with symlinks resolved, so the file is too.
	file = fsx.PhysicalPath(file)
	root, anchored := project.Root(filepath.Dir(file))
	dir := adr.Dir(root)
	if !anchored || dir == "" || fsx.Within(dir, file) {
		return nil
	}
	if named := adrsCiting(h, root, dir, filepath.ToSlash(fsx.Rel(root, file))); named != "" {
		h.AddContext("PreToolUse", named+"A change that contradicts it needs a new ADR that supersedes it.")
	}
	return nil
}

// adrsCiting is a line for each accepted ADR in dir that cites rel and
// wasn't named yet this session, marking each named.
func adrsCiting(h *hook.Hook, root, dir, rel string) string {
	var b strings.Builder
	for _, a := range adr.Accepted(dir) {
		marker := h.Paths.SessionFile(cache.ADRNudged, h.Payload.SessionID(), cache.RootKey(a.Path))
		if marker == "" || fsx.IsFile(marker) || !a.Cites(rel) {
			continue
		}
		if !h.DryRun {
			_ = fsx.WriteAtomic(marker, nil, 0o600)
		}
		b.WriteString("An accepted ADR covers this file, " + fsx.Rel(root, a.Path) + ": " + a.Title + "\nDecision: " + a.Decision + "\n")
	}
	return b.String()
}
