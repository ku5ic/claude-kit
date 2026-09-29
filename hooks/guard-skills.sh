#!/usr/bin/env bash
# PreToolUse hook for Edit, Write, MultiEdit. Blocks edits of mapped file
# types (kit.yml's skill_file_map) until the required patterns skill is
# loaded for this session - one extra round trip per skill-set per session,
# by design. Reads are not gated. guard-dispatch.sh runs the same check when
# CLAUDE_GUARD_SKILLS=1.
#
# Implemented by `kit hook guard-skills` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook guard-skills "$@"
