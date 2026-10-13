package resolve

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// toolchainOf is the manifest, from dir up to root, whose toolchain
// provides name, a task runner's included (a Makefile, make); "" when none
// does.
func toolchainOf(dir, root, name string) string {
	for _, l := range sources.Languages {
		if slices.Contains(l.Bins, name) {
			if path := fsx.FindUp(dir, root, l.Manifests[0]); path != "" {
				return fsx.Rel(root, path)
			}
		}
	}
	for _, tp := range sources.TaskProviders {
		if runner, _, _ := strings.Cut(tp.Run, " "); runner == name {
			if path := fsx.FindUp(dir, root, tp.Manifests...); path != "" {
				return fsx.Rel(root, path)
			}
		}
	}
	return ""
}

// systemDirs hold the operating system's own commands.
var systemDirs = []string{"/bin", "/usr/bin", "/sbin", "/usr/sbin"}

// system is true for a binary the operating system ships: no project
// states it, and none needs to.
func system(path string) bool {
	return slices.Contains(systemDirs, filepath.Dir(path))
}

// pinAliases are plugin names that differ from the binary they install.
var pinAliases = map[string]string{"golang": "go", "nodejs": "node", "opentofu": "tofu", "bats-core": "bats"}

// pinned is the pin file, from dir up to root, that names one of tool's
// names; "" when none does.
func pinned(dir, root string, tool []string) string {
	for _, file := range sources.PinFiles {
		if path := fsx.FindUp(dir, root, file); path != "" && names(path, tool) {
			return fsx.Rel(root, path)
		}
	}
	return ""
}

// userPinned is true when a user_pins file names one of tool's names.
func (r *Resolver) userPinned(tool []string) bool {
	return slices.ContainsFunc(r.userPins, func(path string) bool { return names(path, tool) })
}

// names is true when the pin file at path pins one of tool's names, by
// name or alias.
func names(path string, tool []string) bool {
	return slices.ContainsFunc(sources.Pins(path), func(pin string) bool {
		return slices.Contains(tool, pin) || slices.Contains(tool, pinAliases[pin])
	})
}

// toolNames are what a pin may call the binary at path: name, and the cask
// whose Caskroom it links into (cask 'claude-code@latest' installs claude).
func toolNames(name, path string) []string {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return []string{name}
	}
	for _, prefix := range brewPrefixes() {
		if rel, err := filepath.Rel(filepath.Join(prefix, "Caskroom"), real); err == nil && filepath.IsLocal(rel) {
			cask, _, _ := strings.Cut(rel, string(filepath.Separator))
			return []string{name, cask}
		}
	}
	return []string{name}
}

func brewPrefixes() []string {
	return []string{os.Getenv("HOMEBREW_PREFIX"), "/opt/homebrew", "/usr/local", "/home/linuxbrew/.linuxbrew"}
}

// underManager is true when path, or the file it links to, sits where asdf,
// mise, or Homebrew install: a pinned tool runs only from its manager.
func underManager(path string) bool {
	home, _ := os.UserHomeDir()
	var dirs []string
	add := func(base string, subs ...string) {
		if base != "" {
			for _, sub := range subs {
				dirs = append(dirs, filepath.Join(base, sub))
			}
		}
	}
	for _, base := range []string{os.Getenv("ASDF_DATA_DIR"), os.Getenv("MISE_DATA_DIR"), filepath.Join(home, ".asdf"), filepath.Join(home, ".local/share/mise")} {
		add(base, "shims", "installs")
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		add(filepath.Join(xdg, "mise"), "shims", "installs")
	}
	for _, prefix := range brewPrefixes() {
		add(prefix, "Cellar", "Caskroom")
	}
	real, _ := filepath.EvalSymlinks(path)
	return slices.ContainsFunc(dirs, func(d string) bool {
		return fsx.Within(d, path) || real != "" && fsx.Within(d, real)
	})
}

