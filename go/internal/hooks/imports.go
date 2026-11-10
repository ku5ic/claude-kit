package hooks

import (
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/pkgcheck"
)

// GuardImports blocks an edit that adds an import of a package no manifest
// declares, so a package that may not exist is confirmed on its registry
// and added with the project's package manager first.
// PreToolUse for Edit, Write, and MultiEdit, inside guard-dispatch.
func GuardImports(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	path, cfg := h.Payload.FilePath(), h.Config()
	if path == "" || cfg == nil {
		return nil
	}
	before, after, ok := written(h.Payload, path)
	if !ok {
		return nil
	}
	missing := pkgcheck.Undeclared(cfg.SkipDirs, path, before, after)
	if len(missing) == 0 {
		return nil
	}
	return h.Block("this edit imports "+strings.Join(missing, ", ")+", which no manifest declares. Confirm each exists on its registry, then add it with the project's package manager before importing it.", "pkg-undeclared")
}
