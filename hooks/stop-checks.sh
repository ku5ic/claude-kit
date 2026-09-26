#!/usr/bin/env bash
# Stop hook: runs run-checks.sh when this turn created or edited files via
# Edit/Write/MultiEdit/NotebookEdit, and blocks the stop (exit 2) on failure
# so Claude fixes or reports it. A question-only turn costs nothing.
# Only the subprojects holding an edited file are checked (run-checks.sh
# --only): the deepest kit_subprojects entry containing it, or the root for a
# file in no subproject. Files outside the repo don't count.

HOOK_NAME="stop-checks.sh"
# shellcheck source=../bin/_lib.sh
source "$(dirname "$0")/../bin/_lib.sh"
kit_hook_init

read_payload
require_jq

# Already continuing because of this hook: let the stop through, one retry
# per failure, so a check that can't be fixed never loops.
[[ "$(printf '%s' "$payload" | jq -r '.stop_hook_active // false')" == "true" ]] && exit 0

transcript="$(printf '%s' "$payload" | jq -r '.transcript_path // empty')"
cwd="$(printf '%s' "$payload" | jq -r '.cwd // empty')"
[[ -n "$cwd" ]] && cd "$cwd"
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || exit 0

[[ -r "$transcript" ]] || exit 0
# A clean tree means the edits were committed, which already went through
# verification.
[[ -n "$(git status --porcelain)" ]] || exit 0

# Only run when this turn (entries after the last real user prompt) called a
# file-editing tool. Changes made via Bash, or by the user, don't count.
# One line per edit: the file it touched, empty when the call named none.
mapfile -t edited < <(jq -rs '
  (to_entries
    | map(select(.value.type == "user" and (.value.isMeta | not)
        and ([.value.message.content | arrays | .[] | select(.type == "tool_result")] | length == 0)))
    | last.key // -1) as $start
  | .[$start + 1:][] | select(.type == "assistant") | .message.content | arrays | .[]
  | select(.type == "tool_use"
      and (.name == "Edit" or .name == "Write" or .name == "MultiEdit" or .name == "NotebookEdit"))
  | (.input.file_path // .input.notebook_path // "")
' "$transcript" 2>/dev/null)
((${#edited[@]} > 0)) || exit 0

root="$(git rev-parse --show-toplevel)"
mapfile -t subprojects < <(kit_subprojects "$root")
only=()
full=0
for path in "${edited[@]}"; do
  # An edit without a path can't be placed: check everything.
  if [[ -z "$path" ]]; then
    full=1
    break
  fi
  [[ "$path" == /* ]] || path="$PWD/$path"
  path="$(kit_physical_path "$path")"
  [[ "$path" == "$root/"* ]] || continue
  rel="${path#"$root"/}"
  best=.
  for sub in "${subprojects[@]}"; do
    [[ "$sub" != . && "$rel" == "$sub/"* ]] || continue
    if [[ "$best" == . ]] || ((${#sub} > ${#best})); then
      best="$sub"
    fi
  done
  [[ " ${only[*]:-} " == *" $best "* ]] || only+=("$best")
done

args=()
if ((! full)); then
  ((${#only[@]} > 0)) || exit 0
  args=(--only "${only[@]}")
fi

if ! output="$("$KIT_ROOT/bin/run-checks.sh" "${args[@]}" 2>&1)"; then
  block "checks failed; fix them or report and stop.
$(printf '%s\n' "$output" | grep -E '^FAIL|^checks:' || true)" "checks-failed"
fi

# SKIP lines carry their own reason, e.g. "SKIP js: lint (no lint script)".
summary="$(printf '%s\n' "$output" | grep -E '^(PASS|SKIP|checks:)' || true)"
jq -n --arg msg "$HOOK_NAME: $summary" '{systemMessage: $msg}'
exit 0
