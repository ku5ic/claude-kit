// Command kit is claude-kit's single binary: hooks and helper scripts as
// subcommands, reached through the bin/kit launcher.
package main

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/ku5ic/claude-kit/go/internal/adr"
	"github.com/ku5ic/claude-kit/go/internal/blast"
	"github.com/ku5ic/claude-kit/go/internal/chatbundle"
	"github.com/ku5ic/claude-kit/go/internal/codegraph"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/design"
	"github.com/ku5ic/claude-kit/go/internal/detect"
	"github.com/ku5ic/claude-kit/go/internal/doctor"
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
	"github.com/ku5ic/claude-kit/go/internal/stats"
	"github.com/ku5ic/claude-kit/go/internal/status"
	"github.com/ku5ic/claude-kit/go/internal/vcsread"
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
	shell  string // the Bash tool's shell, bashToolShell, found once here
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
	return &env{exe: exe, paths: paths, stdout: stdout, stderr: stderr, cwd: cwd, home: os.Getenv("HOME"), shell: bashToolShell()}, nil
}

// bashToolShell is the shell Claude Code runs Bash tool commands in, found
// as it finds it (code.claude.com/docs/en/env-vars.md, CLAUDE_CODE_SHELL):
// that variable when it's a working bash or zsh, else $SHELL when it is,
// else the first zsh, then bash, on PATH; "" when there's none.
func bashToolShell() string {
	for _, shell := range []string{os.Getenv("CLAUDE_CODE_SHELL"), os.Getenv("SHELL")} {
		if name := filepath.Base(shell); name == "bash" || name == "zsh" {
			if _, err := exec.LookPath(shell); err == nil {
				return shell
			}
		}
	}
	for _, name := range []string{"zsh", "bash"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
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
	if len(rest) > 0 && (rest[0] == "-h" || rest[0] == "--help") {
		if usage := kitcmd.UsageOf(name); usage != "" {
			fmt.Fprint(stdout, usage)
			return 0
		}
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
	"read":           {needsNothing, streams(vcsread.Run)},
	"config":         {needsEnv, envOnly(cmdConfig)},
	"scratch-rotate": {needsEnv, envOnly(cmdScratchRotate)},
	"clean":          {needsEnv, envOnly(cmdClean)},
	"scratch-dir":    {needsEnv, envOnly(cmdScratchDir)},
	"project-root":   {needsEnv, envOnly(cmdProjectRoot)},
	"adrs":           {needsEnv, envOnly(cmdADRs)},
	"subprojects":    {needsEnv, envOnly(cmdSubprojects)},
	"gates":          {needsEnv, envOnly(cmdGates)},
	"bundle":         {needsEnv, envOnly(cmdBundle)},
	"code-graph":     {needsEnv, envOnly(cmdCodeGraph)},
	"doctor":         {needsEnv, envOnly(cmdDoctor)},
	"stats": {needsEnv, func(e *env, _ *config.Config, args []string) int {
		return stats.Run(e.paths.LogDir(), config.Rules, time.Now(), args, e.stdout, e.stderr)
	}},
	"hook": {needsEnv, func(e *env, _ *config.Config, args []string) int {
		return cmdHook(e, args, os.Stdin)
	}},
	"contrast": {needsEnv, streams(design.Contrast)},
	"statusline": {needsEnv, func(e *env, _ *config.Config, _ []string) int {
		status.Statusline(os.Stdin, e.stdout, e.paths.ClaudeDir)
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
		return explain.Run(e.paths, cfg, e.cwd, e.home, e.shell, args, e.stdout, e.stderr)
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

func cmdDoctor(e *env, args []string) int {
	return doctor.Run(e.paths, version, e.exe, e.cwd, args, e.stdout, e.stderr)
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

func cmdClean(e *env, args []string) int {
	yes := len(args) == 1 && args[0] == "--yes"
	if len(args) > 0 && !yes {
		fmt.Fprintln(e.stderr, "usage: kit clean [--yes]")
		return 2
	}
	return rotate.Clean(e.paths, yes, e.stdout)
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

// cmdADRs prints the project's accepted ADRs that cite args[0], or all of
// them, each as its root-relative file and title.
func cmdADRs(e *env, args []string) int {
	root, _ := project.Root(e.cwd)
	dir := adr.Dir(root)
	if dir == "" {
		fmt.Fprintln(e.stderr, "kit adrs: no ADR folder under", root)
		return 1
	}
	var rel string
	if len(args) > 0 {
		target := args[0]
		if !filepath.IsAbs(target) {
			target = filepath.Join(e.cwd, target)
		}
		rel = filepath.ToSlash(fsx.Rel(root, target))
	}
	for _, a := range adr.Accepted(dir) {
		if rel == "" || a.Cites(rel) {
			fmt.Fprintf(e.stdout, "%s\t%s\n", fsx.Rel(root, a.Path), a.Title)
		}
	}
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
	if strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(e.stderr, "kit scratch-dir: kind %q is a flag\n%s", args[0], kitcmd.UsageOf("scratch-dir"))
		return 2
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

var wholeDays = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

// parseDays reads a [days] argument: 0 or a positive integer, rotate.DefaultDays when absent.
func parseDays(arg string, stderr io.Writer) (int, bool) {
	if arg == "" {
		return rotate.DefaultDays, true
	}
	if !wholeDays.MatchString(arg) {
		fmt.Fprintf(stderr, "scratch-rotate: [days] must be 0 or a positive integer, got '%s'\n", arg)
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
