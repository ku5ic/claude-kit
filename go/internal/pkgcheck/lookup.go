// Package pkgcheck catches a package that may not exist: one an add verb
// names that its registry doesn't know, and an import an edit adds that no
// manifest declares.
package pkgcheck

import (
	"encoding/json"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/resolve/kittool"
)

// Lookups checks package names against their registries until deadline.
// Its methods are safe to call at once.
type Lookups struct {
	c        config.PackageLookups
	deadline time.Time
	dir      string // the command's directory, where registry config is read from
	root     string
	// CacheDir holds the packages each manager's registry found, "" for none.
	CacheDir string

	mu      sync.Mutex                      // guards the maps below, each filled on first use
	local   map[string]bool                 // names the repo's own package.json files declare
	printed map[string][]string             // each private_cmd's globs
	found   map[string]map[string]time.Time // by manager, each package its registry found, and when
}

// foundFor is how long a found package is remembered.
const foundFor = 24 * time.Hour

// NewLookups starts c's time budget, shared by every lookup it runs for a
// command in dir, inside the repository at root.
func NewLookups(c config.PackageLookups, dir, root string) *Lookups {
	return &Lookups{c: c, deadline: time.Now().Add(time.Duration(c.Timeout) * time.Second), dir: dir, root: root}
}

// private is true when e routes name to a registry the lookup can't see: a
// line in one of e's private files from dir up to root, or a glob
// e.PrivateCmd prints matching name's leading path elements.
func (l *Lookups) private(e config.PackageLookup, name string) bool {
	scope := `[^\s\S]` // unscoped: a {scope} alternative never matches
	if s, _, scoped := strings.Cut(name, "/"); scoped && strings.HasPrefix(s, "@") {
		scope = regexp.QuoteMeta(s[1:])
	}
	for dir := range fsx.Up(l.dir, l.root) {
		for file, pattern := range e.Private {
			data, err := os.ReadFile(filepath.Join(dir, file))
			if re := compile("(?m)" + strings.ReplaceAll(pattern, "{scope}", scope)); err == nil && re != nil && re.Match(data) {
				return true
			}
		}
	}
	return slices.ContainsFunc(l.globs(e.PrivateCmd), func(glob string) bool { return matchPrefix(glob, name) })
}

// globs is what cmd prints, split on commas and lines, read once per cmd.
// A failure prints nothing, so it finds nothing private.
func (l *Lookups) globs(cmd []string) []string {
	if len(cmd) == 0 {
		return nil
	}
	key := strings.Join(cmd, "\x00")
	l.mu.Lock()
	defer l.mu.Unlock()
	if globs, ok := l.printed[key]; ok {
		return globs
	}
	var globs []string
	bin, err := kittool.Path(cmd[0])
	if left := time.Until(l.deadline); left > 0 && err == nil {
		c := proc.Command(left, bin, cmd[1:]...)
		c.Dir = os.TempDir()
		out, _ := c.Output()
		globs = strings.FieldsFunc(string(out), func(r rune) bool { return r == ',' || r == '\n' })
	}
	if l.printed == nil {
		l.printed = map[string][]string{}
	}
	l.printed[key] = globs
	return globs
}

// matchPrefix is true when glob matches target's leading path elements, as
// many as glob has: Go's rule for GOPRIVATE.
func matchPrefix(glob, target string) bool {
	glob = strings.TrimSpace(glob)
	n := strings.Count(glob, "/") + 1
	parts := strings.SplitN(target, "/", n+1)
	if glob == "" || len(parts) < n {
		return false
	}
	ok, _ := path.Match(glob, strings.Join(parts[:n], "/"))
	return ok
}

// jsManagers read package.json, where a workspace package is named.
var jsManagers = []string{"npm", "pnpm", "yarn", "bun"}

// workspace is true when a package.json in the repository is named name:
// a workspace package, or a flag's value like --filter's.
func (l *Lookups) workspace(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
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

// FirstMissing is the first of operands, in their order, manager's
// registry says doesn't exist, "" for none; it looks them up at once.
func (l *Lookups) FirstMissing(manager string, operands []string) string {
	missing := make([]bool, len(operands))
	var wg sync.WaitGroup
	for i, p := range operands {
		wg.Go(func() {
			// A panic here would exit 2, which Claude Code reads as a
			// block; the hook's recover can't reach this goroutine.
			defer func() { _ = recover() }()
			missing[i] = l.Missing(manager, p)
		})
	}
	wg.Wait()
	if i := slices.Index(missing, true); i >= 0 {
		return operands[i]
	}
	return ""
}

// foundFile is where manager's found packages are kept, "" with no
// CacheDir.
func (l *Lookups) foundFile(manager string) string {
	if l.CacheDir == "" {
		return ""
	}
	return filepath.Join(l.CacheDir, cache.Packages, manager+".json")
}

// foundMap is manager's found packages, read on first use; the caller
// holds l.mu.
func (l *Lookups) foundMap(manager string) map[string]time.Time {
	if l.found == nil {
		l.found = map[string]map[string]time.Time{}
	}
	found, ok := l.found[manager]
	if !ok {
		found = map[string]time.Time{}
		if data, err := os.ReadFile(l.foundFile(manager)); err != nil || json.Unmarshal(data, &found) != nil {
			found = map[string]time.Time{} // unreadable: everything is asked again
		}
		l.found[manager] = found
	}
	return found
}

// cached is true when manager's registry found name within foundFor.
func (l *Lookups) cached(manager, name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	at, ok := l.foundMap(manager)[name]
	return ok && time.Since(at) < foundFor
}

// remember records that manager's registry found name, dropping entries
// past foundFor; a failed write only costs a lookup next time.
func (l *Lookups) remember(manager, name string) {
	file := l.foundFile(manager)
	if file == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	found := l.foundMap(manager)
	found[name] = now
	maps.DeleteFunc(found, func(_ string, at time.Time) bool { return now.Sub(at) >= foundFor })
	if data, err := json.Marshal(found); err == nil {
		_ = fsx.WriteAtomic(file, data, 0o600)
	}
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
		// A workspace package is named in a package.json, which only the
		// JS package managers read, whatever key an overlay files them under.
		if name == "" || missing == nil || len(e.Cmd) == 0 || !compileOK(e.Name, name) || slices.Contains(jsManagers, manager) && l.workspace(name) || l.private(e, name) || l.cached(manager, name) {
			return false
		}
		for p := name; ; p = parent(p) {
			found, absent := l.lookup(e.Cmd, p, missing)
			if found && p == name {
				l.remember(manager, name)
			}
			if !absent {
				return false
			}
			if !e.Parents || strings.Count(p, "/") < 2 {
				return true
			}
		}
	}
	return false
}

// lookup runs one lookup of name: found when it succeeds, absent only
// when it fails with output matching missing. A timeout and any other
// failure are neither.
func (l *Lookups) lookup(argv []string, name string, missing *regexp.Regexp) (found, absent bool) {
	left := time.Until(l.deadline)
	if left <= 0 {
		return false, false
	}
	args := make([]string, len(argv))
	for i, a := range argv {
		args[i] = strings.ReplaceAll(a, "{pkg}", name)
	}
	bin, err := kittool.Path(args[0])
	if err != nil {
		return false, false
	}
	cmd := proc.Command(left, bin, args[1:]...)
	cmd.Dir = os.TempDir() // outside the project: its config runs nothing before the add is approved
	out, err := cmd.CombinedOutput()
	return err == nil, err != nil && !cmd.TimedOut() && missing.Match(out)
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
