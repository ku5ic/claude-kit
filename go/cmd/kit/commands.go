package main

import (
	"io"
	"os"
	"slices"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/blast"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/design"
	"github.com/ku5ic/claude-kit/go/internal/explain"
	"github.com/ku5ic/claude-kit/go/internal/gitbase"
	"github.com/ku5ic/claude-kit/go/internal/slop"
	"github.com/ku5ic/claude-kit/go/internal/stats"
	"github.com/ku5ic/claude-kit/go/internal/status"
	"github.com/ku5ic/claude-kit/go/internal/vcsread"
)

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
		return stats.Run(e.paths.LogDir(), slices.Concat(config.Rules, config.AskRules), time.Now(), args, e.stdout, e.stderr)
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
