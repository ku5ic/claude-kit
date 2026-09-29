#!/usr/bin/env bash
# subagentStatusLine command (see claude/settings.json). Claude Code runs this
# once per render, not once per row: stdin carries {columns, tasks:[...]} for
# every task in the agent panel, and stdout is parsed as one JSON object per
# line, {"id": <task id>, "content": <rendered line>}. Lines that are not JSON
# matching that shape are discarded and logged, so anything unexpected must
# produce no output rather than a partial or plain-text line.
#
# Content renders "name [status]  <model>  effort:<level>  <ctx%>"; model,
# contextWindowSize and effort are version-gated and omitted when absent.
#
# Implemented by `kit subagent-statusline` (go/internal/status).
dir=${BASH_SOURCE[0]%/*}
[[ $dir == "${BASH_SOURCE[0]}" ]] && dir=.
# shellcheck source=kit
source "$dir/kit" subagent-statusline "$@"
