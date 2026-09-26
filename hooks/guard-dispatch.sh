#!/usr/bin/env bash
# PreToolUse hook for Edit, Write, MultiEdit, Read. Runs guard-edit's and
# guard-skills' checks in one process against one payload read, instead of
# two processes each re-parsing it. Mirrors format-dispatch.sh's role on
# the PostToolUse side. On Read only guard-edit's credential check applies.
#
# Order matches the old settings.json registration: edit-safety, skills-gate.
# A call violating both checks surfaces only the first violation's message
# instead of one per hook - the call is still correctly blocked either way.
HOOK_NAME="guard-dispatch.sh"
# shellcheck source=../bin/_lib.sh
source "$(dirname "$0")/../bin/_lib.sh"
kit_hook_init

read_payload
require_jq

# shellcheck source=guard-edit.sh
source "$(dirname "$0")/guard-edit.sh"
# shellcheck source=guard-skills.sh
source "$(dirname "$0")/guard-skills.sh"

# _lib.sh's ERR trap is process-wide: left in place, an unexpected error in
# ANY one check would fail open the whole dispatcher, skipping the remaining
# checks - a real isolation loss vs. three separate processes. Clear it and
# give each check its own errexit+errtrace+trap inside its own subshell
# instead, restoring per-hook isolation. Verified empirically (clean check,
# internal-error check, blocking check) before writing this.
trap - ERR

run_dispatch_check() {
  local fn="$1" name="$2" rc=0
  # `set +e` around the bare subshell call, not `rc=$?` after it: under the
  # `set -e` still active here, any nonzero subshell exit (a block's `exit 2`
  # included) would trip the *caller's* errexit before rc=$? ever runs.
  # Wrapping in `if`/`||` looks like a fix but bash suppresses -e for
  # everything inside a compound command in that context, including the
  # subshell's own errtrace/ERR trap - confirmed empirically: it broke the
  # malformed-JSON fail-open case, which depends on that inner trap. Plain
  # `set +e` only stops the caller's errexit, leaving the subshell's own
  # set -e/trap intact.
  set +e
  (
    set -e
    set -o errtrace
    # Expanded at trap-set time on purpose: $name is this call's check name.
    # shellcheck disable=SC2064
    trap "echo '${name}: unexpected error, failing open' >&2; exit 0" ERR
    "$fn"
  )
  rc=$?
  set -e
  # `if`, not `((rc != 0)) && exit`: under set -e a standalone `((...))` that
  # evaluates false is itself subject to errexit and would silently kill the
  # dispatcher on every "no violation" pass (hit this exact bug drafting it).
  # An `if` condition is exempt from errexit regardless of its truth value.
  #
  # Only rc==2 (an actual block) aborts the chain. Any other nonzero rc
  # bypassed the subshell's own ERR trap (e.g. a set -u unbound-variable
  # fault, which terminates without invoking ERR) and must not silently skip
  # the remaining checks. soft_rc is deliberately not `local` so the final
  # `exit` below can still report it.
  if ((rc == 2)); then
    exit 2
  fi
  if ((rc != 0)); then
    echo "${name}: exited ${rc}, continuing with remaining checks" >&2
    soft_rc="$rc"
  fi
}

run_dispatch_check run_guard_edit guard-edit.sh
# Opt-in: blocking edits until a patterns skill loads is a personal policy,
# not a default a new adopter should inherit.
if [[ "${CLAUDE_GUARD_SKILLS:-0}" == "1" ]]; then
  run_dispatch_check run_guard_skills guard-skills.sh
fi

exit "${soft_rc:-0}"
