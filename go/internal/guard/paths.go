// Package guard holds the path predicates the edit and bash guards share:
// credential files, shell rc files, lockfiles, the overlay, and bash glob
// matching.
package guard

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"unicode"

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
	return matchSensitive(cfg, home, ExpandHome(home, path), false)
}

// foldHome is whether a "~/" entry matches whatever its case, as on
// macOS, whose home filesystem folds case. A basename entry always does: a
// key copied in from such a system keeps its PROD.PEM anywhere.
var foldHome = runtime.GOOS == "darwin"

// SensitiveExclusion is an rg glob excluding every basename entry of
// sensitive_paths in any case ("!{.[eE][nN][vV],*.[pP][eE][mM]}"), "" when
// there is none. Bracket classes, not --iglob, since the Grep tool has
// none. An entry holding a brace, a comma, or a bracket would break the
// glob and is left out. A "!entry" exception is left out too: one glob
// can't carve it from the entry it narrows, so .env.example under .env.*
// stays excluded from a tree search, and is read when named.
func SensitiveExclusion(cfg *config.Config) string {
	var names []string
	for _, entry := range cfg.SensitivePaths {
		if !strings.HasPrefix(entry, "~/") && !strings.HasPrefix(entry, "!") && !strings.ContainsAny(entry, "{},/[]") {
			names = append(names, anyCase(entry))
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "!{" + strings.Join(names, ",") + "}"
}

// anyCase is glob with each letter a bracket class of both its cases.
func anyCase(glob string) string {
	var b strings.Builder
	for _, r := range glob {
		if lower, upper := unicode.ToLower(r), unicode.ToUpper(r); lower != upper {
			b.WriteString("[" + string(lower) + string(upper) + "]")
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// SearchesSensitive is true when a recursive search from root, its files
// filtered by glob, can read a credential or key file: root is one, or,
// with holds, has a "~/" entry under it, or glob names one. Root is
// absolute, or "" for none.
// shortcut: a search with no telling glob over a directory that merely
// holds a basename entry (a loose *.pem) passes. Inside the project, the
// Grep guard and a Bash rg, alone or piped only into bashguard's
// stdinFilters, exclude them with SensitiveExclusion; outside, and for
// grep -r or a chained rg, catching it means walking the tree.
func SearchesSensitive(cfg *config.Config, home, root, glob string, holds bool) bool {
	return root != "" && matchSensitive(cfg, home, root, holds) || namesSensitive(cfg, glob)
}

// matchSensitive is IsSensitive past the quotes and ~, and with holds, also
// true when a "~/" entry sits under path. A "!entry" is an exception: the
// last entry path matches decides, and an exception matches only path
// itself, never a directory holding it. Each form is judged on its own, so
// a .env.example linked to .env is sensitive through its target.
func matchSensitive(cfg *config.Config, home, path string, holds bool) bool {
	forms := []string{path}
	if filepath.IsAbs(path) {
		forms = []string{filepath.Clean(path), fsx.PhysicalPath(filepath.Clean(path))}
	}
	return slices.ContainsFunc(forms, func(form string) bool {
		sensitive := false
		for _, entry := range cfg.SensitivePaths {
			pattern, exception := strings.CutPrefix(entry, "!")
			if entryMatches(pattern, home, []string{form}, holds && !exception) {
				sensitive = !exception
			}
		}
		return sensitive
	})
}

// entryMatches is true when one of forms is the sensitive_paths entry, or
// with holds, a directory holding a "~/" entry.
func entryMatches(entry, home string, forms []string, holds bool) bool {
	rest, ok := strings.CutPrefix(entry, "~/")
	if !ok {
		return slices.ContainsFunc(forms, func(f string) bool { return Glob(strings.ToLower(entry), strings.ToLower(filepath.Base(f))) })
	}
	target := filepath.Join(home, rest)
	isDir := strings.HasSuffix(rest, "/")
	for _, t := range []string{target, physicalTarget(target)} {
		t := homeCase(t)
		if slices.ContainsFunc(forms, func(f string) bool {
			f = homeCase(f)
			return f == t || isDir && strings.HasPrefix(f, t+"/") || holds && (f == "/" || strings.HasPrefix(t, f+"/"))
		}) {
			return true
		}
	}
	return false
}

// homeCase is path lowercased where foldHome, as a "~/" entry compares it.
func homeCase(path string) string {
	if foldHome {
		return strings.ToLower(path)
	}
	return path
}

// namesSensitive is true when glob picks out a credential file by name,
// whatever its case: a
// basename entry matches its last element taken literally (*.pem), or that
// element, unless it's only wildcards, matches an entry (*.env -> .env).
// Each {a,b} alternative is checked on its own; a glob with more than
// MaxBraces of them counts as naming one, since expanding them all would
// outrun the hook's timeout, which fails open.
func namesSensitive(cfg *config.Config, glob string) bool {
	if glob == "" {
		return false
	}
	alts, ok := Braces(glob)
	if !ok {
		return true
	}
	return slices.ContainsFunc(alts, func(alt string) bool {
		base := alt[strings.LastIndex(alt, "/")+1:]
		generic := strings.Trim(base, "*?") == ""
		base = strings.ToLower(base)
		return slices.ContainsFunc(cfg.SensitivePaths, func(entry string) bool {
			entry = strings.ToLower(entry)
			return !strings.HasPrefix(entry, "~/") && !strings.HasPrefix(entry, "!") && (Glob(entry, base) || !generic && Glob(base, entry))
		})
	})
}

// MaxBraces caps how many {a,b} alternatives Braces expands: each group
// doubles them or more.
const MaxBraces = 1024

// Braces expands a glob's {a,b} alternatives, innermost first. Past
// MaxBraces it stops, with ok false and the alternatives so far.
func Braces(glob string) (alts []string, ok bool) {
	alts = expandBraces(glob, nil)
	if len(alts) > MaxBraces {
		return alts[:MaxBraces], false
	}
	return alts, true
}

func expandBraces(glob string, out []string) []string {
	end := strings.Index(glob, "}")
	start := strings.LastIndex(glob[:max(end, 0)], "{")
	if end < 0 || start < 0 {
		return append(out, glob)
	}
	for alt := range strings.SplitSeq(glob[start+1:end], ",") {
		if out = expandBraces(glob[:start]+alt+glob[end+1:], out); len(out) > MaxBraces {
			break
		}
	}
	return out
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

// KitSwitch names what path is when a write to it can turn the kit off or
// rewrite its record, so the write gets a prompt: the overlay, a Claude
// Code settings file, the kit's logs and state, and the installed plugin.
// "" for any other path. cwd inside the plugin is work on the kit itself.
//
// path is judged twice, with symlinks resolved and as written, each against
// the guarded paths in the same form, so a link in either direction is
// caught: one into a guarded place, and a guarded name linked out to a
// dotfiles tree.
func KitSwitch(paths config.Paths, cwd, path string) string {
	written := filepath.Clean(path)
	if what := kitSwitch(paths, cwd, fsx.PhysicalPath(written), fsx.PhysicalPath); what != "" {
		return what
	}
	return kitSwitch(paths, cwd, written, filepath.Clean)
}

// kitSwitch is KitSwitch for one form of p, with norm putting a guarded
// path in that form.
func kitSwitch(paths config.Paths, cwd, p string, norm func(string) string) string {
	dir, base := filepath.Dir(p), filepath.Base(p)
	in := func(d string) bool { return fsx.Within(norm(d), p) }
	switch {
	case paths.Overlay != "" && p == norm(paths.Overlay):
		return "the claude-kit overlay"
	case (base == "settings.json" || base == "settings.local.json") && (filepath.Base(dir) == ".claude" || dir == norm(paths.ClaudeDir)):
		return "a Claude Code settings file"
	case in(paths.LogDir()):
		return "the kit's logs"
	case in(paths.CacheDir()):
		return "the kit's state"
	case paths.Root != "" && in(paths.Root) && !fsx.Within(norm(paths.Root), norm(cwd)) && !in(filepath.Join(paths.Root, ".claude", "scratch")):
		return "the installed claude-kit plugin"
	}
	return ""
}

// KitSwitchAsk is the prompt for a write to what, as KitSwitch names it.
func KitSwitchAsk(what string) string {
	return "this writes " + what + ", which can turn the kit's checks off or rewrite its record; confirm the change"
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
