#!/usr/bin/env bash
# PreToolUse hook for Read, Edit, Write, MultiEdit: blocks reads and writes of
# credential files (kit.yml's sensitive_paths) and writes to other risky
# paths (lockfiles, .git/, shell rc files), regardless of permission rules;
# asks before a write to the overlay. Plugins can't ship settings.json deny
# rules, so for a plugin install this is the only credential-read protection.
# guard-dispatch.sh runs the same check.
#
# Implemented by `kit hook guard-edit` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook guard-edit "$@"
