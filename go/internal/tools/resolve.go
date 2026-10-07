package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	SourceLocal = "local"
	SourcePM    = "package-manager env"
	SourcePATH  = "PATH"
)

// Resolution is how a tool runs: the words that run it and where they come
// from, or, when Words is nil, why it can't run.
type Resolution struct {
	Words  []string
	Source string
	Skip   string
}

// Resolve finds name for dir: a project-local copy (node_modules/.bin,
// .venv/bin, venv/bin, from dir up to root), else the project's
// package-manager environment, else PATH unless localOnly (a type checker
// from PATH can't see the project's packages). Never npx, pnpm dlx, or uv
// run, which can install packages.
func Resolve(dir, root, name string, localOnly bool) Resolution {
	for _, sub := range []string{"node_modules/.bin", ".venv/bin", "venv/bin"} {
		if found := project.FindUp(dir, root, filepath.Join(sub, name)); found != "" && executable(found) {
			return Resolution{Words: []string{found}, Source: SourceLocal}
		}
	}
	for _, l := range lookups {
		lock := project.FindUp(dir, root, l.lockfile)
		if lock == "" {
			continue
		}
		if words := l.resolve(filepath.Dir(lock), name); words != nil {
			return Resolution{Words: words, Source: SourcePM}
		}
	}
	if localOnly {
		return Resolution{Skip: name + " not in the project environment"}
	}
	if path, err := exec.LookPath(name); err == nil {
		return Resolution{Words: []string{path}, Source: SourcePATH}
	}
	return Resolution{Skip: name + " not installed"}
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
