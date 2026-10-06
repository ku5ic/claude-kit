// Package guard holds the path predicates the edit and bash guards share:
// credential files, shell rc files, lockfiles, the overlay, and bash glob
// matching.
package guard

import (
	"os"
	"regexp"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/project"
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
// sensitive_paths. A "~/" entry is a path under $HOME, a directory when it
// ends in "/"; any other entry is a basename glob. The path may still carry
// the quotes of a shell word: cat "$HOME/.ssh/id_rsa".
func IsSensitive(cfg *config.Config, path string) bool {
	if path != "" && (path[0] == '"' || path[0] == '\'') {
		path = path[1:]
	}
	if path != "" && (path[len(path)-1] == '"' || path[len(path)-1] == '\'') {
		path = path[:len(path)-1]
	}
	path = ExpandHome(os.Getenv("HOME"), path)
	home := os.Getenv("HOME")
	base := path[strings.LastIndex(path, "/")+1:]
	for _, entry := range cfg.SensitivePaths {
		if rest, ok := strings.CutPrefix(entry, "~/"); ok {
			homePath := home + "/" + rest
			if strings.HasSuffix(homePath, "/") {
				if strings.HasPrefix(path, homePath) {
					return true
				}
			} else if path == homePath {
				return true
			}
		} else if Glob(entry, base) {
			return true
		}
	}
	return false
}

// IsRCFile is true when path is a shell rc file per rc_files.
func IsRCFile(cfg *config.Config, path string) bool {
	path = ExpandHome(os.Getenv("HOME"), path)
	home := os.Getenv("HOME")
	for _, entry := range cfg.RCFiles {
		if path == home+"/"+strings.TrimPrefix(entry, "~/") {
			return true
		}
	}
	return false
}

// IsGuardedLockfile is true when path's basename is a tool-generated
// lockfile: package_managers lockfiles not marked hand_edited, plus
// extra_lockfiles.
func IsGuardedLockfile(cfg *config.Config, path string) bool {
	base := path[strings.LastIndex(path, "/")+1:]
	for _, pm := range cfg.PackageManagers {
		if !pm.HandEdited && pm.Lockfile == base {
			return true
		}
	}
	for _, lockfile := range cfg.ExtraLockfiles {
		if lockfile == base {
			return true
		}
	}
	return false
}

// IsOverlay is true when path is the overlay, reached through any path or
// symlink. Writes to it get a prompt: it can switch the kit's guards off.
func IsOverlay(paths config.Paths, path string) bool {
	return project.PhysicalPath(path) == project.PhysicalPath(paths.Overlay)
}

// Glob matches s against a bash pattern as [[ s == pattern ]] does: * and ?
// cross "/", [...] is a bracket expression ([!...] or [^...] negates), and
// a backslash escapes the next character.
func Glob(pattern, s string) bool {
	re, err := globRegexp(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

var globCache = map[string]*regexp.Regexp{}

func globRegexp(pattern string) (*regexp.Regexp, error) {
	if re, ok := globCache[pattern]; ok {
		return re, nil
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
			b.WriteString("[" + strings.ReplaceAll(class, `\`, `\\`) + "]")
			i += end + 1
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString(`)\z`)
	re, err := regexp.Compile(b.String())
	if err == nil {
		globCache[pattern] = re
	}
	return re, err
}
