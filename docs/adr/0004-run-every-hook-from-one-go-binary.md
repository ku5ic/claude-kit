# 4. Run every hook from one Go binary

Date: 2026-09-29

## Status

Accepted

## Context

The hooks and helpers were bash on top of jq, yq, and bash 4, and the bash guard parsed commands with hand-rolled scanners (commit 4c55364). Every Bash, Edit, and Stop event paid for several process forks, and the parsing missed commands a real shell would run.

## Decision

Every hook and helper is one Go binary, called as `kit <subcommand>` or `kit hook <name>` through the `bin/kit` launcher. The bash guard reads commands through a real bash parser, `mvdan.cc/sh`. The port was checked against the bash original on real transcripts and repos, phase by phase, before the original was removed.

## Consequences

- Nothing outside git and a bash is needed at runtime.
- A new hook is Go code in `go/internal/hooks`, never a shell script.
- The guard sees what bash would run, including pipelines, wrappers, and heredocs.
- The binary has to reach the user's machine, which [5](0005-download-versioned-binaries-from-github-releases.md) settles.
