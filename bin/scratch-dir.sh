#!/usr/bin/env bash
# Resolves the scratch directory for the current context: project-scoped
# when project-root.sh finds a real anchor (git worktree or stack
# sentinel), global fallback otherwise. Single source of truth for
# rules/tooling.md.
#
#   scratch-dir.sh                 the directory
#   scratch-dir.sh <kind> [slug]   a report path in it:
#                                  <dir>/<kind>-<slug>-<YYYYMMDD-HHMM>.md
# The slug is made filename-safe (anything outside [A-Za-z0-9._-] becomes
# "-"); the timestamp is the local clock, never a guess.
#
# Creates the directory if missing: some callers (agent instructions using
# a bare `>` redirect instead of the Write tool) have no other chance to
# mkdir before their first write. A project tier is registered in
# scratch-registry.txt so scratch-rotate.sh's cwd-less run can prune it.
#
# Both tiers live under .claude/ so scratch sits beside plans/ and tasks/.
# Claude Code gates .claude/ writes behind its own confirmation; a project's
# settings.json carries Edit()/Write() allows for these paths.
#
# Implemented by `kit scratch-dir` (go/internal/project).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" scratch-dir "$@"
