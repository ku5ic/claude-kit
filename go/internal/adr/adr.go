// Package adr reads a project's architecture decision records, in the
// adr-tools or MADR shape: where they live, which are accepted, and which
// files each cites.
package adr

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// dirs are where a project keeps its ADRs when no .adr-dir names the
// folder: adr-tools' default, then MADR's.
var dirs = []string{"doc/adr", "docs/adr", "docs/decisions"}

// codeSpan is a backticked span in an ADR.
var codeSpan = regexp.MustCompile("`([^`\n]+)`")

// maxDecision caps the Decision text kept; the path is there for the rest.
const maxDecision = 800

// Dir is root's ADR folder: the one .adr-dir names, else the first of
// adrDirs that exists; "" for none.
func Dir(root string) string {
	if data, err := os.ReadFile(filepath.Join(root, ".adr-dir")); err == nil {
		if dir := strings.TrimSpace(string(data)); dir != "" {
			return filepath.Join(root, dir)
		}
	}
	for _, dir := range dirs {
		if path := filepath.Join(root, dir); fsx.IsDir(path) {
			return path
		}
	}
	return ""
}

// ADR is one accepted ADR: its file, title, Decision, and the code spans
// it cites.
type ADR struct {
	Path, Title, Decision string
	spans                 []string
}

// Cites reports whether one of a's spans is rel, or a folder holding it
// written with a slash, so a bare word like `go` names no folder.
func (a ADR) Cites(rel string) bool {
	return slices.ContainsFunc(a.spans, func(span string) bool {
		span = strings.TrimPrefix(span, "./")
		if span == rel {
			return true
		}
		dir := strings.TrimSuffix(span, "/")
		return dir != "" && strings.Contains(span, "/") && strings.HasPrefix(rel, dir+"/")
	})
}

// Accepted is the accepted ADRs in dir.
func Accepted(dir string) []ADR {
	names, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	var out []ADR
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		if a, ok := parseADR(string(data)); ok {
			a.Path = name
			out = append(out, a)
		}
	}
	return out
}

// parseADR reads an adr-tools or MADR record: its status from front
// matter or a Status section, its first H1, and its Decision or Decision
// Outcome section. ok is false unless the status is accepted.
func parseADR(text string) (a ADR, ok bool) {
	var status, section string
	var decision strings.Builder
	front := strings.HasPrefix(text, "---\n")
	for i, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case front && i > 0 && trimmed == "---":
			front = false
		case front:
			if v, found := strings.CutPrefix(trimmed, "status:"); found {
				status = strings.Trim(strings.TrimSpace(v), `"'`)
			}
		case strings.HasPrefix(line, "# ") && a.Title == "":
			a.Title = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "## "):
			section = strings.ToLower(strings.TrimSpace(line[3:]))
		case section == "status" && status == "" && trimmed != "":
			status = trimmed
		case (section == "decision" || section == "decision outcome") && (trimmed != "" || decision.Len() > 0):
			decision.WriteString(line + "\n")
		}
	}
	if !strings.HasPrefix(strings.ToLower(status), "accepted") {
		return a, false
	}
	a.Decision = strings.TrimSpace(decision.String())
	if len(a.Decision) > maxDecision {
		cut := a.Decision[:maxDecision]
		if i := strings.LastIndexAny(cut, " \n"); i > 0 {
			cut = cut[:i]
		}
		a.Decision = cut + " ..."
	}
	for _, m := range codeSpan.FindAllStringSubmatch(text, -1) {
		a.spans = append(a.spans, path.Clean(m[1])+suffixSlash(m[1]))
	}
	return a, true
}

// suffixSlash keeps the trailing slash path.Clean drops.
func suffixSlash(s string) string {
	if strings.HasSuffix(s, "/") && s != "/" {
		return "/"
	}
	return ""
}
