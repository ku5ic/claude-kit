#!/usr/bin/env bash
# Emits a compact stack report for the current project or a nearby ancestor.
# Output is terse on purpose. Each line is meant to be scanned by Claude in
# under a few hundred tokens of context.
#
# Detection runs in every subproject (the root, tracked anchor-sentinel
# directories, workspace members). One line per stack:
#   <stack>: yes (<extras>) [<package manager>] at <subproject>, ...
# The "at" part is left off when the stack is only at the root. The package
# manager is the nearest lockfile of the stack's own ecosystem, so a uv
# service under a pnpm root reports [uv].
#
# Then one line per subproject with any kit.yml `versions` found:
#   versions [<subproject>]: <name> <version> (<label>), ...
# The bracket part is left off for the root.
#
# All stack knowledge (sentinels, extras detection rules, skills) lives in
# kit.yml. To add a new stack or extend an existing one, edit that file only.
#
# Implemented by `kit detect-stack` (go/internal/detect).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" detect-stack "$@"
