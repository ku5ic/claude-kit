// Package resolve says which binary runs a command the kit runs for a
// project, and refuses one the project doesn't state: a package fetched to
// run it, or a copy found only on PATH that nothing pins and no manifest's
// toolchain or verified package manager provides. pre-commit's hooks never
// come here: pre-commit manages their environments.
package resolve

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
)

// Resolution is the binary a command runs and where it comes from, or,
// with Path "", why it can't run. Note qualifies a copy that runs.
type Resolution struct {
	Path, Source, Skip, Note string
}

// Where a Resolution comes from.
const (
	Local       = "local" // a gitignored bin directory of the project
	GoModTool   = "go.mod tool"
	Environment = "active environment"
	Pinned      = "pinned"
	Toolchain   = "toolchain"
	Manager     = "package manager"
	System      = "system"
)

// Resolver resolves binaries for one project root.
type Resolver struct {
	root     string
	bins     []binDir
	managers []string
	userPins []string
}

// binDir is a gitignored directory of binaries and the directory whose
// commands it serves: node_modules/.bin serves node_modules' parent.
type binDir struct{ path, serves string }

// New is a Resolver for root. managers are the binaries the project's
// verified package managers run as (pnpm, poetry): a lockfile states them,
// so their PATH copy runs. userPins are pin files that pin for every
// project, after its own; a "~/" one is under $HOME.
func New(root string, managers, userPins []string) *Resolver {
	home, _ := os.UserHomeDir()
	pins := make([]string, len(userPins))
	for i, p := range userPins {
		if rest, ok := strings.CutPrefix(p, "~/"); ok {
			p = filepath.Join(home, rest)
		}
		pins[i] = p
	}
	return &Resolver{root: root, bins: ignoredBins(root), managers: managers, userPins: pins}
}

// ignoredBins are the directories gitignore hides that hold binaries: an
// ignored bin or .bin, or one directly inside an ignored directory
// (node_modules/.bin, .venv/bin, vendor/bin).
func ignoredBins(root string) []binDir {
	dirs, _ := git.Paths(root, "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--directory")
	var bins []binDir
	for _, d := range dirs {
		d = filepath.Join(root, strings.TrimSuffix(d, "/"))
		if d == root || !fsx.IsDir(d) {
			continue
		}
		if name := filepath.Base(d); name == "bin" || name == ".bin" {
			bins = append(bins, binDir{d, filepath.Dir(d)})
			continue
		}
		for _, sub := range []string{"bin", ".bin"} {
			if fsx.IsDir(filepath.Join(d, sub)) {
				bins = append(bins, binDir{filepath.Join(d, sub), filepath.Dir(d)})
			}
		}
	}
	// The nearest serving directory wins, so deepest first.
	slices.SortStableFunc(bins, func(a, b binDir) int { return len(b.serves) - len(a.serves) })
	return bins
}

// Command resolves a command's words run in dir: refused when it would
// fetch a package to run it, else its first word's binary.
func (r *Resolver) Command(dir string, words []string) Resolution {
	if len(words) == 0 {
		return Resolution{Skip: "no command"}
	}
	if why := r.Fetches(dir, words); why != "" {
		return Resolution{Skip: "fetches a package (" + why + ")"}
	}
	return r.Bin(dir, words[0])
}

// Bin resolves name for a command run in dir: a path inside the project, a
// copy in a gitignored bin directory from dir up to the root, a go.mod
// tool, an active environment inside the project; a package a manifest
// declares but nothing installed is refused; then PATH, only when a pin
// file names it and it runs from that manager, a manifest's toolchain
// provides it, it is a verified package manager, or the system ships it.
func (r *Resolver) Bin(dir, name string) Resolution {
	if strings.Contains(name, "/") {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		if strings.HasPrefix(path, r.root+"/") && fsx.IsExecutable(path) {
			return Resolution{Path: path, Source: Local}
		}
		return Resolution{Skip: name + " not found in the project"}
	}
	if res, ok := r.local(dir, name); ok {
		return res
	}
	if path := GoTool(dir, r.root, name); path != "" {
		return Resolution{Path: path, Source: GoModTool}
	}
	if path := ActiveEnv(r.root, name); path != "" {
		return Resolution{Path: path, Source: Environment}
	}
	if manifest := declared(dir, r.root, name); manifest != "" {
		return Resolution{Skip: name + " declared in " + manifest + " but not installed"}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return Resolution{Skip: name + " not installed"}
	}
	if pin := pinned(dir, r.root, name); pin != "" {
		if underManager(path) {
			return Resolution{Path: path, Source: Pinned}
		}
		return Resolution{Skip: name + " pinned in " + pin + ", but PATH has " + path}
	}
	// A user pin only allows: a formula needn't be the binary PATH finds
	// (brew 'grep' installs ggrep).
	if r.userPinned(name) && underManager(path) {
		return Resolution{Path: path, Source: Pinned}
	}
	if manifest := toolchainOf(dir, r.root, name); manifest != "" {
		return Resolution{Path: path, Source: Toolchain, Note: manifest}
	}
	if slices.Contains(r.managers, name) {
		return Resolution{Path: path, Source: Manager}
	}
	if system(path) {
		return Resolution{Path: path, Source: System}
	}
	return Resolution{Skip: name + " only on PATH (" + path + "); nothing in the project pins or provides it. Pin it (.tool-versions, mise.toml, Brewfile, or a user_pins file)"}
}

// local is name in the nearest gitignored bin directory serving dir, with
// its declared version range checked when it is a node package.
func (r *Resolver) local(dir, name string) (Resolution, bool) {
	for _, b := range r.bins {
		if dir != b.serves && !strings.HasPrefix(dir, b.serves+"/") {
			continue
		}
		found := filepath.Join(b.path, name)
		if !fsx.IsExecutable(found) {
			continue
		}
		res := Resolution{Path: found, Source: Local}
		if pkg := nodePackage(found); pkg != "" {
			if owner, spec := JSOwner(dir, r.root, pkg); owner != "" {
				installed := filepath.Dir(filepath.Dir(b.path))
				switch version, v := Satisfies(installed, pkg, spec); v {
				case OutOfRange:
					return Resolution{Skip: fsx.Rel(r.root, filepath.Join(owner, "package.json")) + " declares " + pkg + " " + spec +
						", installed is " + version + " at " + fsx.Rel(r.root, installed) + "; reinstall the project's packages"}, true
				case Unchecked:
					res.Note = pkg + " " + spec + " not checked against the installed copy"
				}
			}
		}
		return res, true
	}
	return Resolution{}, false
}

// Fetches is the runner in words that would download a package to run it,
// "" when none would: a dlx runner, or npx or npm exec with no local copy.
func (r *Resolver) Fetches(dir string, words []string) string {
	if len(words) == 0 {
		return ""
	}
	two := words[0]
	if len(words) > 1 {
		two += " " + words[1]
	}
	switch {
	case words[0] == "bunx" || words[0] == "uvx":
		return words[0]
	case len(words) > 1 && (words[1] == "dlx" || two == "pipx run" || two == "bun x"):
		return two
	case two == "go run" && len(words) > 2 && strings.Contains(words[2], "@"):
		return "go run " + words[2]
	case words[0] == "npx" || two == "npm exec":
		runner, rest := words[0], words[1:]
		if runner == "npm" {
			runner, rest = two, words[2:]
		}
		bin := ""
		for _, w := range rest {
			if !strings.HasPrefix(w, "-") {
				bin = w
				break
			}
		}
		if _, ok := r.local(dir, bin); bin == "" || !ok {
			return runner + " " + bin + " with no local copy"
		}
	}
	return ""
}
