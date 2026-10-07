package tools

import (
	"path/filepath"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// Toolchain is how a toolchain_checks entry runs in one directory.
type Toolchain struct {
	Words []string // what runs; nil when it can't (Skip says why)
	Shown []string // Words with the binary as <tooling> shows it
	Resolution
}

// ResolveToolchain resolves toolchain_checks entry tc for dir under root,
// filling {bin} with the first of its bins that resolves. run-checks and the
// <tooling> block both use it, so Claude is never told about a check the
// checks skip.
func ResolveToolchain(cfg *config.Config, tc config.ToolchainCheck, dir, root string) Toolchain {
	if tc.WhenDir != "" && !project.IsDir(filepath.Join(dir, tc.WhenDir)) {
		return Toolchain{Resolution: Resolution{Skip: "no " + tc.WhenDir + "/ yet"}}
	}
	template := strings.Fields(tc.Cmd)
	if len(tc.Bin) == 0 {
		return Toolchain{Words: template, Shown: template}
	}
	var first Resolution
	for i, bin := range tc.Bin {
		res := Resolve(cfg, dir, root, bin, Default)
		if res.Words != nil {
			return Toolchain{Words: Fill(template, "{bin}", res.Words), Shown: Fill(template, "{bin}", res.shown(root)), Resolution: res}
		}
		if i == 0 {
			first = res
		}
	}
	return Toolchain{Resolution: first}
}

// Fill replaces each template word equal to placeholder with words, so a
// path with spaces stays one argument and a multi-word resolution (yarn run
// prettier) stays several.
func Fill(template []string, placeholder string, words []string) []string {
	var out []string
	for _, w := range template {
		if w == placeholder {
			out = append(out, words...)
			continue
		}
		out = append(out, w)
	}
	return out
}

// shown is r's words as Claude should read them: a PATH binary by its bare
// name, a local one relative to root, a package-manager command as is.
func (r Resolution) shown(root string) []string {
	switch r.Source {
	case SourcePATH:
		return []string{filepath.Base(r.Words[0])}
	case SourceLocal:
		if rel, err := filepath.Rel(root, r.Words[0]); err == nil {
			return []string{rel}
		}
	}
	return r.Words
}

// Origin is r's words and where they come from, e.g. "/r/node_modules/.bin/x
// (local)", so a fallback is visible; "(cmd not resolved)" for a command
// with no {bin}.
func (r Resolution) Origin() string {
	if r.Words == nil {
		return "(cmd not resolved)"
	}
	why := r.Source
	if r.Note != "" {
		why += "; " + r.Note
	}
	return ShellJoin(r.Words) + " (" + why + ")"
}

// BinLine is the line printed under a check's PASS or FAIL line.
func (r Resolution) BinLine() string { return "  bin: " + r.Origin() + "\n" }

// ShellJoin joins words for display, single-quoting any holding a space.
func ShellJoin(words []string) string {
	quoted := make([]string, len(words))
	for i, w := range words {
		if strings.ContainsAny(w, " \t") {
			w = "'" + strings.ReplaceAll(w, "'", `'\''`) + "'"
		}
		quoted[i] = w
	}
	return strings.Join(quoted, " ")
}
