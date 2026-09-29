#!/usr/bin/env bash
# PostToolUse hook for Write, Edit, MultiEdit: always strips bidi control
# characters (Trojan Source) from written text files. With
# CLAUDE_SANITIZE_TYPOGRAPHY=1 it also rewrites em dashes, smart quotes, etc.
# to ASCII. Translations, templates, snapshots, and fixtures are skipped.
#
# Implemented by `kit hook sanitize-output` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook sanitize-output "$@"
