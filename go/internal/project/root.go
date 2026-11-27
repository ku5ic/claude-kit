package project

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/rotate"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Root resolves the project root for cwd, as `kit project-root`:
//  1. the nearest ancestor holding a .git directory or file, symlinks
//     resolved as git resolves them, else the git worktree root;
//  2. cwd or up to 2 ancestors holding a language manifest;
//  3. cwd itself, with anchored false.
//
// The .git walk comes first: hooks call this on every Read and Edit, and an
// exec costs more than the walk (bashguard/packages.go does the same).
func Root(cwd string) (root string, anchored bool) {
	// git walks the physical path: a link partway up may lead into another
	// repo.
	if dotGit := fsx.FindUp(fsx.PhysicalPath(cwd), "/", ".git"); dotGit != "" {
		return filepath.Dir(dotGit), true
	}
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

// Name is a stable, slug-safe identifier for root: "home" for home,
// "root" for /, else its lowercased basename with every non-alphanumeric
// run as "-", or "unknown" when nothing is left.
func Name(root, home string) string {
	if home != "" && root == home {
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
// A registry it can't write, as in a sandbox that keeps Bash out of the
// config dir, leaves the dir unregistered and still returns it.
func ScratchDir(paths config.Paths, cwd string, create bool) (string, error) {
	dir := paths.ScratchHome()
	if root, anchored := Root(cwd); anchored {
		dir = filepath.Join(root, ".claude", "scratch")
		if create {
			_ = rotate.Register(context.Background(), paths, dir)
		}
	}
	if create {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// SettingsFiles are the Claude Code settings files that apply under root,
// highest first: the project's local, then shared, then the user's; only
// the user's for no root. Managed settings aren't read.
func SettingsFiles(paths config.Paths, root string) []string {
	files := []string{filepath.Join(paths.ClaudeDir, "settings.json")}
	if root != "" {
		files = append([]string{filepath.Join(root, ".claude", "settings.local.json"), filepath.Join(root, ".claude", "settings.json")}, files...)
	}
	return files
}

// PlanDir is where plan mode writes cwd's plans: plansDirectory from the
// first of SettingsFiles setting it, resolved against the project root as
// Claude Code does, else <root>/.claude/plans when it exists, else Claude
// Code's default under the config home.
func PlanDir(paths config.Paths, cwd string) string {
	root, _ := Root(cwd)
	for _, file := range SettingsFiles(paths, root) {
		if dir := sources.JSONValue(file, ".plansDirectory"); dir != "" {
			return filepath.Join(root, dir)
		}
	}
	if dir := filepath.Join(root, ".claude", "plans"); fsx.IsDir(dir) {
		return dir
	}
	return paths.PlansHome()
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// ReportPath is <dir>/<kind>[-<slug>].md, with kind and slug made
// filename-safe character by character. When that name is taken, it's the
// first free <kind>[-<slug>]-<n>.md from 2, so a rerun keeps the last report.
// A slug that ends in -<n> reads the same as a rerun's, so a reader that
// must tell them apart checks the report's own header.
func ReportPath(dir, kind, slug string) string {
	name := unsafeName.ReplaceAllString(kind, "-")
	if slug != "" {
		name += "-" + unsafeName.ReplaceAllString(slug, "-")
	}
	path := filepath.Join(dir, name+".md")
	for n := 2; taken(path); n++ {
		path = filepath.Join(dir, name+"-"+strconv.Itoa(n)+".md")
	}
	return path
}

// taken is whether anything, a dangling link included, sits at path.
func taken(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
