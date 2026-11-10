// Package pkgcheck catches a package that may not exist: one an add verb
// names that its registry doesn't know, and an import an edit adds that no
// manifest declares.
package pkgcheck

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/proc"
)

// Lookups checks package names against their registries until deadline.
type Lookups struct {
	c        config.PackageLookups
	deadline time.Time
	dir      string          // the command's directory, where registry config is read from
	local    map[string]bool // names the repo's own package.json files declare, read on first use
	root     string
}

// NewLookups starts c's time budget, shared by every lookup it runs for a
// command in dir, inside the repository at root.
func NewLookups(c config.PackageLookups, dir, root string) *Lookups {
	return &Lookups{c: c, deadline: time.Now().Add(time.Duration(c.Timeout) * time.Second), dir: dir, root: root}
}

// private is true when an rc file from dir up to root has a registry line
// for name's scope, or for every package.
func (l *Lookups) private(rc, name string) bool {
	if rc == "" {
		return false
	}
	scope, _, scoped := strings.Cut(name, "/")
	for dir := range fsx.Up(l.dir, l.root) {
		data, err := os.ReadFile(filepath.Join(dir, rc))
		if err != nil {
			continue
		}
		for line := range strings.Lines(string(data)) {
			key, _, _ := strings.Cut(strings.TrimSpace(line), "=")
			key = strings.TrimSpace(key)
			if key == "registry" || scoped && key == scope+":registry" {
				return true
			}
		}
	}
	return false
}

// workspace is true when a package.json in the repository is named name:
// a workspace package, or a flag's value like --filter's.
func (l *Lookups) workspace(name string) bool {
	if l.local == nil {
		l.local = map[string]bool{}
		files, _ := git.Files(l.root, ":(glob)**/package.json")
		for _, f := range files {
			data, err := os.ReadFile(filepath.Join(l.root, f))
			if err != nil {
				continue
			}
			var manifest struct{ Name string }
			if json.Unmarshal(data, &manifest) == nil && manifest.Name != "" {
				l.local[manifest.Name] = true
			}
		}
	}
	return l.local[name]
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
		if name == "" || missing == nil || len(e.Cmd) == 0 || !compileOK(e.Name, name) || l.workspace(name) || l.private(e.RCFile, name) {
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
	cmd.Dir = os.TempDir() // outside the project: its config runs nothing before the add is approved
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
