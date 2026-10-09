// Package project answers questions about a checkout: where its
// subprojects are, which task providers they have, and which package
// manager owns a directory.
package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/extract"
	"github.com/ku5ic/claude-kit/go/internal/git"
)

// FindUp returns the first dir/name for each dir from start up to and
// including stop, never above it; "" when none exists. A start outside stop
// is checked on its own.
func FindUp(start, stop string, names ...string) string {
	dir := start
	for {
		for _, name := range names {
			candidate := filepath.Join(dir, name)
			if _, err := os.Lstat(candidate); err == nil {
				return candidate
			}
		}
		if dir == stop || dir == "/" || !strings.HasPrefix(dir, stop+"/") {
			return ""
		}
		dir = filepath.Dir(dir)
	}
}

// PhysicalPath follows a symlink at path itself, then resolves its
// directory. Works for paths that don't exist yet.
func PhysicalPath(path string) string {
	for range 40 {
		target, err := os.Readlink(path)
		if err != nil {
			break
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = target
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return path
	}
	return filepath.Join(dir, filepath.Base(path))
}

// Toplevel is the git worktree root holding dir, "" outside a repo. Git
// resolves symlinks, so it's the physical path.
func Toplevel(dir string) string {
	top, err := git.Line(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return top
}

// Branch is the branch checked out in dir's repo, "" on a detached HEAD; it
// errors outside a repo.
func Branch(dir string) (string, error) {
	return git.Line(dir, "branch", "--show-current")
}

// Lockfile is a package manager and the lockfile that names it.
type Lockfile struct {
	Manager string
	File    string
}

// NearestLockfile walks from dir up to its git toplevel (dir alone outside a
// repo) and returns the first lockfile of ecosystem, in kit.yml order per
// directory. Zero when there is none: greenfield. dir must be physical.
func NearestLockfile(cfg *config.Config, dir, ecosystem string) (Lockfile, bool) {
	top := Toplevel(dir)
	for {
		for _, pm := range cfg.PackageManagers {
			if pm.Ecosystem != ecosystem {
				continue
			}
			if IsFile(filepath.Join(dir, pm.Lockfile)) {
				return Lockfile{pm.Manager, pm.Lockfile}, true
			}
		}
		if top == "" || dir == top || dir == "/" {
			return Lockfile{}, false
		}
		dir = filepath.Dir(dir)
	}
}

// IsScratch is true for a path in, or at, a project's .claude/scratch or
// the home scratch, which $CLAUDE_CONFIG_DIR can move. Any other directory
// named scratch is project code and gets every check.
func IsScratch(paths config.Paths, path string) bool {
	p := filepath.Clean(path)
	home := paths.ScratchHome()
	return p == home || strings.HasPrefix(p, home+"/") ||
		strings.HasSuffix(p, "/.claude/scratch") || strings.Contains(p, "/.claude/scratch/")
}

// IsFile is true for an existing regular file.
func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// IsDir is true for an existing directory.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Provider is a task_providers entry whose manifest is in a directory.
type Provider struct {
	Name     string
	Stack    string // "" when the provider has no stack
	Manifest string
	index    int
}

// Providers lists the task providers with a manifest in dir, tasks or not.
func Providers(cfg *config.Config, dir string) []Provider {
	var out []Provider
	for i, tp := range cfg.TaskProviders {
		if slices.Contains(cfg.DisabledTaskProviders, tp.Name) {
			continue
		}
		for _, manifest := range tp.Manifests {
			if path := filepath.Join(dir, manifest); IsFile(path) {
				out = append(out, Provider{tp.Name, tp.Stack, path, i})
				break
			}
		}
	}
	return out
}

// Task is one runnable task: its provider, name, the command that runs it,
// and, where the manifest holds it, the shell it runs (its body).
type Task struct {
	Provider string
	Stack    string
	Name     string
	Cmd      string
	Body     string
	PerLine  bool // each line of Body runs in its own shell (make, just)
}

// Tasks lists every task of every provider in dir. {pm} is the nearest
// lockfile's manager for the provider's stack, else the stack's default, so a
// poetry service under a pnpm root runs poe through poetry.
func Tasks(cfg *config.Config, dir string) []Task {
	pmByStack := map[string]string{}
	physical := ""
	var out []Task
	for _, p := range Providers(cfg, dir) {
		tp := cfg.TaskProviders[p.index]
		pm, cached := pmByStack[p.Stack]
		if !cached {
			if physical == "" {
				physical, _ = filepath.EvalSymlinks(dir)
			}
			if p.Stack != "" && physical != "" {
				if lock, ok := NearestLockfile(cfg, physical, p.Stack); ok {
					pm = lock.Manager
				}
			}
			pmByStack[p.Stack] = pm
		}
		run := tp.Run
		if override, ok := tp.RunByPM[pm]; ok {
			run = override
		}
		if pm == "" {
			pm = cfg.DefaultManager(p.Stack)
		}
		run = strings.ReplaceAll(run, "{pm}", pm)
		names, err := extract.Run(tp.Extractor, p.Manifest, tp.Arg)
		if err != nil {
			_, _ = os.Stderr.WriteString("kit: " + err.Error() + "\n")
			continue
		}
		bodies := extract.Bodies(tp.Extractor, p.Manifest, tp.Arg)
		for _, name := range names {
			if name == "" {
				continue
			}
			body := bodies[name]
			if tp.Body != "" && body.Text != "" {
				body.Text = strings.ReplaceAll(tp.Body, "{body}", body.Text)
			}
			out = append(out, Task{Provider: tp.Name, Stack: tp.Stack, Name: name, Cmd: strings.ReplaceAll(run, "{task}", name), Body: body.Text, PerLine: body.PerLine})
		}
	}
	return out
}

const goWorkUse = `^[[:space:]]*(use[[:space:]]+)?\(?[[:space:]]*(\.[^[:space:]()]*)`

// Subprojects lists ".", then every subproject directory relative to root,
// sorted: each directory holding a tracked anchor sentinel at most
// subproject_max_depth levels down, and each member a workspace manifest
// names (package.json workspaces, pnpm-workspace.yaml, Cargo, uv, go.work).
// Tracked files only, so node_modules and virtualenvs never count.
func Subprojects(cfg *config.Config, root string) []string {
	var found []string

	var pathspecs []string
	for _, name := range cfg.AnchorSentinels() {
		pathspecs = append(pathspecs, ":(glob)**/"+name)
	}
	if len(pathspecs) > 0 {
		paths, _ := git.Lines(root, append([]string{"ls-files", "--"}, pathspecs...)...)
		for _, path := range paths {
			dir := filepath.Dir(path)
			if !strings.Contains(path, "/") || strings.Count(dir, "/")+1 > cfg.SubprojectMaxDepth {
				continue
			}
			found = append(found, dir)
		}
	}

	var patterns []string
	patterns = append(patterns, extract.JSONArray(filepath.Join(root, "package.json"), ".workspaces")...)
	patterns = append(patterns, extract.JSONArray(filepath.Join(root, "package.json"), ".workspaces.packages")...)
	patterns = append(patterns, extract.YAMLArray(filepath.Join(root, "pnpm-workspace.yaml"), ".packages")...)
	patterns = append(patterns, extract.TOMLArray(filepath.Join(root, "Cargo.toml"), ".workspace.members")...)
	patterns = append(patterns, extract.TOMLArray(filepath.Join(root, "pyproject.toml"), ".tool.uv.workspace.members")...)
	patterns = append(patterns, extract.RegexLines(filepath.Join(root, "go.work"), goWorkUse)...)
	for _, pattern := range patterns {
		// Negated entries only narrow a pnpm glob; nothing to add.
		if strings.HasPrefix(pattern, "!") {
			continue
		}
		found = append(found, globDirs(root, strings.TrimPrefix(pattern, "./"))...)
	}

	seen := map[string]bool{}
	var subs []string
	for _, dir := range found {
		dir = strings.TrimSuffix(strings.TrimPrefix(dir, "./"), "/")
		if dir == "" || dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		subs = append(subs, dir)
	}
	slices.Sort(subs)
	return append([]string{"."}, subs...)
}

// globDirs expands pattern relative to root as bash does with globstar and
// nullglob: ** spans zero or more directories, and a wildcard never matches
// a leading dot. Only directories are returned.
func globDirs(root, pattern string) []string {
	segments := strings.Split(strings.Trim(pattern, "/"), "/")
	var out []string
	var walk func(rel string, rest []string)
	walk = func(rel string, rest []string) {
		abs := filepath.Join(root, rel)
		if len(rest) == 0 {
			if IsDir(abs) {
				out = append(out, rel)
			}
			return
		}
		seg := rest[0]
		if seg == "**" {
			walk(rel, rest[1:])
			for _, child := range subdirs(abs) {
				walk(filepath.Join(rel, child), rest)
			}
			return
		}
		if !strings.ContainsAny(seg, "*?[") {
			walk(filepath.Join(rel, seg), rest[1:])
			return
		}
		entries, _ := os.ReadDir(abs)
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") && !strings.HasPrefix(seg, ".") {
				continue
			}
			if ok, _ := filepath.Match(seg, name); ok {
				walk(filepath.Join(rel, name), rest[1:])
			}
		}
	}
	walk("", segments)
	return out
}

func subdirs(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out
}
