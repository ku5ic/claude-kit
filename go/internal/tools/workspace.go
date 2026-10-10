package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// binPackage is the package that provides binary name.
func binPackage(cfg *config.Config, name string) string {
	if pkg := cfg.ToolResolution.BinPackages[name]; pkg != "" {
		return pkg
	}
	return name
}

// jsOwner is the nearest directory, from dir up to root, whose package.json
// declares pkg, with the spec it declares: in a workspace, that package's
// version is the one that counts. "" when none does.
func jsOwner(dir, root, pkg string) (owner, spec string) {
	for d := dir; ; d = filepath.Dir(d) {
		if s, ok := sources.JSSpecs(d)[pkg]; ok {
			return d, s
		}
		if d == root || d == "/" || !strings.HasPrefix(d, root) {
			return "", ""
		}
	}
}

type verdict int

const (
	matches verdict = iota
	mismatch
	unchecked // a spec semver can't read (workspace:, catalog:, git...), or no readable version
)

// satisfies checks the version of pkg installed under dir/node_modules
// against spec, returning the installed version.
func satisfies(dir, pkg, spec string) (string, verdict) {
	constraint, err := semver.NewConstraint(spec)
	if err != nil {
		return "", unchecked
	}
	data, err := os.ReadFile(filepath.Join(dir, "node_modules", pkg, "package.json"))
	if err != nil {
		return "", unchecked
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return "", unchecked
	}
	version, err := semver.NewVersion(manifest.Version)
	if err != nil {
		return manifest.Version, unchecked
	}
	if constraint.Check(version) {
		return manifest.Version, matches
	}
	return manifest.Version, mismatch
}
