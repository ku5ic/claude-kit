#!/usr/bin/env bash
# Appends one JSONL line per skill activation to skills.jsonl. Covers three
# paths: UserPromptExpansion (user typed /skillname directly), PostToolUse
# Skill (Claude invoked the Skill tool), and PostToolUse Read - the primary
# signal, direct SKILL.md reads, which is what guard-skills.sh actually
# checks the log for.
#
# Implemented by `kit hook log-skills` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook log-skills "$@"
