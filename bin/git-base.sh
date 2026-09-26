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

set -euo pipefail

mode="base"
case "${1:-}" in
--diff)
  mode="diff"
  shift
  ;;
--log)
  mode="log"
  shift
  ;;
esac

# The base is the first word that resolves as a ref. A word right after a
# flag is that flag's value (-n 5) and goes to git with the flags. Any other
# word is a base that doesn't resolve: an error in every mode, not a silent
# fallback that would diff against the wrong branch. After --, words are
# pathspecs, placed after the range where git expects them.
explicit=""
extra=()
paths=()
in_paths=0
prev=""
for arg in "$@"; do
  if ((in_paths)); then
    paths+=("$arg")
  elif [[ "$arg" == -- ]]; then
    in_paths=1
  elif [[ "$arg" == -* ]]; then
    extra+=("$arg")
  elif [[ -z "$explicit" ]] && git rev-parse --verify --quiet "$arg" >/dev/null; then
    explicit="$arg"
  elif [[ "$prev" == -* && "$prev" != *=* ]]; then
    extra+=("$arg")
  else
    echo "git-base.sh: '$arg' is not a ref" >&2
    exit 1
  fi
  prev="$arg"
done

resolve_base() {
  if [ -n "$explicit" ] && git rev-parse --verify "$explicit" >/dev/null 2>&1; then
    echo "$explicit"
    return 0
  fi

  local upstream current_branch resolved b
  if upstream="$(git rev-parse --abbrev-ref '@{upstream}' 2>/dev/null)"; then
    current_branch="$(git rev-parse --abbrev-ref HEAD)"
    if [ "${upstream#*/}" != "$current_branch" ]; then
      echo "$upstream"
      return 0
    fi
  fi

  if git symbolic-ref refs/remotes/origin/HEAD >/dev/null 2>&1; then
    resolved="$(git symbolic-ref --short refs/remotes/origin/HEAD)"
    if git rev-parse --verify "$resolved" >/dev/null 2>&1; then
      echo "$resolved"
      return 0
    fi
  fi

  for b in main master develop trunk; do
    if git rev-parse --verify "$b" >/dev/null 2>&1; then
      echo "$b"
      return 0
    fi
  done
  return 1
}

base="$(resolve_base)" || exit 1

case "$mode" in
base) echo "$base" ;;
diff) git diff "${extra[@]}" "${base}...HEAD" -- "${paths[@]}" ;;
log) git log --oneline "${extra[@]}" "${base}..HEAD" -- "${paths[@]}" ;;
esac
