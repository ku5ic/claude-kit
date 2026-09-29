#!/usr/bin/env bash
# Resolves the project root absolute path. Single source of truth for the
# walk-up logic shared by project-name.sh, detect-stack.sh, and any hook
# that needs a project root.
#
# Resolution order:
#   1. Git working tree:    `git rev-parse --show-toplevel` (walks up itself)
#   2. Project anchor walk: $PWD plus up to 2 ancestors, with any kit.yml
#                           sentinel marked anchor: true
#   3. Current directory:   $PWD
#
# Always prints an absolute path. Consumers decide how to handle special
# cases like $HOME or /.
#
# --check: print nothing; exit 0 if tier 1 or 2 found a real anchor, exit 1
# if resolution fell through to bare $PWD.
#
# Implemented by `kit project-root` (go/internal/project).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" project-root "$@"
