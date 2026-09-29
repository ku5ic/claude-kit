#!/usr/bin/env bash
# Prunes ~/.claude/scratch/ artifacts older than 30 days, plus every
# project-scoped scratch/ dir registered in scratch-registry.txt (see
# scratch-dir.sh - this run has no project cwd of its own, only $HOME, so
# the registry is the only way it finds those directories).
# Also trims every ~/.claude/logs/*.jsonl to kit.yml's log_max_lines.
# Run manually or wire to launchd. Safe to run repeatedly; idempotent.
#
#   scratch-rotate.sh             # 30 days (default)
#   scratch-rotate.sh 14          # 14 days
#   scratch-rotate.sh --dry-run   # report what would go, delete nothing
#
# The project tier deletes files of any extension at any depth, so every
# registry line is validated first: absolute, under $HOME, a real directory
# named exactly `scratch`, not a symlink, not a repo. A single bad line would
# otherwise mean unrecoverable mass deletion. Every deleted path is appended
# to the rotate log so an accident stays diagnosable after the fact.
#
# Implemented by `kit scratch-rotate` (go/internal/rotate).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" scratch-rotate "$@"
