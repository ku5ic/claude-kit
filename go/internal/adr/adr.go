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
// dirs that exists; "" for none.
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
	p := adrParse{front: strings.HasPrefix(text, "---\n")}
	for i, line := range strings.Split(text, "\n") {
		p.line(i, line)
	}
	if !strings.HasPrefix(strings.ToLower(p.status), "accepted") {
		return a, false
	}
	a.Title = p.title
	a.Decision = clip(strings.TrimSpace(p.decision.String()))
	for _, m := range codeSpan.FindAllStringSubmatch(text, -1) {
		a.spans = append(a.spans, path.Clean(m[1])+suffixSlash(m[1]))
	}
	return a, true
}

// adrParse is what parseADR has read so far: the status, the first H1,
// the Decision section's lines, and the H2 section and front matter the
// next line is in.
type adrParse struct {
	status, section, title string
	decision               strings.Builder
	front                  bool
}

// line reads line i of the record.
func (p *adrParse) line(i int, line string) {
	trimmed := strings.TrimSpace(line)
	if p.front {
		p.frontLine(i, trimmed)
		return
	}
	switch {
	case strings.HasPrefix(line, "# ") && p.title == "":
		p.title = strings.TrimSpace(line[2:])
	case strings.HasPrefix(line, "## "):
		p.section = strings.ToLower(strings.TrimSpace(line[3:]))
	case p.section == "status" && p.status == "" && trimmed != "":
		p.status = trimmed
	case (p.section == "decision" || p.section == "decision outcome") && (trimmed != "" || p.decision.Len() > 0):
		p.decision.WriteString(line + "\n")
	}
}

// frontLine reads line i, trimmed, of the front matter: its status, or
// the --- that closes it.
func (p *adrParse) frontLine(i int, trimmed string) {
	if i > 0 && trimmed == "---" {
		p.front = false
		return
	}
	if v, found := strings.CutPrefix(trimmed, "status:"); found {
		p.status = strings.Trim(strings.TrimSpace(v), `"'`)
	}
}

// clip is decision cut at a word to maxDecision bytes, marked with " ...".
func clip(decision string) string {
	if len(decision) <= maxDecision {
		return decision
	}
	cut := decision[:maxDecision]
	if i := strings.LastIndexAny(cut, " \n"); i > 0 {
		cut = cut[:i]
	}
	return cut + " ..."
}

// suffixSlash keeps the trailing slash path.Clean drops.
func suffixSlash(s string) string {
	if strings.HasSuffix(s, "/") && s != "/" {
		return "/"
	}
	return ""
}
