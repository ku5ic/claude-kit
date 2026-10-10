package tools

import (
	"cmp"
	"os"
	"path/filepath"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/resolve"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// declared is the manifest, from dir up to root, that declares the package
// providing name, and the command that installs it; "" when none does.
func declared(cfg *config.Config, dir, root, name string) (manifest, install string) {
	pkg := binPackage(cfg, name)
	for d := dir; ; d = filepath.Dir(d) {
		switch {
		case sources.JSDeps(d)[pkg]:
			return fsx.Rel(root, filepath.Join(d, "package.json")), installCmd(cfg, d, "js")
		case sources.PythonDeps(d)[sources.PyName(pkg)]:
			manifest := filepath.Join(d, "pyproject.toml")
			if !fsx.IsFile(manifest) {
				manifest = filepath.Join(d, "requirements.txt")
			}
			return fsx.Rel(root, manifest), installCmd(cfg, d, "python")
		case sources.RubyDeps(d)[pkg]:
			return fsx.Rel(root, filepath.Join(d, "Gemfile.lock")), installCmd(cfg, d, "bundler")
		case fsx.IsFile(filepath.Join(d, "go.mod")) && resolve.GoModDeclares(filepath.Join(d, "go.mod"), name):
			return fsx.Rel(root, filepath.Join(d, "go.mod")), installCmd(cfg, d, "go")
		}
		if d == root || d == "/" || !strings.HasPrefix(d, root) {
			return "", ""
		}
	}
}

// installCmd is tool_resolution.install's command for the manager of the
// nearest lockfile of ecosystem, else its default manager. An ecosystem
// package_managers doesn't list (bundler, go) names its manager itself.
func installCmd(cfg *config.Config, dir, ecosystem string) string {
	manager := cmp.Or(cfg.DefaultManager(ecosystem), ecosystem)
	if lock, ok := project.NearestLockfile(cfg, fsx.PhysicalPath(dir), ecosystem); ok {
		manager = lock.Manager
	}
	if cmd := cfg.ToolResolution.Install[manager]; cmd != "" {
		return cmd
	}
	return manager + " install"
}

// pinned is the pin file, from dir up to root, that names name, "" when
// none does. A pin names a plugin: "npm:prettier" and "aqua:owner/tool"
// count by their last segment, and pin_aliases maps it to a binary.
func pinned(cfg *config.Config, dir, root, name string) string {
	for _, file := range cfg.ToolResolution.PinFiles {
		path := fsx.FindUp(dir, root, file)
		if path == "" {
			continue
		}
		for _, t := range sources.Pins(path) {
			if alias := cfg.ToolResolution.PinAliases[t]; alias != "" {
				t = alias
			}
			if t == name {
				return fsx.Rel(root, path)
			}
		}
	}
	return ""
}

// underManagerDir is true when path sits in one of manager_dirs, with "~"
// and a leading $VAR expanded; an entry whose variable is unset is skipped.
func underManagerDir(cfg *config.Config, path string) bool {
	home, _ := os.UserHomeDir()
	for _, dir := range cfg.ToolResolution.ManagerDirs {
		switch {
		case strings.HasPrefix(dir, "~/"):
			if home == "" {
				continue
			}
			dir = filepath.Join(home, dir[2:])
		case strings.HasPrefix(dir, "$"):
			key, rest, _ := strings.Cut(dir[1:], "/")
			value := os.Getenv(key)
			if value == "" {
				continue
			}
			dir = filepath.Join(value, rest)
		}
		if strings.HasPrefix(path, filepath.Clean(dir)+"/") {
			return true
		}
	}
	return false
}
