#!/usr/bin/env bash
# Resolves the scratch directory for the current context: project-scoped
# when project-root.sh finds a real anchor (git worktree or stack
# sentinel), global fallback otherwise. Single source of truth for
# rules/tooling.md. The resolution lives in _lib.sh's kit_dir.
#
#   scratch-dir.sh                 the directory
#   scratch-dir.sh <kind> [slug]   a report path in it:
#                                  <dir>/<kind>-<slug>-<YYYYMMDD-HHMM>.md
# The slug is made filename-safe (anything outside [A-Za-z0-9._-] becomes
# "-"); the timestamp is the local clock, never a guess.
#
# Creates the directory if missing: some callers (agent instructions using
# a bare `>` redirect instead of the Write tool) have no other chance to
# mkdir before their first write.
#
# Both tiers live under .claude/ so scratch sits beside plans/ and tasks/.
# Claude Code gates .claude/ writes behind its own confirmation; a project's
# settings.json carries Edit()/Write() allows for these paths. If the prompts
# come back regardless, move the project tier back to <root>/scratch - that
# was the previous arrangement, and that gating was the reason for it.

set -euo pipefail

# shellcheck source=_lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

dir="$(kit_dir scratch)"
if (($# == 0)); then
  printf '%s\n' "$dir"
  exit 0
fi

kind="${1//[^A-Za-z0-9._-]/-}"
slug="${2:-}"
slug="${slug//[^A-Za-z0-9._-]/-}"
printf '%s/%s%s-%s.md\n' "$dir" "$kind" "${slug:+-$slug}" "$(date +%Y%m%d-%H%M)"
