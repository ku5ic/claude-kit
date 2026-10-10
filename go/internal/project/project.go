// Package project answers questions about a checkout: where it starts,
// and where its subprojects are.
package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// maxDepth is how deep below the root a manifest still makes its directory
// a subproject.
const maxDepth = 4

// IsScratch is true for a path in, or at, a project's .claude/scratch or
// the home scratch, which $CLAUDE_CONFIG_DIR can move. Any other directory
// named scratch is project code and gets every check.
func IsScratch(paths config.Paths, path string) bool {
	p := filepath.Clean(path)
	home := paths.ScratchHome()
	return fsx.Within(home, p) ||
		strings.HasSuffix(p, "/.claude/scratch") || strings.Contains(p, "/.claude/scratch/")
}

// SubLabel is the " [sub]" a subproject's label ends with, "" for the
// root ("." or "").
func SubLabel(sub string) string {
	if sub == "." || sub == "" {
		return ""
	}
	return " [" + sub + "]"
}

// Subprojects lists ".", then every subproject directory relative to root,
// sorted: each directory holding a tracked language manifest at most
// maxDepth levels down, and each member a workspace manifest names
// (each sources.Languages row's Workspaces).
// Tracked files only, so node_modules and virtualenvs never count.
func Subprojects(root string) []string { return memo(&subprojects, root, subprojectsOf) }

// Asked once per root per process, as git.Toplevel is: a hook asks from
// several places.
var subprojects, workspaces sync.Map // root -> []string

func memo(m *sync.Map, root string, of func(string) []string) []string {
	v, ok := m.Load(root)
	if !ok {
		v, _ = m.LoadOrStore(root, of(root))
	}
	return slices.Clone(v.([]string))
}

func subprojectsOf(root string) []string {
	var found, pathspecs []string
	for _, name := range sources.Anchors() {
		pathspecs = append(pathspecs, ":(glob)**/"+name)
	}
	paths, _ := git.Lines(root, append([]string{"ls-files", "--"}, pathspecs...)...)
	for _, path := range paths {
		dir := filepath.Dir(path)
		if !strings.Contains(path, "/") || strings.Count(dir, "/")+1 > maxDepth {
			continue
		}
		found = append(found, dir)
	}

	found = append(found, Workspace(root)...)

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

// Workspace is every member directory, relative to root, a workspace
// manifest at root names (each sources.Languages row's Workspaces).
func Workspace(root string) []string { return memo(&workspaces, root, workspaceOf) }

func workspaceOf(root string) []string {
	var patterns, out []string
	for _, l := range sources.Languages {
		for _, m := range l.Workspaces {
			patterns = append(patterns, m.Read(filepath.Join(root, m.Manifest), m.Path)...)
		}
	}
	for _, pattern := range patterns {
		// Negated entries only narrow a pnpm glob; nothing to add.
		if strings.HasPrefix(pattern, "!") {
			continue
		}
		out = append(out, globDirs(root, strings.TrimPrefix(pattern, "./"))...)
	}
	return out
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
			if fsx.IsDir(abs) {
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
