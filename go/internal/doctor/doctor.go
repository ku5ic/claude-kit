// Package doctor is `kit doctor`: what's wrong with the kit's install and
// the settings around it. It changes nothing.
package doctor

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/resolve/kittool"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// A check's states. Only a problem changes the exit code.
const (
	ok      = "ok"
	info    = "info"
	problem = "problem"
)

// check is one line: its state, what was checked, and what it found.
type check struct {
	state, name, found string
}

// Run prints one line per check and is 1 when one is a problem. version and
// exe are the running binary's.
func Run(paths config.Paths, version, exe, cwd string, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: kit doctor")
		return 2
	}
	cfg, overlay := overlayCheck(paths)
	checks := []check{
		binaryCheck(paths, version, exe),
		overlay,
		sandboxCheck(paths, cwd),
		attributionCheck(paths, cwd),
	}
	if cfg != nil {
		checks = append(checks, toolsCheck(cfg))
	}
	checks = append(checks, scratchCheck(paths, cwd))

	code := 0
	tw := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	for _, c := range checks {
		if c.state == problem {
			code = 1
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", c.state, c.name, c.found)
	}
	_ = tw.Flush()
	return code
}

// binaryCheck compares the running binary to the plugin whose kit.yml and
// skills it reads.
func binaryCheck(paths config.Paths, version, exe string) check {
	c := check{ok, "binary", version + ", " + exe}
	if want := sources.JSONValue(paths.Manifest, ".version"); want != version {
		c.state, c.found = problem, c.found+", but "+paths.Manifest+" states "+cmp.Or(want, "no version")
	}
	return c
}

// overlayCheck reports what loading kit.yml and what's merged over it warns
// of, and is also the config, nil when kit.yml can't be read.
func overlayCheck(paths config.Paths) (*config.Config, check) {
	c := check{ok, "overlay", "none"}
	cfg, warnings, err := sources.LoadConfig(paths)
	if err != nil {
		c.state, c.found = problem, err.Error()
		return nil, c
	}
	if fsx.IsFile(paths.Overlay) {
		c.found = paths.Overlay
	}
	if len(warnings) > 0 {
		var found []string
		for _, w := range warnings {
			found = append(found, w.String())
		}
		c.state, c.found = problem, strings.Join(found, "; ")
	}
	return cfg, c
}

// sandboxCheck reads sandbox.enabled as Claude Code does: the highest file
// setting it, except that the user's false holds over a project's true
// (code.claude.com/docs/en/sandboxing). Managed settings aren't read. Off is
// no problem: the sandbox breaks some tools, so it's the user's call.
func sandboxCheck(paths config.Paths, cwd string) check {
	from, value := "default", "off"
	for _, file := range settingsFiles(paths, cwd) {
		enabled := sources.Decode[struct {
			Sandbox struct{ Enabled *bool }
		}](file, json.Unmarshal).Sandbox.Enabled
		if enabled == nil {
			continue
		}
		if userFalse := file == paths.Settings && !*enabled; from != "default" && !userFalse {
			continue
		}
		from, value = file, "off"
		if *enabled {
			value = "on"
		}
	}
	return check{info, "sandbox", value + " (" + from + ")"}
}

func attributionCheck(paths config.Paths, cwd string) check {
	if file := AttributionOn(paths, cwd); file != "" {
		return check{problem, "attribution", "on (" + file + ")"}
	}
	return check{ok, "attribution", "off"}
}

func toolsCheck(cfg *config.Config) check {
	if _, missing := Tools(cfg.Tools); len(missing) > 0 {
		return check{problem, "tools", "missing: " + strings.Join(missing, ", ")}
	}
	return check{ok, "tools", "none missing"}
}

// scratchCheck asks git about a file in scratch: a pattern for a directory
// matches no directory that doesn't exist yet.
func scratchCheck(paths config.Paths, cwd string) check {
	dir, _ := project.ScratchDir(paths, cwd, false)
	top := git.Toplevel(cwd)
	file := filepath.Join(dir, "x")
	switch {
	case top == "":
		return check{ok, "scratch", "outside a git repo: " + dir}
	case git.Ignored(top, []string{file})[file]:
		return check{ok, "scratch", "gitignored: " + dir}
	}
	return check{problem, "scratch", "not gitignored: " + dir}
}

// AttributionOn names the settings file whose attribution leaves Claude
// Code's attribution on, "default" when none sets it. The highest file
// setting attribution must turn off all of it on its own: the docs don't say
// whether a file replaces the object or merges into it. Managed settings
// aren't read.
func AttributionOn(paths config.Paths, cwd string) string {
	for _, file := range settingsFiles(paths, cwd) {
		var settings struct {
			Attribution json.RawMessage `json:"attribution"`
		}
		data, err := os.ReadFile(file)
		if err != nil || json.Unmarshal(data, &settings) != nil || settings.Attribution == nil {
			continue
		}
		var parts struct {
			Commit, PR *string
			SessionURL *bool `json:"sessionUrl"`
		}
		off := string(settings.Attribution) == "false" ||
			json.Unmarshal(settings.Attribution, &parts) == nil &&
				parts.Commit != nil && *parts.Commit == "" && parts.PR != nil && *parts.PR == "" && parts.SessionURL != nil && !*parts.SessionURL
		if off {
			return ""
		}
		return file
	}
	return "default"
}

// settingsFiles are cwd's settings files, highest first: the project's
// local, then shared, then the user's.
func settingsFiles(paths config.Paths, cwd string) []string {
	files := []string{filepath.Join(paths.ClaudeDir, "settings.json")}
	if root, _ := project.Root(cwd); root != "" {
		files = append([]string{filepath.Join(root, ".claude", "settings.local.json"), filepath.Join(root, ".claude", "settings.json")}, files...)
	}
	return files
}

// Tools splits tools by whether PATH has them. One found under another name
// says which.
func Tools(tools []string) (available, missing []string) {
	for _, tool := range tools {
		path, err := kittool.Path(tool)
		if err != nil {
			missing = append(missing, tool)
			continue
		}
		if name := filepath.Base(path); name != tool {
			tool += " (as " + name + ")"
		}
		available = append(available, tool)
	}
	return available, missing
}
