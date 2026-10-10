// Package tools resolves the binaries the catalog engine runs (the full
// gate's old path and the per-edit formatters) and finds a tool's config.
// The enforcement engine's own resolution is the resolve package.
package tools

import (
	"path/filepath"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Claim is where a tool's config was found: the directory a tool reading
// it runs from, and the evidence, for kit explain.
type Claim struct {
	Dir string
	Why string
}

// HasSignal finds a tool's config walking up from dir to root: one of
// files by name, else a "<file> <dotted path>" TOML table. The TOML walk
// goes past a nearer file without the table, as ruff's lookup does.
func HasSignal(files []string, toml, dir, root string) (Claim, bool) {
	if len(files) > 0 {
		if found := fsx.FindUp(dir, root, files...); found != "" {
			return Claim{filepath.Dir(found), "config " + fsx.Rel(root, found)}, true
		}
	}
	if toml == "" {
		return Claim{}, false
	}
	file, table, _ := strings.Cut(toml, " ")
	for from := dir; ; {
		found := fsx.FindUp(from, root, file)
		if found == "" {
			return Claim{}, false
		}
		if sources.TOMLHas(found, table) {
			return Claim{filepath.Dir(found), "[" + strings.TrimPrefix(table, ".") + "] in " + fsx.Rel(root, found)}, true
		}
		if filepath.Dir(found) == root {
			return Claim{}, false
		}
		from = filepath.Dir(filepath.Dir(found))
	}
}
