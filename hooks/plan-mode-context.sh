#!/usr/bin/env bash
# UserPromptSubmit and PostToolUse (ExitPlanMode) hook: points Claude at
# investigate in plan mode, then holds an approved plan to one step per turn
# while it has open "- [ ]" steps. Must stay fast: a timed-out hook's context
# is discarded.
#
# Implemented by `kit hook plan-mode-context` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook plan-mode-context "$@"
