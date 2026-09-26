#!/usr/bin/env bash
# Runs a project's declared checks in every subproject kit_subprojects finds:
# the repo root, directories holding a tracked anchor sentinel, and workspace
# members. What counts as a task, and how it runs, is kit.yml data:
#   task_providers    where each runner declares tasks, and its run form
#   checks            which task names count as typecheck, lint, format-check,
#                     and test
#   toolchain_checks  a stack's own check commands (cargo, go), run where that
#                     stack is detected
# It never invokes a third-party linter binary directly off a config file. A
# check with no declared task is skipped, not synthesized.
#
# Output contract, parsed by hooks/stop-checks.sh: one PASS, FAIL, or SKIP
# line per check, labeled "<stack>: <check> (<task>) [<subproject>]" (a
# provider without a stack labels with its own name; the root has no
# [<subproject>]), then a final "checks: N passed, N failed, N skipped" line.
# Exit status is the failure count. Each check is independent: failures are
# reported, not aborted.
#
#   run-checks.sh                      every subproject
#   run-checks.sh --only . services/api  only these (as kit_subprojects names
#                                        them; "." is the root)
set -uo pipefail

only=()
if [[ "${1:-}" == --only ]]; then
  shift
  only=("$@")
fi

# Before the cd: a relative script path only resolves from the caller's cwd.
# shellcheck source=_lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$root" || exit 1
kit_stacks_load

pass=0
fail=0
skip=0

# run <label> <dir> <command...>: runs the command in <dir>.
run() {
  local label="$1" dir="$2"
  shift 2
  local out
  # Portable template: `mktemp -t <prefix>` differs between BSD (macOS) and GNU
  # (Linux CI); an explicit path template with X's behaves the same on both.
  out="$(mktemp "${TMPDIR:-/tmp}/run-checks.XXXXXX")"
  if (cd "$dir" && "$@") >"$out" 2>&1; then
    echo "PASS $label"
    pass=$((pass + 1))
  else
    echo "FAIL $label ($*)"
    head -30 "$out"
    fail=$((fail + 1))
  fi
  rm -f "$out"
}

skip_msg() {
  echo "SKIP $1"
  skip=$((skip + 1))
}

# True when task name $2 matches any glob in the space-separated list $1.
matches_any() {
  local glob
  local -a globs
  read -ra globs <<<"$1"
  for glob in "${globs[@]}"; do
    # shellcheck disable=SC2053  # a glob on purpose
    [[ "$2" == $glob ]] && return 0
  done
  return 1
}

# True when task name $2 counts as check index $1: it matches the check's
# task globs and none of its exclude globs.
task_matches_check() {
  matches_any "${KIT_CHECK_TASKS[$1]}" "$2" || return 1
  [[ "${KIT_CHECK_EXCLUDES[$1]}" == - ]] && return 0
  ! matches_any "${KIT_CHECK_EXCLUDES[$1]}" "$2"
}

# Checks an orchestrator ran; their JS provider tasks are skipped per package.
declare -A orchestrated=()

