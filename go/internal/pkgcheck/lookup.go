// Package pkgcheck catches a package that may not exist: one an add verb
// names that its registry doesn't know, and an import an edit adds that no
// manifest declares.
package pkgcheck

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/proc"
)

// Lookups checks package names against their registries until deadline.
type Lookups struct {
	c        config.PackageLookups
	deadline time.Time
}

// NewLookups starts c's time budget, shared by every lookup it runs.
func NewLookups(c config.PackageLookups) *Lookups {
	return &Lookups{c, time.Now().Add(time.Duration(c.Timeout) * time.Second)}
}

// Missing is true only when manager's registry says operand doesn't exist.
// An operand that isn't a plain package name, a manager no ecosystem
// covers, a timeout, or any other failure is false: this never blocks on
// a lookup it can't read.
func (l *Lookups) Missing(manager, operand string) bool {
	if l.c.Timeout <= 0 {
		return false
	}
	for _, key := range slices.Sorted(maps.Keys(l.c.Ecosystems)) {
		e := l.c.Ecosystems[key]
		if !slices.Contains(e.Managers, manager) {
			continue
		}
		name, missing := bare(operand), compile(e.Missing)
		if name == "" || missing == nil || len(e.Cmd) == 0 || !compileOK(e.Name, name) {
			return false
		}
		for p := name; ; p = parent(p) {
			switch l.run(e.Cmd, p, missing) {
			case found, unknown:
				return false
			}
			if !e.Parents || strings.Count(p, "/") < 2 {
				return true
			}
		}
	}
	return false
}

type answer int

const (
	found answer = iota
	absent
	unknown
)

// run is one lookup of name: found on exit 0, absent when the output
// matches missing, unknown otherwise.
func (l *Lookups) run(argv []string, name string, missing *regexp.Regexp) answer {
	left := time.Until(l.deadline)
	if left <= 0 {
		return unknown
	}
	args := make([]string, len(argv))
	for i, a := range argv {
		args[i] = strings.ReplaceAll(a, "{pkg}", name)
	}
	cmd := proc.Command(left, args[0], args[1:]...)
	cmd.Dir = os.TempDir() // outside any project, so no workspace config steers it
	out, err := cmd.CombinedOutput()
	switch {
	case cmd.TimedOut():
		return unknown
	case err == nil:
		return found
	case missing.Match(out):
		return absent
	}
	return unknown
}

// bare is operand without its version, or "" when it's a path, a URL, an
// alias, or a protocol spec rather than a registry name.
func bare(operand string) string {
	if operand == "" || strings.ContainsAny(operand, ":\\") || strings.HasPrefix(operand, ".") || strings.HasPrefix(operand, "/") || strings.HasPrefix(operand, "~") {
		return ""
	}
	if i := strings.LastIndex(operand, "@"); i > 0 {
		operand = operand[:i]
	}
	return operand
}

func parent(p string) string { return p[:strings.LastIndex(p, "/")] }

func compile(re string) *regexp.Regexp {
	if re == "" {
		return nil
	}
	r, err := regexp.Compile(re) // config.validate names a bad one
	if err != nil {
		return nil
	}
	return r
}

func compileOK(re, s string) bool {
	r := compile(re)
	return r != nil && r.MatchString(s)
}
