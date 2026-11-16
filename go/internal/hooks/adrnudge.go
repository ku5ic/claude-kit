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
	if file == "" || (tool != "Read" && !slices.Contains(filePathTools, tool)) {
		return nil
	}
	root, anchored := project.Root(filepath.Dir(file))
	dir := adr.Dir(root)
	if !anchored || dir == "" || fsx.Within(dir, file) {
		return nil
	}
	rel := filepath.ToSlash(fsx.Rel(root, file))
	var b strings.Builder
	for _, a := range adr.Accepted(dir) {
		marker := h.Paths.SessionFile(cache.ADRNudged, p.SessionID(), cache.RootKey(a.Path))
		if marker == "" || fsx.IsFile(marker) || !a.Cites(rel) {
			continue
		}
		if !h.DryRun {
			_ = fsx.WriteAtomic(marker, nil, 0o600)
		}
		b.WriteString("An accepted ADR covers this file, " + fsx.Rel(root, a.Path) + ": " + a.Title + "\nDecision: " + a.Decision + "\n")
	}
	if b.Len() > 0 {
		h.AddContext("PreToolUse", b.String()+"A change that contradicts it needs a new ADR that supersedes it.")
	}
	return nil
}
