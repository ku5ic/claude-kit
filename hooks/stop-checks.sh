#!/usr/bin/env bash
# Stop hook: when this turn created or edited files via
# Edit/Write/MultiEdit/NotebookEdit and left the tree dirty, runs kit.yml's
# file_checks on just those files, and blocks the stop (exit 2) on failure
# so Claude fixes or reports it; a linter's findings block only on changed
# lines. A question-only turn costs nothing. Files
# outside the repo, deleted files, and files no check claims don't count.
# Whole-project checks (run-checks.sh) are left to /code-review.
#
# Implemented by `kit hook stop-checks` (go/internal/hooks, go/internal/checks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook stop-checks "$@"
