#!/usr/bin/env bash
# Prints `git diff <base>...HEAD` where <base> is resolved by git-base.sh.
# Three-dot diff (against the merge-base) so commits that landed on base
# after this branch forked don't show up reversed in the output - matches
# GitHub's own PR diff semantics.
# Accepts an optional explicit base via $1.
set -euo pipefail
base="$(git-base.sh "${1:-}")"
git diff "${base}...HEAD"
