#!/usr/bin/env bash
# Resolves the project root absolute path. Single source of truth for the
# walk-up logic shared by project-name.sh, detect-stack.sh, and any hook
# that needs a project root.
#
# Resolution order:
#   1. Git working tree:    `git rev-parse --show-toplevel` (walks up itself)
#   2. Project anchor walk: $PWD plus up to 2 ancestors, with any
#                           sentinel in STACK_SENTINELS_PROJECT_ROOT
#                           (see bin/_lib.sh)
#   3. Current directory:   $PWD
#
# Always prints an absolute path. Consumers decide how to handle special
# cases like $HOME or /.
#
# --check: print nothing; exit 0 if tier 1 or 2 found a real anchor, exit 1
# if resolution fell through to bare $PWD. Every existing caller passes no
# argument and is unaffected - this flag is additive only.

set -euo pipefail

check_only=false
[[ "${1:-}" == "--check" ]] && check_only=true

# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"

if root="$(git rev-parse --show-toplevel 2>/dev/null)"; then
  if $check_only; then
    exit 0
  fi
  echo "$root"
  exit 0
fi

dir="$PWD"
depth=0
while [[ "$dir" != "/" && $depth -lt 3 ]]; do
  for f in "${STACK_SENTINELS_PROJECT_ROOT[@]}"; do
    if [[ -f "$dir/$f" ]]; then
      if $check_only; then
        exit 0
      fi
      echo "$dir"
      exit 0
    fi
  done
  dir="$(dirname "$dir")"
  depth=$((depth + 1))
done

if $check_only; then
  exit 1
fi
echo "$PWD"
