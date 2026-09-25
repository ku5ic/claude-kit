#!/usr/bin/env bash
# Resolves the scratch directory for the current context: project-scoped
# when project-root.sh finds a real anchor (git worktree or stack
# sentinel), global fallback otherwise. Single source of truth for
# rules/tooling.md.
#
# project-name.sh's home/root/unknown categories are not a project signal
# (every directory gets a slug, anchored or not), so this checks
# project-root.sh's own anchor detection directly via --check.
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
#
# Registers each project dir it resolves in scratch-registry.txt so
# scratch-rotate.sh's scheduled (launchd) run can find and prune it later -
# that run has no project cwd of its own, only $HOME.

set -euo pipefail

project_root="$(dirname "${BASH_SOURCE[0]}")/project-root.sh"
if "$project_root" --check; then
  dir="$("$project_root")/.claude/scratch"
  registry="$HOME/.claude/logs/scratch-registry.txt"
  mkdir -p "$(dirname "$registry")"
  grep -qxF "$dir" "$registry" 2>/dev/null || echo "$dir" >>"$registry"
else
  dir="$HOME/.claude/scratch"
fi

mkdir -p "$dir"
echo "$dir"
