#!/usr/bin/env bash
# Runs a project's declared checks in every subproject: the repo root,
# directories holding a tracked anchor sentinel, and workspace members. What
# counts as a task, and how it runs, is kit.yml data:
#   task_providers    where each runner declares tasks, and its run form
#   checks            which task names count as typecheck, lint, format-check,
#                     and test
#   toolchain_checks  a stack's own check commands (cargo, go), run where that
#                     stack is detected
#   orchestrators     turbo or nx, run once at the root when usable
# It never invokes a third-party linter binary directly off a config file. A
# check with no declared task is skipped, not synthesized.
#
# Output contract, read by the checker agent: one PASS, FAIL, or SKIP
# line per check, labeled "<stack>: <check> (<task>) [<subproject>]" (a
# provider without a stack labels with its own name; the root has no
# [<subproject>]), then a final "checks: N passed, N failed, N skipped" line.
# Exit status is the failure count. Each check is independent: failures are
# reported, not aborted.
#
#   run-checks.sh                        every subproject
#   run-checks.sh --only . services/api  only these (as `kit subprojects`
#                                        names them; "." is the root)
#
# Implemented by `kit run-checks` (go/internal/checks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" run-checks "$@"
