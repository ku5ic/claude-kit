#!/usr/bin/env bash
# Stop hook: runs run-checks.sh when the working tree changed since the last
# Stop in this session, and blocks the stop (exit 2) on failure so Claude
# fixes or reports it. A question-only turn leaves the tree hash unchanged and
# costs nothing.

HOOK_NAME="stop-checks.sh"
# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"

read_payload
require_jq

# Already continuing because of this hook: let the stop through, one retry
# per failure, so a check that can't be fixed never loops.
[[ "$(printf '%s' "$payload" | jq -r '.stop_hook_active // false')" == "true" ]] && exit 0

session_id="$(printf '%s' "$payload" | jq -r '.session_id // empty')"
cwd="$(printf '%s' "$payload" | jq -r '.cwd // empty')"
[[ -n "$session_id" ]] || exit 0
[[ -n "$cwd" ]] && cd "$cwd"
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || exit 0

status="$(git status --porcelain)"
state="$({
  printf '%s\n' "$status"
  git diff HEAD 2>/dev/null
  # git diff skips untracked files; hash their contents so editing one counts.
  # Regular files only: hash-object fails on a symlink to a directory, and
  # symlinks already show up by name in the status above.
  git ls-files --others --exclude-standard -z | while IFS= read -r -d '' path; do
    if [[ -f "$path" && ! -L "$path" ]]; then git hash-object -- "$path"; fi
  done
} | git hash-object --stdin)"

state_dir="$HOME/.claude/logs/stop-checks"
state_file="$state_dir/$session_id"
mkdir -p "$state_dir"
previous="$(cat "$state_file" 2>/dev/null || true)"
printf '%s\n' "$state" >"$state_file"

[[ "$state" == "$previous" ]] && exit 0
# First stop of the session on a clean tree: nothing was changed.
[[ -z "$previous" && -z "$status" ]] && exit 0

if ! output="$("$KIT_ROOT/bin/run-checks.sh" 2>&1)"; then
  block "checks failed; fix them or report and stop.
$(printf '%s\n' "$output" | grep -E '^FAIL|^checks:' || true)"
fi
exit 0
