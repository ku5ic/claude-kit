#!/usr/bin/env bash
# PreToolUse hook for Edit, Write, MultiEdit, Read. Runs guard-edit's and
# (with CLAUDE_GUARD_SKILLS=1) guard-skills' checks against one payload read,
# in that order. Each check is isolated: one failing open never skips the
# next, and the first block wins. On Read only guard-edit's credential check
# applies.
#
# Implemented by `kit hook guard-dispatch` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook guard-dispatch "$@"
