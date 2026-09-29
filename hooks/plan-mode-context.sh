#!/usr/bin/env bash
# UserPromptSubmit hook: in plan mode, points Claude at investigate so the
# read-only investigation procedure feeds the plan. Plain stdout on this event
# becomes context. Must stay fast: a timed-out hook's context is discarded.
#
# Implemented by `kit hook plan-mode-context` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook plan-mode-context "$@"
