package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// lookup finds a binary inside a package manager's environment when the
// project doesn't keep one in node_modules/.bin or a .venv. Only commands
// that never create an environment or install anything: poetry run and uv
// run would; poetry env info, pipenv --venv, yarn bin, and bundle info don't.
type lookup struct {
	lockfile string
	venvCmd  []string // prints the environment dir; the bin is <dir>/bin/<name>
	probe    []string // exits 0 when the bin is available ({bin} replaced)
	run      []string // replaces the bin in the command ({bin} replaced)
}

var lookups = []lookup{
	{lockfile: "poetry.lock", venvCmd: []string{"poetry", "env", "info", "-p"}},
	{lockfile: "Pipfile.lock", venvCmd: []string{"pipenv", "--venv"}},
	{lockfile: ".pnp.cjs", probe: []string{"yarn", "bin", "{bin}"}, run: []string{"yarn", "run", "{bin}"}},
	// bundle info, not `bundle exec which`: which also finds a global gem
	// stub that bundle exec then refuses to load. Assumes gem name == bin.
	{lockfile: "Gemfile.lock", probe: []string{"bundle", "info", "{bin}"}, run: []string{"bundle", "exec", "{bin}"}},
}

// Where a Resolution's words come from.
const (
	SourceLocal   = "local"
	SourcePM      = "package-manager env"
	SourceManager = "version manager"
	SourcePATH    = "PATH"
)

// Resolution is how a tool runs: the words that run it and where they come
// from, or, when Words is nil, why it can't run. Missing is true only when
// no copy exists anywhere, the one skip a fallback formatter moves past.
// Note qualifies a copy that runs (its declared range couldn't be checked).
type Resolution struct {
	Words   []string
	Source  string
	Skip    string
	Missing bool
	Note    string
}

// Mode is how far past the project Resolve may look.
type Mode int

const (
	// Default allows PATH only for a pinned or path_fallback tool.
	Default Mode = iota
	// LocalOnly never takes PATH: a copy from there can't see the
	// project's packages.
	LocalOnly
	// AnyPath takes any PATH copy, for fallback formatters.
	AnyPath
)

// Resolve finds name for dir, in tool_resolution's order (kit.yml): a
// project-local copy from dir up to root, a go.mod tool, the project's
// package-manager environment, then a skip when the project declares it but
// it isn't installed, then PATH as mode and the policy allow. Never npx,
// pnpm dlx, or uv run, which can install packages.
func Resolve(cfg *config.Config, dir, root, name string, mode Mode) Resolution {
	pkg := binPackage(cfg, name)
	owner, spec := jsOwner(dir, root, pkg)
	for _, sub := range []string{"node_modules/.bin", ".venv/bin", "venv/bin"} {
		found := project.FindUp(dir, root, filepath.Join(sub, name))
		if found == "" || !executable(found) {
			continue
		}
		res := Resolution{Words: []string{found}, Source: SourceLocal}
		if sub == "node_modules/.bin" && owner != "" {
			// The copy that runs must be the one the owning package
			// declares, whether it's the owner's own or hoisted above it.
			installed := filepath.Dir(filepath.Dir(filepath.Dir(found)))
			switch version, verdict := satisfies(installed, pkg, spec); verdict {
			case mismatch:
				return Resolution{Skip: rel(root, filepath.Join(owner, "package.json")) + " declares " + pkg + " " + spec +
					", installed is " + version + " at " + rel(root, installed) + "; run " + installCmd(cfg, owner, "js", "npm")}
			case unchecked:
				res.Note = pkg + " " + spec + " not checked against the installed copy"
			}
		}
		return res
	}
	if path := goTool(dir, root, name); path != "" {
		return Resolution{Words: []string{path}, Source: SourcePM}
	}
	for _, l := range lookups {
		lock := project.FindUp(dir, root, l.lockfile)
		if lock == "" {
			continue
		}
		at := filepath.Dir(lock)
		if l.lockfile == ".pnp.cjs" && owner != "" {
			// yarn bin answers for the workspace it runs in.
			at = owner
		}
		if words := l.resolve(at, name); words != nil {
			return Resolution{Words: words, Source: SourcePM}
		}
	}
	if bin := activeEnv(root, name); bin != "" {
		return Resolution{Words: []string{bin}, Source: SourcePM}
	}
	if manifest, install := declared(cfg, dir, root, name); manifest != "" {
		return Resolution{Skip: name + " declared in " + manifest + " but not installed; run " + install}
	}
	if mode == LocalOnly {
		return Resolution{Skip: name + " not in the project environment"}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return Resolution{Skip: name + " not installed", Missing: true}
	}
	if mode == AnyPath {
		return Resolution{Words: []string{path}, Source: SourcePATH}
	}
	if pin := pinned(cfg, dir, root, name); pin != "" {
		if underManagerDir(cfg, path) {
			return Resolution{Words: []string{path}, Source: SourceManager}
		}
		return Resolution{Skip: name + " pinned in " + pin + ", but PATH has " + path}
	}
	if slices.Contains(cfg.ToolResolution.PathFallback, name) {
		return Resolution{Words: []string{path}, Source: SourcePATH}
	}
	return Resolution{Skip: name + " only on PATH (" + path + "); nothing in the project declares or pins it. Add it to tool_resolution.path_fallback to allow"}
}

func (l lookup) resolve(dir, name string) []string {
	fill := func(words []string) []string {
		out := make([]string, len(words))
		for i, w := range words {
			out[i] = strings.ReplaceAll(w, "{bin}", name)
		}
		return out
	}
	if l.venvCmd != nil {
		if _, err := exec.LookPath(l.venvCmd[0]); err != nil {
			return nil
		}
		cmd := exec.Command(l.venvCmd[0], l.venvCmd[1:]...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if venv := strings.TrimSpace(string(out)); err == nil && venv != "" {
			if bin := filepath.Join(venv, "bin", name); executable(bin) {
				return []string{bin}
			}
		}
		return nil
	}
	probe := fill(l.probe)
	if _, err := exec.LookPath(probe[0]); err != nil {
		return nil
	}
	cmd := exec.Command(probe[0], probe[1:]...)
	cmd.Dir = dir
	if cmd.Run() != nil {
		return nil
	}
	return fill(l.run)
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}
