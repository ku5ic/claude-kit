// Command kit is claude-kit's single binary: hooks and helper scripts as
// subcommands, reached through the bin/kit launcher.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/kitcmd"
	"github.com/ku5ic/claude-kit/go/internal/resolve"
	"github.com/ku5ic/claude-kit/go/internal/sources"
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
	shell  string // the Bash tool's shell, resolve.BashToolShell, found once here
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
	return &env{exe: exe, paths: paths, stdout: stdout, stderr: stderr, cwd: cwd, home: os.Getenv("HOME"), shell: resolve.BashToolShell()}, nil
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
