#!/usr/bin/env bash
# Reports skill activation telemetry from ~/.claude/logs/skills.jsonl.
# Read-only: never writes, trims, or rotates the log (see scratch-rotate.sh
# for trimming). Malformed JSONL lines are counted and reported, never fatal.
#
# Reports, over a window defaulting to 30 days (override as the first argument):
#   1. Activation count per skill, descending
#   2. Counts split by path: slash command, Skill tool invocation, Read
#      fallback, plus a separate "surfaced only" section for the
#      required-skill/suggested-skill markers inject-context writes (shown
#      to the model, not confirmed loaded)
#   3. Skills referenced anywhere in kit.yml with zero activations
#   4. Activated skills that appear nowhere in kit.yml (expected for audit,
#      write, meta, deps, and investigate procedure skills)
#   5. Sessions where a suggested skill was surfaced but never activated
#   6. Guard rules that blocked (or would have, when disabled) per
#      guards.jsonl: count, disabled count, last seen
#
#   skills-report.sh        # last 30 days
#   skills-report.sh 7      # last 7 days
#
# Implemented by `kit skills-report` (go/internal/report).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" skills-report "$@"