# Runs each check the first usable orchestrator declares a task for, once, at
# the root: its signal file is there and its binary (named after it) is in
# the root's node_modules/.bin. None usable: providers cover JS as usual.
run_orchestrator() {
  local i c bin path task check cmd
  local -a paths tasks parts
  for ((i = 0; i < ${#KIT_ORCH_NAMES[@]}; i++)); do
    [[ -f "$root/${KIT_ORCH_SIGNALS[i]}" ]] || continue
    bin="$root/node_modules/.bin/${KIT_ORCH_NAMES[i]}"
    [[ -x "$bin" ]] || continue
    tasks=()
    read -ra paths <<<"${KIT_ORCH_TASK_PATHS[i]}"
    for path in "${paths[@]}"; do
      mapfile -t -O "${#tasks[@]}" tasks < <(json_keys "$root/${KIT_ORCH_SIGNALS[i]}" "$path")
    done
    for ((c = 0; c < ${#KIT_CHECK_NAMES[@]}; c++)); do
      check="${KIT_CHECK_NAMES[c]}"
      for task in "${tasks[@]}"; do
        task_matches_check "$c" "$task" || continue
        orchestrated[$check]=1
        cmd="${KIT_ORCH_RUNS[i]//\{bin\}/$bin}"
        read -ra parts <<<"${cmd//\{task\}/$task}"
        run "js: $check (${KIT_ORCH_NAMES[i]} affected: $task)" "$root" "${parts[@]}"
      done
    done
    return 0
  done
}

check_subproject() {
  local sub="$1" dir="$root" sfx="" i
  if [[ "$sub" != . ]]; then
    dir="$root/$sub"
    sfx=" [$sub]"
  fi

  local -a task_labels=() task_names=() task_cmds=()
  local provider stack task cmd
  while IFS=$'\t' read -r provider stack task cmd; do
    [[ "$stack" == - ]] && stack="$provider"
    task_labels+=("$stack")
    task_names+=("$task")
    task_cmds+=("$cmd")
  done < <(kit_tasks "$dir")

  # SKIP lines take the first present provider's stack (else its name), so a
  # package.json with no scripts still reports what it lacks.
  local skip_label=""
  while IFS=$'\t' read -r provider stack; do
    if [[ "$stack" != - ]]; then
      skip_label="$stack"
      break
    fi
    [[ -n "$skip_label" ]] || skip_label="$provider"
  done < <(kit_providers "$dir")

  local check j matched
  local -a parts
  for ((i = 0; i < ${#KIT_CHECK_NAMES[@]}; i++)); do
    check="${KIT_CHECK_NAMES[i]}"
    matched=0
    for ((j = 0; j < ${#task_names[@]}; j++)); do
      task_matches_check "$i" "${task_names[j]}" || continue
      [[ -n "${orchestrated[$check]:-}" && "${task_labels[j]}" == js ]] && continue
      matched=1
      read -ra parts <<<"${task_cmds[j]}"
      run "${task_labels[j]}: $check (${task_names[j]})$sfx" "$dir" "${parts[@]}"
    done
    if ((! matched)) && [[ -n "$skip_label" ]]; then
      [[ -n "${orchestrated[$check]:-}" && "$skip_label" == js ]] && continue
      skip_msg "$skip_label: $check$sfx (no $check task)"
    fi
  done

  for ((i = 0; i < ${#KIT_TC_STACKS[@]}; i++)); do
    kit_dir_has_stack "$dir" "${KIT_TC_STACKS[i]}" || continue
    local label="${KIT_TC_STACKS[i]}: ${KIT_TC_NAMES[i]}$sfx"
    if ! kit_toolchain_cmd "$i" "$dir"; then
      skip_msg "$label ($KIT_TC_SKIP)"
      continue
    fi
    read -ra parts <<<"$KIT_TC_CMD"
    run "$label" "$dir" "${parts[@]}"
  done
}

# The orchestrator only when a JS subproject is in scope: a stop hook scoped
# to a Python service has nothing for turbo or nx to do.
wants_js=1
if ((${#only[@]} > 0)); then
  wants_js=0
  for sub in "${only[@]}"; do
    dir="$root"
    [[ "$sub" == . ]] || dir="$root/$sub"
    # A string test, not `| grep -q`: grep exiting early SIGPIPEs cut, and
    # pipefail would turn the match into a failure.
    if [[ $'\n'"$(kit_providers "$dir" | cut -f2)"$'\n' == *$'\n'js$'\n'* ]]; then
      wants_js=1
      break
    fi
  done
fi
((wants_js)) && run_orchestrator

while IFS= read -r sub; do
  if ((${#only[@]} > 0)) && [[ " ${only[*]} " != *" $sub "* ]]; then
    continue
  fi
  check_subproject "$sub"
done < <(kit_subprojects "$root")

echo ""
echo "checks: $pass passed, $fail failed, $skip skipped"
exit "$fail"
