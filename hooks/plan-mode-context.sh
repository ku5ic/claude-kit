#!/usr/bin/env bash
# UserPromptSubmit hook: in plan mode, points Claude at explore-patterns so the
# read-only investigation procedure feeds the plan. Plain stdout on this event
# becomes context. Must stay fast: a timed-out hook's context is discarded.

HOOK_NAME="plan-mode-context.sh"
# shellcheck source=_lib.sh
source "$(dirname "$0")/_lib.sh"

read_payload
require_jq

mode="$(printf '%s' "$payload" | jq -r '.permission_mode // empty' 2>/dev/null || true)"
if [[ "$mode" == "plan" ]]; then
  echo "Plan mode: load the explore-patterns skill and follow it; its findings feed the plan."
fi
exit 0
