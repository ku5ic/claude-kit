package project

import (
	"path/filepath"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

// ResolvePackageManager is the manager of the first package_managers
// lockfile (kit.yml order) found in dir or at its git toplevel, "" when
// none: monorepo lockfiles live at the root.
func ResolvePackageManager(cfg *config.Config, dir string) string {
	top := Toplevel(dir)
	for _, pm := range cfg.PackageManagers {
		if pm.Lockfile == "" || pm.Manager == "" {
			continue
		}
		if IsFile(filepath.Join(dir, pm.Lockfile)) || (top != "" && IsFile(filepath.Join(top, pm.Lockfile))) {
			return pm.Manager
		}
	}
	return ""
}
