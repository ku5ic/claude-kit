package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
)

// Root resolves the project root for cwd, as `kit project-root`:
//  1. the git worktree root;
//  2. cwd or up to 2 ancestors holding an anchor sentinel;
//  3. cwd itself, with anchored false.
func Root(cfg *config.Config, cwd string) (root string, anchored bool) {
	if top := git.Toplevel(cwd); top != "" {
		return top, true
	}
	anchors := cfg.AnchorSentinels()
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

// Dir resolves the scratch or plans directory for cwd: <root>/.claude/<kind>
// when the project is anchored, else the global one under the kit home.
// With create, it makes the directory, and registers a project scratch dir
// in scratch-registry.txt so `kit scratch-rotate`'s cwd-less run can prune it.
func Dir(cfg *config.Config, paths config.Paths, cwd, kind string, create bool) (string, error) {
	if kind != "scratch" && kind != "plans" {
		return "", fmt.Errorf("unknown kind: %s", kind)
	}
	var dir string
	if root, anchored := Root(cfg, cwd); anchored {
		dir = filepath.Join(root, ".claude", kind)
		if create && kind == "scratch" {
			if err := register(paths, dir); err != nil {
				return "", err
			}
		}
	} else if kind == "scratch" {
		dir = paths.ScratchHome()
	} else {
		dir = paths.PlansHome()
	}
	if create {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	return dir, nil
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
