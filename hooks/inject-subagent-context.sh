#!/usr/bin/env bash
# SubagentStart hook. Gives every subagent (matcher: "*" in hooks.json)
# agent-context's scratch path, repo context, and skills - see
# rules/agents.md. Fires even for agents without Bash, since hooks run in
# the harness. SubagentStart takes additionalContext in JSON, not plain
# stdout, so the text is wrapped.
#
# Implemented by `kit hook inject-subagent-context` (go/internal/hooks).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=../bin/kit
source "$dir/../bin/kit" hook inject-subagent-context "$@"
