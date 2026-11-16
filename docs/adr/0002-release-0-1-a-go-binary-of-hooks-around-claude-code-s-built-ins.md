# 2. Release 0.1: a Go binary of hooks around Claude Code's built-ins

Date: 2026-10-01

## Status

Accepted

## Context

Release 0.1 (v0.1.0 and v0.1.1, 2026-10-01) set the kit's shape. This record replaces the former ADRs 2 to 6, and its sources are theirs.

### Built-ins

Before the kit was a plugin (commit 909e23e), the setup had grown its own copies of what Claude Code ships: plan mode, code review, security review, Explore, and resume. Eleven `flow-*` skills did this work. The setup cost about 47.5k tokens before any work started, and it could only be used by cloning a dotfiles repo.

### Hooks over prose

A rule in CLAUDE.md, a skill, or a prompt is advice, and Claude can skip it. Permission rules can't fill the gap. A prefix rule can't tell `git push` from `git push --force origin main`, it never sees a command inside a pipeline, and a plugin can't ship deny rules at all.

### One Go binary

The hooks and helpers were bash on top of jq, yq, and bash 4, and the bash guard parsed commands with hand-rolled scanners (commit 4c55364). Every Bash, Edit, and Stop event paid for several process forks, and the parsing missed commands a real shell would run.

### Versioned binaries

The binary has to match the plugin version the user installed, on four platforms. Committing binaries to the repo grows it with every release.

### Fail open

The kit sits in front of every tool call. A bug in it that blocked would stop the user's work for a reason that has nothing to do with their code.

## Decision

### Built-ins

Where Claude Code has a built-in for a job, the kit uses it and adds hooks around it. It doesn't ship its own version. The workflow runs on `/plan`, `/code-review`, `/verify`, and Explore, and the kit's own skills cover only what no built-in does.

### Hooks over prose

A rule the kit means to hold runs as a hook on the command, edit, plan, subagent report, or stop it governs. A broken rule sends Claude back with a message saying what to do next.

### One Go binary

Every hook and helper is one Go binary, called as `kit <subcommand>` or `kit hook <name>` through the `bin/kit` launcher. The bash guard reads commands through a real bash parser, `mvdan.cc/sh`. The port was checked against the bash original on real transcripts and repos, phase by phase, before the original was removed.

### Versioned binaries

`bin/kit` runs `kit-<plugin.json version>-<os>-<arch>`, for darwin and linux on arm64 and amd64. When the binary is missing, it downloads it from the GitHub release `v<version>` (commit 674bc06). With `KIT_DEV=1` it builds from `go/` instead. Binaries are never committed. Only the CLI and the session-start hooks download, since any other hook would be killed mid-download (0.2.1).

### Fail open

A kit bug never blocks the user. A crashing hook fails open and is logged in `~/.claude/logs/guards.jsonl` with its error. A missing binary fails open, and is reported when a session starts without it. An overlay that doesn't merge is dropped, and the kit runs on `kit.yml` alone and says why at session start.

## Consequences

### Built-ins

- Before adding a skill or an agent, check whether a built-in already does the job.
- Per-session context stays small, because built-ins cost the kit nothing to carry.
- When a built-in changes its behavior, the kit's hooks around it may need to follow.

### Hooks over prose

- A new behavior a hook can see becomes a hook. The prose rule shrinks or goes, since the block message carries it when it matters.
- Every block is a turn the user waits through, so a new block has to earn it. `CLAUDE.md` asks for the user's view of a hook change before one ships. Release 0.10 ([9](0009-release-0-10-the-engineer-in-the-loop-the-rules-as-a-gated-skill-a-visible-claim-checker.md)) turned one block into a nudge for that reason.
- What no hook can see, such as a chat reply with no tool call, still depends on prose.

### One Go binary

- Nothing outside git and a bash is needed at runtime.
- A new hook is Go code in `go/internal/hooks`, never a shell script.
- The guard sees what bash would run, including pipelines, wrappers, and heredocs.

### Versioned binaries

- A release has to publish the binaries before the plugin version that names them goes out.
- Until the first download lands, every guard is off, since a missing binary fails open. Offline, a read-only plugin directory, a slow link, or another platform leaves it missing.
- In this repo, `bin/kit` with `KIT_DEV=1` tests the checkout's code. A bare `kit` on PATH is the installed release.

### Fail open

- Every new hook inherits this. A hook that returns an error must let the call through.
- A broken kit is quieter than a working one. The session-start report and the guards log are where it shows.
- An attacker who can crash a hook can get a call past it. Release 0.10 guards the kit's own off switches.
