#!/usr/bin/env bash
# PreToolUse hook: blocks genuinely destructive bash patterns that
# permission rules can't express reliably, and asks where a prefix rule
# can't tell the safe form from the risky one (git push, pnpm install
# without --frozen-lockfile, a loose redirect into the repo, an overlay
# write). exit 2 blocks, with stderr as the reason shown to Claude.
#
# Commands are parsed as bash (mvdan.cc/sh): every command of every
# pipeline, substitution, subshell, function body, and heredoc fed to a
# shell is checked; quoted text and plain heredoc bodies are data.
#
# Implemented by `kit hook guard-bash` (go/internal/bashguard).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook guard-bash "$@"
