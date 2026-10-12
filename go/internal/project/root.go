package project

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Root resolves the project root for cwd, as `kit project-root`:
//  1. the git worktree root;
//  2. cwd or up to 2 ancestors holding a language manifest;
//  3. cwd itself, with anchored false.
func Root(cwd string) (root string, anchored bool) {
	if top := git.Toplevel(cwd); top != "" {
		return top, true
	}
	anchors := sources.Anchors()
	dir := cwd
	for depth := 0; dir != "/" && depth < 3; depth++ {
		for _, name := range anchors {
			if fsx.IsFile(filepath.Join(dir, name)) {
				return dir, true
			}
		}
		dir = filepath.Dir(dir)
	}
	return cwd, false
}

var (
	leadingDots = regexp.MustCompile(`^\.+`)
	nonSlug     = regexp.MustCompile(`[^a-z0-9]+`)
)

// Name is a stable, slug-safe identifier for root: "home" for $HOME,
// "root" for /, else its lowercased basename with every non-alphanumeric
// run as "-", or "unknown" when nothing is left.
func Name(root string) string {
	if home, err := os.UserHomeDir(); err == nil && root == home {
		return "home"
	}
	if root == "/" {
		return "root"
	}
	slug := leadingDots.ReplaceAllString(filepath.Base(root), "")
	slug = nonSlug.ReplaceAllString(strings.ToLower(slug), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "unknown"
	}
	return slug
}

// ScratchDir resolves the scratch directory for cwd: <root>/.claude/scratch
// when the project is anchored, else the global one under the kit home.
// With create, it makes the directory, and registers a project scratch dir
// in scratch-registry.txt so `kit scratch-rotate`'s cwd-less run can prune it.
func ScratchDir(paths config.Paths, cwd string, create bool) (string, error) {
	dir := paths.ScratchHome()
	if root, anchored := Root(cwd); anchored {
		dir = filepath.Join(root, ".claude", "scratch")
		if create {
			if err := register(paths, dir); err != nil {
				return "", err
			}
		}
	}
	if create {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// PlanDir is where plan mode writes cwd's plans: plansDirectory from the
// first settings file setting it (the project's local, then shared, then the
// user's), resolved against the project root as Claude Code does, else
// <root>/.claude/plans when it exists, else Claude Code's default under the
// config home. Managed settings aren't read.
func PlanDir(paths config.Paths, cwd string) string {
	root, _ := Root(cwd)
	for _, file := range []string{
		filepath.Join(root, ".claude", "settings.local.json"),
		filepath.Join(root, ".claude", "settings.json"),
		filepath.Join(paths.Home, "settings.json"),
	} {
		if dir := sources.JSONValue(file, ".plansDirectory"); dir != "" {
			return filepath.Join(root, dir)
		}
	}
	if dir := filepath.Join(root, ".claude", "plans"); fsx.IsDir(dir) {
		return dir
	}
	return paths.PlansHome()
}

func register(paths config.Paths, dir string) error {
	registry := paths.ScratchRegistry()
	data, err := os.ReadFile(registry)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if slices.Contains(strings.Split(string(data), "\n"), dir) {
		return nil
	}
	return fsx.Append(registry, []byte(dir+"\n"))
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// ReportPath is <dir>/<kind>[-<slug>]-<YYYYMMDD-HHMM>.md, with kind and
// slug made filename-safe character by character.
func ReportPath(dir, kind, slug, stamp string) string {
	name := unsafeName.ReplaceAllString(kind, "-")
	if slug != "" {
		name += "-" + unsafeName.ReplaceAllString(slug, "-")
	}
	return filepath.Join(dir, name+"-"+stamp+".md")
}
