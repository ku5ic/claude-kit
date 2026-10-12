package sources

import (
	"path/filepath"
	"regexp"
	"strings"
)

// PinFiles are the files that pin a project's tools to a manager that
// installs them: asdf's and mise's versions, and Homebrew's Brewfile.
var PinFiles = []string{".tool-versions", "mise.toml", ".mise.toml", "Brewfile"}

var brewFormula = regexp.MustCompile(`^\s*brew\s+["']([^"']+)["']`)

// Pins are the tools a pin file names, each by its last segment: a pin
// names a plugin or a formula, and "npm:prettier", "aqua:owner/tool", and
// "owner/tap/tool" count as the tool at their end.
func Pins(file string) []string {
	var names []string
	switch filepath.Base(file) {
	case ".tool-versions":
		EachLine(file, func(line string) {
			if f := strings.Fields(line); len(f) > 0 && !strings.HasPrefix(f[0], "#") {
				names = append(names, f[0])
			}
		})
	case "Brewfile":
		EachLine(file, func(line string) {
			if m := brewFormula.FindStringSubmatch(line); m != nil {
				names = append(names, m[1])
			}
		})
	default:
		names = TOMLKeys(file, ".tools")
	}
	for i, n := range names {
		n = n[strings.LastIndex(n, ":")+1:]
		names[i] = n[strings.LastIndex(n, "/")+1:]
	}
	return names
}
