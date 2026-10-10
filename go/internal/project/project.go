// Package project answers questions about a checkout: where it starts,
// and where its subprojects are.
package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	return p == home || strings.HasPrefix(p, home+"/") ||
		strings.HasSuffix(p, "/.claude/scratch") || strings.Contains(p, "/.claude/scratch/")
}

// SubLabel is the " [sub]" a subproject's label ends with, "" for the
// root (".").
func SubLabel(sub string) string {
	if sub == "." {
		return ""
	}
	return " [" + sub + "]"
}

const goWorkUse = `^[[:space:]]*(use[[:space:]]+)?\(?[[:space:]]*(\.[^[:space:]()]*)`

// Subprojects lists ".", then every subproject directory relative to root,
// sorted: each directory holding a tracked language manifest at most
// maxDepth levels down, and each member a workspace manifest names
// (package.json workspaces, pnpm-workspace.yaml, Cargo, uv, go.work).
// Tracked files only, so node_modules and virtualenvs never count.
func Subprojects(root string) []string {
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
// manifest at root names: package.json workspaces, pnpm-workspace.yaml,
// Cargo, uv, go.work.
func Workspace(root string) []string {
	var patterns, out []string
	patterns = append(patterns, sources.JSONArray(filepath.Join(root, "package.json"), ".workspaces")...)
	patterns = append(patterns, sources.JSONArray(filepath.Join(root, "package.json"), ".workspaces.packages")...)
	patterns = append(patterns, sources.YAMLArray(filepath.Join(root, "pnpm-workspace.yaml"), ".packages")...)
	patterns = append(patterns, sources.TOMLArray(filepath.Join(root, "Cargo.toml"), ".workspace.members")...)
	patterns = append(patterns, sources.TOMLArray(filepath.Join(root, "pyproject.toml"), ".tool.uv.workspace.members")...)
	patterns = append(patterns, sources.RegexLines(filepath.Join(root, "go.work"), goWorkUse)...)
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
