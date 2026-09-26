#!/usr/bin/env bash
# SubagentStart hook. Runs agent-context.sh for every subagent (matcher: "*"
# in settings.json), replacing the manual "run agent-context.sh via Bash"
# step every agent's Startup section used to need - see rules/agents.md.
# Fires even for agents without Bash (e.g. researcher) since hooks run in the
# harness, independent of the subagent's own tool grants; agents that don't
# act on repo context (researcher, checker) just don't consume it.
#
# SubagentStart supports additionalContext/systemMessage in JSON output but
# not hookSpecificOutput (unlike SessionStart) - see the doc excerpt fetched
# 2026-08-28. agent-context.sh prints plain text; wrap it in that JSON shape.
HOOK_NAME="inject-subagent-context.sh"
# shellcheck source=../bin/_lib.sh
source "$(dirname "$0")/../bin/_lib.sh"
kit_hook_init

payload=""
read_payload

context="$("$KIT_ROOT/bin/agent-context.sh" 2>/dev/null || true)"

[[ -z "$context" ]] && exit 0

jq -cn --arg ctx "$context" '{additionalContext: $ctx}'
