#!/usr/bin/env bash
# SessionStart hook. Prepends repo context at session start/resume/compact/
# clear: prerequisite warnings, <repo-context>, <required-skills>,
# <suggested-skills>, and <tooling>. The harness guarantees this fires once
# per boundary (matcher: startup|resume|compact|clear in settings.json).
#
# Implemented by `kit hook inject-context` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook inject-context "$@"
