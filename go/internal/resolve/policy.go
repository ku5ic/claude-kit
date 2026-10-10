package resolve

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	for manifest, bins := range sources.Toolchains {
		if slices.Contains(bins, name) {
			if path := fsx.FindUp(dir, root, manifest); path != "" {
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

// pinned is the pin file, from dir up to root, that names name; "" when
// none does.
func pinned(dir, root, name string) string {
	for _, file := range sources.PinFiles {
		path := fsx.FindUp(dir, root, file)
		if path == "" {
			continue
		}
		for _, pin := range sources.Pins(path) {
			if pin == name || pinAliases[pin] == name {
				return fsx.Rel(root, path)
			}
		}
	}
	return ""
}

// userPinned is true when a user_pins file names name.
func (r *Resolver) userPinned(name string) bool {
	return slices.ContainsFunc(r.userPins, func(path string) bool {
		return slices.ContainsFunc(sources.Pins(path), func(pin string) bool { return pin == name || pinAliases[pin] == name })
	})
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
	for _, prefix := range []string{os.Getenv("HOMEBREW_PREFIX"), "/opt/homebrew", "/usr/local", "/home/linuxbrew/.linuxbrew"} {
		add(prefix, "Cellar")
	}
	real, _ := filepath.EvalSymlinks(path)
	return slices.ContainsFunc(dirs, func(d string) bool {
		return strings.HasPrefix(path, d+"/") || real != "" && strings.HasPrefix(real, d+"/")
	})
}

// declared is the manifest, from dir up to root, that declares a package
// named name; "" when none does.
func declared(dir, root, name string) string {
	for d := dir; ; d = filepath.Dir(d) {
		var manifest string
		switch {
		case sources.JSDeps(d)[name]:
			manifest = "package.json"
		case sources.PythonDeps(d)[sources.PyName(name)]:
			manifest = "pyproject.toml"
		case sources.RubyDeps(d)[name]:
			manifest = "Gemfile.lock"
		case fsx.IsFile(filepath.Join(d, "go.mod")) && GoModDeclares(filepath.Join(d, "go.mod"), name):
			manifest = "go.mod"
		}
		if manifest != "" {
			return fsx.Rel(root, filepath.Join(d, manifest))
		}
		if d == root || d == "/" || !strings.HasPrefix(d, root) {
			return ""
		}
	}
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
		if rel, err := filepath.Rel(physRoot, fsx.PhysicalPath(env)); err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
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
	if mod == "" || !GoModDeclares(mod, name) {
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

var majorSuffix = regexp.MustCompile(`^v[0-9]+$`)

// GoModDeclares is true when go.mod has a tool directive whose package
// builds a binary called name: the last path element, or the one before a
// /vN major-version suffix.
func GoModDeclares(mod, name string) bool {
	found, inBlock := false, false
	sources.EachLine(mod, func(line string) {
		line = strings.TrimSpace(line)
		var pkg string
		switch {
		case inBlock && line == ")":
			inBlock = false
		case inBlock:
			pkg = line
		case line == "tool (":
			inBlock = true
		case strings.HasPrefix(line, "tool "):
			pkg = strings.TrimSpace(strings.TrimPrefix(line, "tool "))
		}
		if pkg == "" || strings.HasPrefix(pkg, "//") {
			return
		}
		parts := strings.Split(strings.Fields(pkg)[0], "/")
		bin := parts[len(parts)-1]
		if majorSuffix.MatchString(bin) && len(parts) > 1 {
			bin = parts[len(parts)-2]
		}
		found = found || bin == name
	})
	return found
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
	for d := dir; ; d = filepath.Dir(d) {
		if s, ok := sources.JSSpecs(d)[pkg]; ok {
			return d, s
		}
		if d == root || d == "/" || !strings.HasPrefix(d, root) {
			return "", ""
		}
	}
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
	var manifest struct {
		Version string `json:"version"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "node_modules", pkg, "package.json"))
	if err != nil || json.Unmarshal(data, &manifest) != nil {
		return "", Unchecked
	}
	version, err := semver.NewVersion(manifest.Version)
	if err != nil {
		return manifest.Version, Unchecked
	}
	if constraint.Check(version) {
		return manifest.Version, InRange
	}
	return manifest.Version, OutOfRange
}
