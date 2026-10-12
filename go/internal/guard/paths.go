// Package guard holds the path predicates the edit and bash guards share:
// credential files, shell rc files, lockfiles, the overlay, and bash glob
// matching.
package guard

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// ExpandHome turns a leading ~, $HOME, or ${HOME} into home.
func ExpandHome(home, path string) string {
	for _, prefix := range []string{"~", "$HOME", "${HOME}"} {
		if strings.HasPrefix(path, prefix) {
			return home + path[len(prefix):]
		}
	}
	return path
}

// IsSensitive is true when path is a credential or key file per
// sensitive_paths. A "~/" entry is a path under home, a directory when it
// ends in "/"; any other entry is a basename glob. The path may still carry
// the quotes of a shell word: cat "$HOME/.ssh/id_rsa". An absolute path
// also matches cleaned and with its symlinks resolved.
func IsSensitive(cfg *config.Config, home, path string) bool {
	if path != "" && (path[0] == '"' || path[0] == '\'') {
		path = path[1:]
	}
	if path != "" && (path[len(path)-1] == '"' || path[len(path)-1] == '\'') {
		path = path[:len(path)-1]
	}
	path = ExpandHome(home, path)
	forms := []string{path}
	if filepath.IsAbs(path) {
		forms = []string{filepath.Clean(path), fsx.PhysicalPath(filepath.Clean(path))}
	}
	for _, entry := range cfg.SensitivePaths {
		rest, ok := strings.CutPrefix(entry, "~/")
		if !ok {
			if slices.ContainsFunc(forms, func(f string) bool { return Glob(entry, filepath.Base(f)) }) {
				return true
			}
			continue
		}
		target := filepath.Join(home, rest)
		isDir := strings.HasSuffix(rest, "/")
		for _, t := range []string{target, physicalTarget(target)} {
			if slices.ContainsFunc(forms, func(f string) bool { return f == t || isDir && strings.HasPrefix(f, t+"/") }) {
				return true
			}
		}
	}
	return false
}

// SearchesSensitive is true when a recursive search from root, its files
// filtered by glob, can read a credential or key file: root is one or holds
// a "~/" entry, or glob names one. Root is absolute, or "" for none.
func SearchesSensitive(cfg *config.Config, home, root, glob string) bool {
	if root != "" {
		if IsSensitive(cfg, home, root) {
			return true
		}
		root = filepath.Clean(root)
		forms := []string{root, fsx.PhysicalPath(root)}
		for _, entry := range cfg.SensitivePaths {
			rest, ok := strings.CutPrefix(entry, "~/")
			if !ok {
				continue
			}
			target := filepath.Join(home, rest)
			for _, t := range []string{target, physicalTarget(target)} {
				if slices.ContainsFunc(forms, func(f string) bool { return f == "/" || strings.HasPrefix(t, f+"/") }) {
					return true
				}
			}
		}
	}
	return namesSensitive(cfg, glob)
}

// namesSensitive is true when glob picks out a credential file by name: a
// basename entry matches its last element taken literally (*.pem), or that
// element, not leading with a wildcard, matches an entry (.env* -> .env).
// shortcut: no brace expansion, and a leading * or ? reads as generic, so
// *.{pem,key} or *env slip through; matching rg's glob semantics fixes it.
func namesSensitive(cfg *config.Config, glob string) bool {
	if glob == "" {
		return false
	}
	base := glob[strings.LastIndex(glob, "/")+1:]
	generic := strings.HasPrefix(base, "*") || strings.HasPrefix(base, "?")
	return slices.ContainsFunc(cfg.SensitivePaths, func(entry string) bool {
		return !strings.HasPrefix(entry, "~/") && (Glob(entry, base) || !generic && Glob(base, entry))
	})
}

var physicalTargets sync.Map // sensitive_paths target -> its physical path

// physicalTarget is fsx.PhysicalPath of a sensitive_paths target, resolved
// once per process: every operand of every command is checked against all
// of them.
func physicalTarget(target string) string {
	if p, ok := physicalTargets.Load(target); ok {
		return p.(string)
	}
	p := fsx.PhysicalPath(target)
	physicalTargets.Store(target, p)
	return p
}

// IsRCFile is true when path is a shell rc file under home per rc_files.
func IsRCFile(cfg *config.Config, home, path string) bool {
	path = ExpandHome(home, path)
	for _, entry := range cfg.RCFiles {
		if path == home+"/"+strings.TrimPrefix(entry, "~/") {
			return true
		}
	}
	return false
}

// IsGuardedLockfile is true when path's basename matches lockfile_globs.
func IsGuardedLockfile(cfg *config.Config, path string) bool {
	return GlobAny(cfg.LockfileGlobs, path[strings.LastIndex(path, "/")+1:])
}

// IsOverlay is true when path is the overlay, reached through any path or
// symlink. Writes to it get a prompt: it can switch the kit's guards off.
func IsOverlay(paths config.Paths, path string) bool {
	return fsx.PhysicalPath(path) == fsx.PhysicalPath(paths.Overlay)
}

// GlobAny is true when s matches any of patterns.
func GlobAny(patterns []string, s string) bool {
	return slices.ContainsFunc(patterns, func(p string) bool { return Glob(p, s) })
}

// Glob matches s against a bash pattern as [[ s == pattern ]] does: * and ?
// cross "/", [...] is a bracket expression ([!...] or [^...] negates), and
// a backslash escapes the next character.
func Glob(pattern, s string) bool {
	// A literal pattern needs no regexp.
	if !strings.ContainsAny(pattern, `*?[\`) {
		return pattern == s
	}
	re, err := globRegexp(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

var globCache sync.Map // pattern -> *regexp.Regexp

func globRegexp(pattern string) (*regexp.Regexp, error) {
	if re, ok := globCache.Load(pattern); ok {
		return re.(*regexp.Regexp), nil
	}
	var b strings.Builder
	b.WriteString(`\A(?s:`)
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '\\':
			if i+1 < len(pattern) {
				i++
				b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
			} else {
				b.WriteString(`\\`)
			}
		case '[':
			end := strings.IndexByte(pattern[i+1:], ']')
			if end < 0 {
				b.WriteString(`\[`)
				continue
			}
			// A ] right after [ or [! is a literal member, not the close.
			if end == 0 || (end == 1 && (pattern[i+1] == '!' || pattern[i+1] == '^')) {
				next := strings.IndexByte(pattern[i+end+2:], ']')
				if next < 0 {
					b.WriteString(`\[`)
					continue
				}
				end += next + 1
			}
			class := pattern[i+1 : i+1+end]
			if class[0] == '!' {
				class = "^" + class[1:]
			}
			fmt.Fprintf(&b, "[%s]", strings.ReplaceAll(class, `\`, `\\`))
			i += end + 1
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString(`)\z`)
	re, err := regexp.Compile(b.String())
	if err == nil {
		globCache.Store(pattern, re)
	}
	return re, err
}
