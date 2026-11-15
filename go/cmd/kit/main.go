// Command kit is claude-kit's single binary: hooks and helper scripts as
// subcommands, reached through the bin/kit launcher.
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

	"go.yaml.in/yaml/v3"

	"github.com/ku5ic/claude-kit/go/internal/blast"
	"github.com/ku5ic/claude-kit/go/internal/chatbundle"
	"github.com/ku5ic/claude-kit/go/internal/codegraph"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/design"
	"github.com/ku5ic/claude-kit/go/internal/detect"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/explain"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/gitbase"
	"github.com/ku5ic/claude-kit/go/internal/kitcmd"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/rotate"
	"github.com/ku5ic/claude-kit/go/internal/slop"
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
	exe    string
	paths  config.Paths
	stdout io.Writer
	stderr io.Writer
	cwd    string
	home   string // $HOME, read once here
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
	return &env{exe: exe, paths: paths, stdout: stdout, stderr: stderr, cwd: cwd, home: os.Getenv("HOME")}, nil
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
	name, rest := args[0], args[1:]
	cmd, ok := commands[name]
	if !ok {
		fmt.Fprintf(stderr, "kit: unknown command %q\n%s", name, kitcmd.Usage())
		return 2
	}
	e := &env{stdout: stdout, stderr: stderr}
	if cmd.needs >= needsEnv {
		var err error
		if e, err = newEnv(stdout, stderr); err != nil {
			fmt.Fprintln(stderr, "kit:", err)
			return 1
		}
	}
	var cfg *config.Config
	if cmd.needs == needsConfig {
		var err error
		if cfg, err = e.config(); err != nil {
			fmt.Fprintln(stderr, "kit:", err)
			return 1
		}
	}
	return cmd.run(e, cfg, rest)
}

// needs is what a command reads before it runs.
type needs int

const (
	needsNothing needs = iota // only stdout and stderr: runs where newEnv fails
	needsEnv                  // the kit's paths and the cwd
	needsConfig               // those, and kit.yml
)

// command is one kit subcommand. Its run gets a nil cfg below needsConfig.
type command struct {
	needs needs
	run   func(e *env, cfg *config.Config, args []string) int
}

// commands are kit's subcommands by name, each kitcmd.Commands lists and
// the help and version aliases.
var commands = map[string]command{
	"help":           {needsNothing, cmdHelp},
	"-h":             {needsNothing, cmdHelp},
	"--help":         {needsNothing, cmdHelp},
	"version":        {needsNothing, cmdVersion},
	"--version":      {needsNothing, cmdVersion},
	"completion":     {needsNothing, streams(cmdCompletion)},
	"git-base":       {needsNothing, streams(gitbase.Run)},
	"config":         {needsEnv, envOnly(cmdConfig)},
	"scratch-rotate": {needsEnv, envOnly(cmdScratchRotate)},
	"scratch-dir":    {needsEnv, envOnly(cmdScratchDir)},
	"project-root":   {needsEnv, envOnly(cmdProjectRoot)},
	"subprojects":    {needsEnv, envOnly(cmdSubprojects)},
	"gates":          {needsEnv, envOnly(cmdGates)},
	"bundle":         {needsEnv, envOnly(cmdBundle)},
	"code-graph":     {needsEnv, envOnly(cmdCodeGraph)},
	"hook": {needsEnv, func(e *env, _ *config.Config, args []string) int {
		return cmdHook(e, args, os.Stdin)
	}},
	"contrast": {needsEnv, streams(design.Contrast)},
	"statusline": {needsEnv, func(e *env, _ *config.Config, _ []string) int {
		status.Statusline(os.Stdin, e.stdout, e.paths.ClaudeDir)
		return 0
	}},
	"subagent-statusline": {needsEnv, func(e *env, _ *config.Config, _ []string) int {
		status.SubagentStatusline(os.Stdin, e.stdout)
		return 0
	}},
	"tasks":        {needsConfig, cmdTasks},
	"detect-stack": {needsConfig, cmdDetectStack},
	"run-checks":   {needsConfig, cmdRunChecks},
	"enforce":      {needsConfig, cmdEnforce},
	"blast-radius": {needsConfig, func(e *env, cfg *config.Config, args []string) int {
		return blast.Run(cfg, args, e.stdout, e.stderr)
	}},
	"slop": {needsConfig, func(e *env, cfg *config.Config, args []string) int {
		return slop.Run(cfg, args, os.Stdin, e.stdout, e.stderr)
	}},
	"explain": {needsConfig, func(e *env, cfg *config.Config, args []string) int {
		return explain.Run(e.paths, cfg, e.cwd, e.home, args, e.stdout, e.stderr)
	}},
}

// streams is a handler that reads only its args and writes stdout and
// stderr, as a command's run.
func streams(run func([]string, io.Writer, io.Writer) int) func(*env, *config.Config, []string) int {
	return func(e *env, _ *config.Config, args []string) int { return run(args, e.stdout, e.stderr) }
}

// envOnly is a handler that reads no kit.yml as a command's run.
func envOnly(run func(*env, []string) int) func(*env, *config.Config, []string) int {
	return func(e *env, _ *config.Config, args []string) int { return run(e, args) }
}

func cmdHelp(e *env, _ *config.Config, _ []string) int {
	fmt.Fprint(e.stdout, kitcmd.Usage())
	return 0
}

func cmdVersion(e *env, _ *config.Config, _ []string) int {
	fmt.Fprintln(e.stdout, version)
	return 0
}

