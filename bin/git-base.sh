#!/usr/bin/env bash
# Prints the base branch or ref for the current git checkout. Detection order:
#   1. Explicit base argument (one that doesn't resolve exits 1, in every mode)
#   2. Upstream tracking branch (@{upstream}), unless it's just this branch's own
#      push destination (e.g. `git push -u origin <same-branch-name>`) rather than
#      a distinct merge target - that case is skipped in favor of step 3/4, since
#      a feature branch pushed under its own name always diffs empty against itself.
#   3. Remote HEAD (origin/HEAD)
#   4. Common defaults: main, master, develop, trunk (first that exists)
#
#   git-base.sh [base]                  the base ref
#   git-base.sh --diff [base] [-flags]  git diff <base>...HEAD: three-dot, against
#                                       the merge-base, so commits that landed on
#                                       base after this branch forked don't show
#                                       up reversed - GitHub's PR diff semantics
#   git-base.sh --log [base] [-flags]   git log --oneline <base>..HEAD
#   ... -- <path>...                    limit --diff or --log to those paths
# Flags (e.g. --no-merges, -n 5) pass through to git; the base is the first
# word that resolves as a ref.
# The mode is a flag, not a word, so a branch named diff or log still works as
# the base.
#
# Exits non-zero if nothing resolves. Prints nothing to stderr on normal use.
#
# Implemented by `kit git-base` (go/internal/gitbase).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" git-base "$@"
