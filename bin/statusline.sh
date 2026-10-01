#!/usr/bin/env bash
# statusLine command, wired in your own settings.json. Reads the payload Claude Code
# pipes to stdin and prints a two-row status: model/agent/dir/git on row 1,
# context/cost/effort/rate-limit on row 2. The actual model is read from the
# transcript, so a skill's model override that didn't take shows in red.
# Never blocks a render: any unexpected input gives a best-effort line.
# STATUSLINE_CACHE_TTL (seconds, default 1) sets how long the git segment is
# cached per session.
#
# Implemented by `kit statusline` (go/internal/status).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" statusline "$@"
