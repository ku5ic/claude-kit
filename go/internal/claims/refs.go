package claims

import (
	"cmp"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cite"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/resolve/kittool"
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

// settleBudget caps the no-model pass's lookups for one text. It runs in
// stop-dispatch ahead of stop-checks' own budget (go/internal/hooks/stop.go).
const settleBudget = 15 * time.Second

// References is each statement of text whose bare reference doesn't
// resolve from dir, found with no model run.
func References(dir, text string) []Unsupported {
	flagged, _ := settleReferences(dir, text, time.Now().Add(settleBudget))
	return flagged
}

// settleReferences is the no-model pass over text from dir: each statement
// made of references and filler alone, every reference resolving, is
// dropped; one with a broken reference is flagged; the rest is what's left
// of text for the model. A reference that resolves proves only that the
// thing exists, so a statement saying anything about one stays for the model.
// Past deadline, the statements left stay in rest, unchecked.
func settleReferences(dir, text string, deadline time.Time) (flagged []Unsupported, rest string) {
	root := cmp.Or(git.Toplevel(dir), dir)
	type statement struct {
		text string
		refs []string
	}
	var bare []statement
	var refs []string
	for _, s := range Statements(text) {
		if r, ok := references(s); ok {
			bare = append(bare, statement{s, r})
			refs = append(refs, r...)
		}
	}
	settled := settleAll(root, refs, deadline)
	rest = text
	for _, s := range bare {
		var problems []string
		allSettled := true
		for _, r := range s.refs {
			v, checked := settled[r]
			if !checked {
				// Past the deadline: this statement and the rest stay.
				return flagged, rest
			}
			switch v.res {
			case broken:
				problems = append(problems, "`"+r+"`: "+v.why)
			case unsettled:
				allSettled = false
			}
		}
		if len(problems) > 0 {
			flagged = append(flagged, Unsupported{Claim: s.text, Check: strings.Join(problems, "; ")})
		} else if !allSettled || len(s.refs) > 1 && blockStart.MatchString(s.text) {
			// A row or item pairing references says they belong together.
			continue
		}
		rest = cut(rest, s.text)
	}
	return flagged, rest
}

// verdict is one reference's settle result.
type verdict struct {
	res resolution
	why string
}

// settleAll settles each distinct ref against root, one per CPU at a time;
// a ref whose turn comes past deadline has no entry.
func settleAll(root string, refs []string, deadline time.Time) map[string]verdict {
	settled := map[string]verdict{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, runtime.NumCPU())
	seen := map[string]bool{}
	for _, r := range refs {
		if seen[r] {
			continue
		}
		seen[r] = true
		wg.Go(func() {
			// A panic here would exit 2, which Claude Code reads as a
			// block; the hook's recover can't reach this goroutine. The ref
			// stays unchecked.
			defer func() { _ = recover() }()
			slots <- struct{}{}
			defer func() { <-slots }()
			if time.Now().After(deadline) {
				return
			}
			res, why := settle(root, r)
			mu.Lock()
			settled[r] = verdict{res, why}
			mu.Unlock()
		})
	}
	wg.Wait()
	return settled
}

// cut is text without statement s, the first time it occurs. Statements
// come from prose, which joins a wrapped line with one space, so any run
// of whitespace in text matches each space of s.
func cut(text, s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = regexp.QuoteMeta(w)
	}
	re, err := regexp.Compile(strings.Join(words, `\s+`))
	if err != nil {
		return text
	}
	if loc := re.FindStringIndex(text); loc != nil {
		return text[:loc[0]] + text[loc[1]:]
	}
	return text
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
	rg, err := kittool.Path("rg")
	if err != nil {
		return unsettled, ""
	}
	// Hidden files hold real symbols (.github, dotfile settings), and so do
	// gitignored ones (generated code, node_modules); .git doesn't.
	err = proc.Command(symbolTimeout, rg, "-qF", "--hidden", "--no-ignore-vcs", "-g", "!.git", "--", ref, root).Run()
	if err == nil {
		return resolved, ""
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok && exit.ExitCode() == 1 {
		return broken, "not found in the project"
	}
	return unsettled, ""
}
