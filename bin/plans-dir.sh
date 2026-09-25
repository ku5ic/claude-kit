#!/usr/bin/env bash
# Resolves the plans directory for the current context: project-scoped when
# project-root.sh finds a real anchor (git worktree or stack sentinel), global
# fallback otherwise. Sibling of scratch-dir.sh; see rules/tooling.md
# section 3 for the three-directory split under a project's .claude/.
#
# Unlike scratch-dir.sh this registers nothing in scratch-registry.txt: plan
# files are not on scratch-rotate.sh's prune. They accumulate until cleared by
# hand, which is the point - a plan outlives the 30-day scratch window.
#
# Creates the directory if missing, for the same reason scratch-dir.sh does:
# a caller using a bare `>` redirect has no other chance to mkdir first.

set -euo pipefail

project_root="$(dirname "${BASH_SOURCE[0]}")/project-root.sh"
if "$project_root" --check; then
  dir="$("$project_root")/.claude/plans"
else
  dir="$HOME/.claude/plans"
fi

mkdir -p "$dir"
echo "$dir"
