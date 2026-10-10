package tools

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/resolve"
)

// Where a Resolution's words come from.
const (
	SourceLocal   = "local"
	SourcePM      = "package-manager env"
	SourceManager = "version manager"
	SourcePATH    = "PATH"
)

// Resolution is how a tool runs: the words that run it and where they come
// from, or, when Words is nil, why it can't run. Project is true when the
// project itself names the tool (declared, pinned, a range the copy
// misses), so the skip is the project's to fix. Note qualifies a copy that
// runs (its declared range couldn't be checked).
type Resolution struct {
	Words   []string
	Source  string
	Skip    string
	Project bool
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
)

// Resolve finds name for dir, in tool_resolution's order (kit.yml): a
// project-local copy from dir up to root, a go.mod tool, the project's
// package-manager environment, then a skip when the project declares it but
// it isn't installed, then PATH as mode and the policy allow. Never npx,
// pnpm dlx, or uv run, which can install packages.
func Resolve(cfg *config.Config, dir, root, name string, mode Mode) Resolution {
	pkg := binPackage(cfg, name)
	owner, spec := resolve.JSOwner(dir, root, pkg)
	for _, sub := range []string{"node_modules/.bin", ".venv/bin", "venv/bin"} {
		found := fsx.FindUp(dir, root, filepath.Join(sub, name))
		if found == "" || !fsx.IsExecutable(found) {
			continue
		}
		res := Resolution{Words: []string{found}, Source: SourceLocal}
		if sub == "node_modules/.bin" && owner != "" {
			// The copy that runs must be the one the owning package
			// declares, whether it's the owner's own or hoisted above it.
			installed := filepath.Dir(filepath.Dir(filepath.Dir(found)))
			switch version, inRange := resolve.Satisfies(installed, pkg, spec); inRange {
			case resolve.OutOfRange:
				return Resolution{Skip: fsx.Rel(root, filepath.Join(owner, "package.json")) + " declares " + pkg + " " + spec +
					", installed is " + version + " at " + fsx.Rel(root, installed) + "; run " + installCmd(cfg, owner, "js"), Project: true}
			case resolve.Unchecked:
				res.Note = pkg + " " + spec + " not checked against the installed copy"
			}
		}
		return res
	}
	if path := resolve.GoTool(dir, root, name); path != "" {
		return Resolution{Words: []string{path}, Source: SourcePM}
	}
	for _, l := range cfg.ToolResolution.EnvLookups {
		lock := fsx.FindUp(dir, root, l.Marker)
		if lock == "" {
			continue
		}
		at := filepath.Dir(lock)
		if l.Marker == ".pnp.cjs" && owner != "" {
			// yarn bin answers for the workspace it runs in.
			at = owner
		}
		if words := envBin(l, at, name); words != nil {
			return Resolution{Words: words, Source: SourcePM}
		}
	}
	if bin := resolve.ActiveEnv(root, name); bin != "" {
		return Resolution{Words: []string{bin}, Source: SourcePM}
	}
	if manifest, install := declared(cfg, dir, root, name); manifest != "" {
		return Resolution{Skip: name + " declared in " + manifest + " but not installed; run " + install, Project: true}
	}
	if mode == LocalOnly {
		return Resolution{Skip: name + " not in the project environment"}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return Resolution{Skip: name + " not installed"}
	}
	if pin := pinned(cfg, dir, root, name); pin != "" {
		if underManagerDir(cfg, path) {
			return Resolution{Words: []string{path}, Source: SourceManager}
		}
		return Resolution{Skip: name + " pinned in " + pin + ", but PATH has " + path, Project: true}
	}
	if slices.Contains(cfg.ToolResolution.PathFallback, name) {
		return Resolution{Words: []string{path}, Source: SourcePATH}
	}
	return Resolution{Skip: name + " only on PATH (" + path + "); nothing in the project declares or pins it. Pin it (.tool-versions), or add it to tool_resolution.path_fallback in ~/.claude/claude-kit.local.yml to allow"}
}

// envBin runs an env_lookups entry (kit.yml) for name from dir: the words
// that run it, nil when the environment doesn't have it.
func envBin(l config.EnvLookup, dir, name string) []string {
	fill := func(words []string) []string {
		out := make([]string, len(words))
		for i, w := range words {
			out[i] = strings.ReplaceAll(w, "{bin}", name)
		}
		return out
	}
	if len(l.VenvCmd) > 0 {
		if _, err := exec.LookPath(l.VenvCmd[0]); err != nil {
			return nil
		}
		cmd := proc.Command(proc.Quick, l.VenvCmd[0], l.VenvCmd[1:]...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if venv := strings.TrimSpace(string(out)); err == nil && venv != "" {
			if bin := filepath.Join(venv, "bin", name); fsx.IsExecutable(bin) {
				return []string{bin}
			}
		}
		return nil
	}
	probe := fill(l.Probe)
	if len(probe) == 0 || len(l.Run) == 0 {
		return nil
	}
	if _, err := exec.LookPath(probe[0]); err != nil {
		return nil
	}
	cmd := proc.Command(proc.Quick, probe[0], probe[1:]...)
	cmd.Dir = dir
	if cmd.Run() != nil {
		return nil
	}
	return fill(l.Run)
}
