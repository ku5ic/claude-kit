#!/usr/bin/env bash
# PreToolUse hook: inspects git commit commands for AI signatures, a secret
# in the staged diff (gitleaks, when installed), AI-tell phrasing in the
# subject, and, per rules/output.md section 1, an unchunked wall of text in
# a heredoc body.
#
# Implemented by `kit hook guard-commit` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook guard-commit "$@"
