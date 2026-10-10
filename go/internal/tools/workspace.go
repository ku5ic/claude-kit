package tools

import (
	"github.com/ku5ic/claude-kit/go/internal/config"
)

// binPackage is the package that provides binary name.
func binPackage(cfg *config.Config, name string) string {
	if pkg := cfg.ToolResolution.BinPackages[name]; pkg != "" {
		return pkg
	}
	return name
}
