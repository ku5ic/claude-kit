// Command kit is claude-kit's single binary: hooks and helper scripts as
// subcommands, reached through same-name shims in hooks/ and bin/.
package main

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/ku5ic/claude-kit/go/internal/blast"
	"github.com/ku5ic/claude-kit/go/internal/checks"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/detect"
	"github.com/ku5ic/claude-kit/go/internal/explain"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/gitbase"
	"github.com/ku5ic/claude-kit/go/internal/kitcmd"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/rotate"
	"github.com/ku5ic/claude-kit/go/internal/sources"
	"github.com/ku5ic/claude-kit/go/internal/status"
)

// version is stamped by go/build.sh from .claude-plugin/plugin.json.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// env is what every command needs: the kit's paths and, loaded on first
// use, its config.
type env struct {
	paths  config.Paths
	stdout io.Writer
	stderr io.Writer
	cwd    string
	cfg    *config.Config
	warned bool // config produced warnings
}

// newEnv resolves the kit's paths from the running binary, and the cwd.
func newEnv(stdout, stderr io.Writer) (*env, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	paths, err := config.ResolvePaths(exe)
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &env{paths: paths, stdout: stdout, stderr: stderr, cwd: cwd}, nil
}

func (e *env) config() (*config.Config, error) {
	if e.cfg != nil {
		return e.cfg, nil
	}
	cfg, warnings, err := sources.LoadConfig(e.paths)
	for _, w := range warnings {
		fmt.Fprintln(e.stderr, "kit: warning:", w)
	}
	e.cfg, e.warned = cfg, len(warnings) > 0
	return cfg, err
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, kitcmd.Usage())
		return 2
	}
	e, err := newEnv(stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kit:", err)
		return 1
	}

	name, rest := args[0], args[1:]
	switch name {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, kitcmd.Usage())
		return 0
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	case "completion":
		return cmdCompletion(rest, stdout, stderr)
	case "config":
		return cmdConfig(e, rest)
	case "git-base":
		return gitbase.Run(rest, stdout, stderr)
	case "scratch-rotate":
		return cmdScratchRotate(e, rest)
	case "hook":
		return cmdHook(e, rest, os.Stdin)
	case "statusline":
		status.Statusline(os.Stdin, stdout, e.paths.Home)
		return 0
	case "subagent-statusline":
		status.SubagentStatusline(os.Stdin, stdout)
		return 0
	}

	cmd, ok := configCommands[name]
	if !ok {
		fmt.Fprintf(stderr, "kit: unknown command %q\n%s", name, kitcmd.Usage())
		return 2
	}
	cfg, err := e.config()
	if err != nil {
		fmt.Fprintln(stderr, "kit:", err)
		return 1
	}
	return cmd(e, cfg, rest)
}

// configCommands are the commands that read kit.yml.
var configCommands = map[string]func(*env, *config.Config, []string) int{
	"subprojects":  cmdSubprojects,
	"tasks":        cmdTasks,
	"project-root": cmdProjectRoot,
	"scratch-dir":  cmdScratchDir,
	"detect-stack": cmdDetectStack,
	"run-checks":   cmdRunChecks,
	"enforce":      cmdEnforce,
	"blast-radius": func(e *env, cfg *config.Config, args []string) int {
		return blast.Run(cfg, args, e.stdout, e.stderr)
	},
	"explain": func(e *env, cfg *config.Config, args []string) int {
		return explain.Run(e.paths, cfg, e.cwd, args, e.stdout, e.stderr)
	},
}

func cmdRunChecks(e *env, cfg *config.Config, args []string) int {
	plan, only, err := parseRunChecksArgs(args)
	if err != nil {
		fmt.Fprintf(e.stderr, "kit run-checks: %v\nusage: kit run-checks [--plan] [--only sub...]\n", err)
		return 2
	}
	root := cmp.Or(git.Toplevel(e.cwd), e.cwd)
	subs := project.Subprojects(cfg, root)
	for _, sub := range only {
		if !slices.Contains(subs, sub) {
			fmt.Fprintf(e.stderr, "kit run-checks: %q is not a subproject; kit subprojects lists them\n", sub)
			return 2
		}
	}
	if plan {
		checks.PrintPlan(cfg, root, only, e.stdout)
		return 0
	}
	return min(checks.RunAll(cfg, root, only, e.stdout), 125)
}

func cmdScratchRotate(e *env, args []string) int {
	dryRun, arg := false, ""
	for _, a := range args {
		if a == "--dry-run" {
			dryRun = true
		} else {
			arg = a
		}
	}
	days, ok := parseDays("scratch-rotate", arg, e.stderr)
	if !ok {
		return 2
	}
	return rotate.Run(e.paths, days, dryRun, e.stdout, e.stderr)
}

