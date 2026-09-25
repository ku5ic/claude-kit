#!/usr/bin/env bash
# Prints the base branch or ref for the current git checkout. Detection order:
#   1. Explicit argument ($1), if it resolves to a valid ref
#   2. Upstream tracking branch (@{upstream}), unless it's just this branch's own
#      push destination (e.g. `git push -u origin <same-branch-name>`) rather than
#      a distinct merge target - that case is skipped in favor of step 3/4, since
#      a feature branch pushed under its own name always diffs empty against itself.
#   3. Remote HEAD (origin/HEAD)
#   4. Common defaults: main, master, develop, trunk (first that exists)
#
# Exits non-zero if nothing resolves. Prints nothing to stderr on normal use.

set -euo pipefail

explicit="${1:-}"

if [ -n "$explicit" ]; then
  if git rev-parse --verify "$explicit" >/dev/null 2>&1; then
    echo "$explicit"
    exit 0
  fi
fi

if upstream="$(git rev-parse --abbrev-ref '@{upstream}' 2>/dev/null)"; then
  current_branch="$(git rev-parse --abbrev-ref HEAD)"
  upstream_branch="${upstream#*/}"
  if [ "$upstream_branch" != "$current_branch" ]; then
    echo "$upstream"
    exit 0
  fi
fi

if git symbolic-ref refs/remotes/origin/HEAD >/dev/null 2>&1; then
  _resolved="$(git symbolic-ref --short refs/remotes/origin/HEAD)"
  if git rev-parse --verify "$_resolved" >/dev/null 2>&1; then
    echo "$_resolved"
    exit 0
  fi
fi

for b in main master develop trunk; do
  if git rev-parse --verify "$b" >/dev/null 2>&1; then
    echo "$b"
    exit 0
  fi
done

exit 1
