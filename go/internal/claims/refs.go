package claims

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cite"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/proc"
)

// A reference resolves, is broken, or is one this pass can't settle.
type resolution int

const (
	unsettled resolution = iota
	resolved
	broken
)

var (
	backticked = regexp.MustCompile("`([^`\n]+)`")
	citation   = regexp.MustCompile(`^[^\s:]+:\d+(-\d+)?$`)
	commitSHA  = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	digit      = regexp.MustCompile(`[0-9]`)
	listMarker = regexp.MustCompile(`^\d+[.)]$`)
	// filler is the words a statement made only of references may carry
	// around them: "See `a.go:3` and `b.go`." says nothing about either.
	filler = map[string]bool{"see": true, "also": true, "in": true, "at": true, "and": true, "cf": true,
		"the": true, "file": true, "line": true, "lines": true, "commit": true}
)

// symbolTimeout caps one rg run for a backticked symbol.
const symbolTimeout = 5 * time.Second

// settleReferences is the no-model pass over text from dir: each statement
// made of references and filler alone, every reference resolving, is
// dropped; one with a broken reference is flagged; the rest is what's left
// of text for the model. A reference that resolves proves only that the
// thing exists, so a statement saying anything about one stays for the model.
func settleReferences(dir, text string) (flagged []Unsupported, rest string) {
	root := dir
	if top := git.Toplevel(dir); top != "" {
		root = top
	}
	rest = text
	for _, s := range Statements(text) {
		refs, bare := references(s)
		if !bare {
			continue
		}
		var problems []string
		settled := true
		for _, r := range refs {
			switch res, why := settle(root, r); res {
			case broken:
				problems = append(problems, "`"+r+"`: "+why)
			case unsettled:
				settled = false
			}
		}
		if len(problems) > 0 {
			flagged = append(flagged, Unsupported{Claim: s, Check: strings.Join(problems, "; ")})
		} else if !settled {
			continue
		}
		rest = strings.Replace(rest, s, "", 1)
	}
	return flagged, rest
}

// references is each reference in statement s, and whether s is nothing
// but references and filler: every backticked span, and outside them each
// word shaped like a path, a file:line, or a commit SHA.
func references(s string) (refs []string, bare bool) {
	for _, m := range backticked.FindAllStringSubmatch(s, -1) {
		refs = append(refs, m[1])
	}
	for i, word := range strings.Fields(backticked.ReplaceAllString(s, " ")) {
		if i == 0 && listMarker.MatchString(word) {
			continue
		}
		word = strings.Trim(word, `.,;:!()[]"'|-*+#>`)
		switch {
		case word == "" || filler[strings.ToLower(word)]:
		case strings.Contains(word, "/") || citation.MatchString(word) || isSHA(word):
			refs = append(refs, word)
		default:
			return nil, false
		}
	}
	return refs, len(refs) > 0
}

// isSHA is whether word is shaped like an abbreviated or full commit SHA;
// a digit tells one from a hex-letter word such as "defaced".
func isSHA(word string) bool { return commitSHA.MatchString(word) && digit.MatchString(word) }

// settle checks one reference against root: a file:line with cite, a path
// on disk, a SHA with git, and any other backticked span as a symbol rg
// finds. A path whose first segment root lacks points at another tree.
func settle(root, ref string) (resolution, string) {
	switch {
	case citation.MatchString(ref):
		if problems := cite.Broken(root, " "+ref); len(problems) > 0 {
			return broken, problems[0].Reason
		}
		path, _, _ := strings.Cut(ref, ":")
		if exists(root, path) {
			return resolved, ""
		}
		return unsettled, ""
	case isSHA(ref):
		if git.Toplevel(root) == "" {
			return unsettled, ""
		}
		if git.Verify(root, ref+"^{commit}") {
			return resolved, ""
		}
		return broken, "no such commit"
	case strings.Contains(ref, "/"):
		if exists(root, ref) {
			return resolved, ""
		}
		first, _, _ := strings.Cut(strings.TrimPrefix(ref, "./"), "/")
		if !filepath.IsAbs(ref) && first != "" && exists(root, first) {
			return broken, "no such file"
		}
		return unsettled, ""
	case strings.ContainsAny(ref, " \t"):
		return unsettled, ""
	}
	return symbol(root, ref)
}

// exists is whether path, relative to root unless absolute, is on disk.
func exists(root, path string) bool {
	_, err := os.Stat(fsx.Abs(root, path))
	return err == nil
}

// symbol is whether rg finds ref, as a fixed string, in root's files;
// unsettled when rg is missing or fails.
func symbol(root, ref string) (resolution, string) {
	err := proc.Command(symbolTimeout, "rg", "-qF", "--", ref, root).Run()
	if err == nil {
		return resolved, ""
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok && exit.ExitCode() == 1 {
		return broken, "not found in the project"
	}
	return unsettled, ""
}