// parseRunChecksArgs reads [--plan] [--only sub...]. Anything else is an
// error: a typo'd --plan must never fall through to a real run. A repeated
// --only adds to the list; a subproject written as a path (./api, api/) is
// cleaned to its name.
func parseRunChecksArgs(args []string) (plan bool, only []string, err error) {
	for i, arg := range args {
		switch arg {
		case "--plan":
			plan = true
		case "--only":
			for _, a := range args[i+1:] {
				switch {
				case a == "--only":
				case a == "--plan":
					return false, nil, fmt.Errorf("--plan must come before --only")
				case strings.HasPrefix(a, "-"):
					return false, nil, fmt.Errorf("unknown argument %q", a)
				default:
					only = append(only, filepath.Clean(a))
				}
			}
			if len(only) == 0 {
				return false, nil, fmt.Errorf("--only needs at least one subproject")
			}
			return plan, only, nil
		default:
			return false, nil, fmt.Errorf("unknown argument %q", arg)
		}
	}
	return plan, nil, nil
}

func cmdConfig(e *env, args []string) int {
	_, err := e.config()
	if err != nil {
		fmt.Fprintln(e.stderr, "kit:", err)
		return 1
	}
	if len(args) > 0 && args[0] == "--check" {
		if e.warned {
			return 1
		}
		return 0
	}
	merged, err := config.Merged(e.paths)
	if err != nil {
		fmt.Fprintln(e.stderr, "kit:", err)
		return 1
	}
	enc := yaml.NewEncoder(e.stdout)
	enc.SetIndent(2)
	if err := enc.Encode(merged); err != nil {
		fmt.Fprintln(e.stderr, "kit:", err)
		return 1
	}
	return 0
}

func cmdSubprojects(e *env, cfg *config.Config, args []string) int {
	root := cmp.Or(first(args), git.Toplevel(e.cwd), e.cwd)
	for _, dir := range project.Subprojects(cfg, root) {
		fmt.Fprintln(e.stdout, dir)
	}
	return 0
}

func cmdTasks(e *env, cfg *config.Config, args []string) int {
	for _, task := range project.Tasks(cfg, cmp.Or(first(args), ".")) {
		fmt.Fprintln(e.stdout, strings.Join([]string{task.Provider, cmp.Or(task.Stack, "-"), task.Name, task.Cmd}, "\t"))
	}
	return 0
}

func cmdEnforce(e *env, cfg *config.Config, args []string) int {
	if len(args) != 1 || args[0] != "--list" {
		fmt.Fprintln(e.stderr, "usage: kit enforce --list")
		return 2
	}
	root, _ := project.Root(cfg, e.cwd)
	for _, entry := range sources.Entries(cfg, root) {
		fmt.Fprintln(e.stdout, entry)
	}
	return 0
}

func cmdProjectRoot(e *env, cfg *config.Config, args []string) int {
	root, anchored := project.Root(cfg, e.cwd)
	if len(args) > 0 && args[0] == "--check" {
		if anchored {
			return 0
		}
		return 1
	}
	fmt.Fprintln(e.stdout, root)
	return 0
}

func cmdScratchDir(e *env, cfg *config.Config, args []string) int {
	dir, err := project.Dir(cfg, e.paths, e.cwd, "scratch", true)
	if err != nil {
		fmt.Fprintln(e.stderr, "kit scratch-dir:", err)
		return 1
	}
	if len(args) == 0 {
		fmt.Fprintln(e.stdout, dir)
		return 0
	}
	slug := ""
	if len(args) > 1 {
		slug = args[1]
	}
	fmt.Fprintln(e.stdout, project.ReportPath(dir, args[0], slug, time.Now().Format("20060102-1504")))
	return 0
}

func cmdDetectStack(e *env, cfg *config.Config, _ []string) int {
	root, _ := project.Root(cfg, e.cwd)
	fmt.Fprint(e.stdout, detect.Report(cfg, root))
	return 0
}

var positive = regexp.MustCompile(`^[1-9][0-9]*$`)

// parseDays reads a [days] argument: a positive integer, 30 when absent.
func parseDays(name, arg string, stderr io.Writer) (int, bool) {
	if arg == "" {
		return 30, true
	}
	if !positive.MatchString(arg) {
		fmt.Fprintf(stderr, "%s: [days] must be a positive integer, got '%s'\n", name, arg)
		return 0, false
	}
	n, err := strconv.Atoi(arg)
	return n, err == nil
}

func first(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
