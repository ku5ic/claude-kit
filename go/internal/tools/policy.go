package tools

import (
	"cmp"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/extract"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// goTool is the binary `go tool -n` builds for a tool go.mod declares, ""
// when go.mod doesn't declare name or it can't be built. GOPROXY=off keeps
// it from downloading; a module not in the cache counts as not installed.
func goTool(dir, root, name string) string {
	mod := project.FindUp(dir, root, "go.mod")
	if mod == "" || !goModDeclares(mod, name) {
		return ""
	}
	if _, err := exec.LookPath("go"); err != nil {
		return ""
	}
	modDir := filepath.Dir(mod)
	flags := os.Getenv("GOFLAGS")
	if !project.IsDir(filepath.Join(modDir, "vendor")) {
		flags = strings.TrimSpace(flags + " -mod=readonly")
	}
	cmd := exec.Command("go", "tool", "-n", name)
	cmd.Dir = modDir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS="+flags)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	if fields := strings.Fields(string(out)); len(fields) > 0 && executable(fields[0]) {
		return fields[0]
	}
	return ""
}

var majorSuffix = regexp.MustCompile(`^v[0-9]+$`)

// goModDeclares is true when go.mod has a tool directive whose package
// builds a binary called name: the last path element, or the one before a
// /vN major-version suffix.
func goModDeclares(mod, name string) bool {
	found, inBlock := false, false
	extract.EachLine(mod, func(line string) {
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

// activeEnv is name in an activated virtualenv or conda env, only when that
// env lives inside the repo: an unrelated env says nothing about the project.
func activeEnv(root, name string) string {
	physRoot := project.PhysicalPath(root)
	for _, key := range []string{"VIRTUAL_ENV", "CONDA_PREFIX"} {
		env := os.Getenv(key)
		if env == "" {
			continue
		}
		if rel, err := filepath.Rel(physRoot, project.PhysicalPath(env)); err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			continue
		}
		if bin := filepath.Join(env, "bin", name); executable(bin) {
			return bin
		}
	}
	return ""
}

// declared is the manifest, from dir up to root, that declares the package
// providing name, and the command that installs it; "" when none does.
func declared(cfg *config.Config, dir, root, name string) (manifest, install string) {
	pkg := binPackage(cfg, name)
	for d := dir; ; d = filepath.Dir(d) {
		switch {
		case JSDeps(d)[pkg]:
			return Rel(root, filepath.Join(d, "package.json")), installCmd(cfg, d, "js")
		case PythonDeps(d)[normalize(pkg)]:
			return manifestName(Python, d, root), installCmd(cfg, d, "python")
		case RubyDeps(d)[pkg]:
			return Rel(root, filepath.Join(d, "Gemfile.lock")), installCmd(cfg, d, "bundler")
		case project.IsFile(filepath.Join(d, "go.mod")) && goModDeclares(filepath.Join(d, "go.mod"), name):
			return Rel(root, filepath.Join(d, "go.mod")), installCmd(cfg, d, "go")
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
	if lock, ok := project.NearestLockfile(cfg, project.PhysicalPath(dir), ecosystem); ok {
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
		path := project.FindUp(dir, root, file)
		if path == "" {
			continue
		}
		var names []string
		if filepath.Base(path) == ".tool-versions" {
			extract.EachLine(path, func(line string) {
				if f := strings.Fields(line); len(f) > 0 && !strings.HasPrefix(f[0], "#") {
					names = append(names, f[0])
				}
			})
		} else {
			names = extract.TOMLKeys(path, ".tools")
		}
		for _, t := range names {
			t = t[strings.LastIndex(t, ":")+1:]
			t = t[strings.LastIndex(t, "/")+1:]
			if alias := cfg.ToolResolution.PinAliases[t]; alias != "" {
				t = alias
			}
			if t == name {
				return strings.TrimPrefix(path, root+"/")
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
