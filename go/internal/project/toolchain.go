package project

import (
	"os/exec"
	"path/filepath"
	"strings"

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
		if isFile(filepath.Join(dir, pm.Lockfile)) || (top != "" && isFile(filepath.Join(top, pm.Lockfile))) {
			return pm.Manager
		}
	}
	return ""
}

// ToolchainCmd resolves toolchain_checks entry tc for dir: the command with
// {bin} filled in, or the reason it can't run (its when_dir is missing, or
// none of its bins is on PATH). run-checks and the <tooling> block both use
// it, so Claude is never told about a check the checks skip.
func ToolchainCmd(tc config.ToolchainCheck, dir string) (cmd, skip string) {
	if tc.WhenDir != "" && !isDir(filepath.Join(dir, tc.WhenDir)) {
		return "", "no " + tc.WhenDir + "/ yet"
	}
	if len(tc.Bin) == 0 {
		return tc.Cmd, ""
	}
	for _, bin := range tc.Bin {
		if _, err := exec.LookPath(bin); err == nil {
			return strings.ReplaceAll(tc.Cmd, "{bin}", bin), ""
		}
	}
	return "", "none of " + strings.Join(tc.Bin, " ") + " on PATH"
}