// declared is the manifest, from dir up to root, that declares a package
// named name; "" when none does.
func declared(dir, root, name string) string {
	for d := range fsx.Up(dir, root) {
		for _, l := range sources.Languages {
			if l.Declares == nil {
				continue
			}
			if manifest := l.Declares(d, name); manifest != "" {
				return fsx.Rel(root, filepath.Join(d, manifest))
			}
		}
	}
	return ""
}

// ActiveEnv is name in an activated virtualenv or conda env, only when that
// env lives inside the project: an unrelated env says nothing about it.
func ActiveEnv(root, name string) string {
	physRoot := fsx.PhysicalPath(root)
	for _, key := range []string{"VIRTUAL_ENV", "CONDA_PREFIX"} {
		env := os.Getenv(key)
		if env == "" {
			continue
		}
		if !fsx.Within(physRoot, fsx.PhysicalPath(env)) {
			continue
		}
		if bin := filepath.Join(env, "bin", name); fsx.IsExecutable(bin) {
			return bin
		}
	}
	return ""
}

// GoTool is the binary `go tool -n` builds for a tool go.mod declares, ""
// when go.mod doesn't declare name or it can't be built. GOPROXY=off keeps
// it from downloading; a module not in the cache counts as not installed.
func GoTool(dir, root, name string) string {
	mod := fsx.FindUp(dir, root, "go.mod")
	if mod == "" || !sources.GoModDeclares(mod, name) {
		return ""
	}
	if _, err := exec.LookPath("go"); err != nil {
		return ""
	}
	modDir := filepath.Dir(mod)
	flags := os.Getenv("GOFLAGS")
	if !fsx.IsDir(filepath.Join(modDir, "vendor")) {
		flags = strings.TrimSpace(flags + " -mod=readonly")
	}
	cmd := proc.Command(proc.Quick, "go", "tool", "-n", name)
	cmd.Dir = modDir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS="+flags)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	if fields := strings.Fields(string(out)); len(fields) > 0 && fsx.IsExecutable(fields[0]) {
		return fields[0]
	}
	return ""
}

// nodePackage is the package a node_modules/.bin link points into, ""
// when bin isn't such a link: its target names the package that provides
// it (node_modules/@biomejs/biome/bin/biome).
func nodePackage(bin string) string {
	target, err := os.Readlink(bin)
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(bin), target)
	}
	i := strings.LastIndex(target, "/node_modules/")
	if i < 0 {
		return ""
	}
	parts := strings.Split(target[i+len("/node_modules/"):], "/")
	if len(parts) > 1 && strings.HasPrefix(parts[0], "@") {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// JSOwner is the nearest directory, from dir up to root, whose package.json
// declares pkg, with the spec it declares: in a workspace, that package's
// version is the one that counts. "" when none does.
func JSOwner(dir, root, pkg string) (owner, spec string) {
	for d := range fsx.Up(dir, root) {
		if s, ok := sources.JSSpecs(d)[pkg]; ok {
			return d, s
		}
	}
	return "", ""
}

// Range is how an installed version stands against a declared range.
type Range int

const (
	InRange Range = iota
	OutOfRange
	Unchecked // a spec semver can't read (workspace:, catalog:, git...), or no readable version
)

// Satisfies checks the version of pkg installed under dir/node_modules
// against spec, returning the installed version.
func Satisfies(dir, pkg, spec string) (string, Range) {
	constraint, err := semver.NewConstraint(spec)
	if err != nil {
		return "", Unchecked
	}
	// An unreadable manifest has no version, which semver rejects.
	manifest := sources.Decode[struct {
		Version string `json:"version"`
	}](filepath.Join(dir, "node_modules", pkg, "package.json"), json.Unmarshal)
	version, err := semver.NewVersion(manifest.Version)
	if err != nil {
		return manifest.Version, Unchecked
	}
	if constraint.Check(version) {
		return manifest.Version, InRange
	}
	return manifest.Version, OutOfRange
}