// cmdCodeGraph reads kit.yml for skip_dirs, which imports and deps leave
// out, and for model-dir's architecture folder.
func cmdCodeGraph(e *env, args []string) int {
	cfg, err := e.config()
	if err != nil {
		fmt.Fprintln(e.stderr, "kit:", err)
		return 1
	}
	if len(args) == 0 || args[0] != "model-dir" {
		return codegraph.Run(args, cfg.SkipDirs, e.stdout, e.stderr)
	}
	return codegraph.RunModelDir(cfg.ArchitectureDir, e.home, args[1:], e.stdout, e.stderr)
}

// cmdBundle reads the checkout holding this binary, and kit.yml alone: an
// overlay is this machine's, and what ships isn't.
func cmdBundle(e *env, args []string) int {
	paths, err := config.ResolveCheckout(e.exe)
	if err != nil {
		fmt.Fprintln(e.stderr, "kit:", err)
		return 1
	}
	cfg, warnings, err := config.LoadBase(paths)
	for _, w := range warnings {
		fmt.Fprintln(e.stderr, "kit: warning:", w)
	}
	if err != nil {
		fmt.Fprintln(e.stderr, "kit:", err)
		return 1
	}
	return chatbundle.Run(paths.Root, cfg, args, e.stdout, e.stderr)
}

func cmdRunChecks(e *env, cfg *config.Config, args []string) int {
	a, err := parseRunChecksArgs(args)
	if err != nil {
		fmt.Fprintf(e.stderr, "kit run-checks: %v\nusage: kit run-checks [--plan] [--all] [--only sub...]\n", err)
		return 2
	}
	root := cmp.Or(git.Toplevel(e.cwd), e.cwd)
	subs := project.Subprojects(root)
	for _, sub := range a.only {
		if !slices.Contains(subs, sub) {
			fmt.Fprintf(e.stderr, "kit run-checks: %q is not a subproject; kit subprojects lists them\n", sub)
			return 2
		}
	}
	return runGates(e, cfg, root, a)
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
	days, ok := parseDays(arg, e.stderr)
	if !ok {
		return 2
	}
	return rotate.Run(e.paths, e.home, days, dryRun, e.stdout, e.stderr)
}

// runChecksArgs are run-checks' arguments.
type runChecksArgs struct {
	plan bool
	all  bool // the whole tree, not only what changed since the git base
	only []string
}

// parseRunChecksArgs reads [--plan] [--all] [--only sub...].
// Anything else is an error: a typo'd --plan must never fall through to a
// real run. A repeated --only adds to the list; a subproject written as a
// path (./api, api/) is cleaned to its name.
func parseRunChecksArgs(args []string) (runChecksArgs, error) {
	var a runChecksArgs
	for i, arg := range args {
		switch arg {
		case "--plan":
			a.plan = true
		case "--all":
			a.all = true
		case "--only":
			for _, sub := range args[i+1:] {
				switch {
				case sub == "--only":
				case sub == "--plan" || sub == "--all":
					return a, fmt.Errorf("%s must come before --only", sub)
				case strings.HasPrefix(sub, "-"):
					return a, fmt.Errorf("unknown argument %q", sub)
				default:
					a.only = append(a.only, filepath.Clean(sub))
				}
			}
			if len(a.only) == 0 {
				return a, fmt.Errorf("--only needs at least one subproject")
			}
			return a, nil
		default:
			return a, fmt.Errorf("unknown argument %q", arg)
		}
	}
	return a, nil
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

func cmdSubprojects(e *env, args []string) int {
	root := cmp.Or(first(args), git.Toplevel(e.cwd), e.cwd)
	for _, dir := range project.Subprojects(root) {
		fmt.Fprintln(e.stdout, dir)
	}
	return 0
}

func cmdTasks(e *env, cfg *config.Config, args []string) int {
	dir := fsx.PhysicalPath(fsx.Abs(e.cwd, cmp.Or(first(args), ".")))
	root, _ := project.Root(dir)
	for _, task := range enforce.Tasks(cfg, enforce.Options{Root: root, CacheDir: e.paths.CacheDir(), Home: e.home}, fsx.Rel(root, dir)) {
		fmt.Fprintln(e.stdout, strings.Join([]string{task.Provider, cmp.Or(task.Stack, "-"), task.Name, task.Cmd}, "\t"))
	}
	return 0
}

func cmdProjectRoot(e *env, args []string) int {
	root, anchored := project.Root(e.cwd)
	if len(args) > 0 && args[0] == "--check" {
		if anchored {
			return 0
		}
		return 1
	}
	fmt.Fprintln(e.stdout, root)
	return 0
}

func cmdScratchDir(e *env, args []string) int {
	dir, err := project.ScratchDir(e.paths, e.cwd, true)
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
	fmt.Fprintln(e.stdout, project.ReportPath(dir, args[0], slug))
	return 0
}

func cmdDetectStack(e *env, cfg *config.Config, _ []string) int {
	root, _ := project.Root(e.cwd)
	fmt.Fprint(e.stdout, detect.Report(cfg, root, e.paths.CacheDir()))
	return 0
}

var positive = regexp.MustCompile(`^[1-9][0-9]*$`)

// parseDays reads a [days] argument: a positive integer, 30 when absent.
func parseDays(arg string, stderr io.Writer) (int, bool) {
	if arg == "" {
		return 30, true
	}
	if !positive.MatchString(arg) {
		fmt.Fprintf(stderr, "scratch-rotate: [days] must be a positive integer, got '%s'\n", arg)
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
