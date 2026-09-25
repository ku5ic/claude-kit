#!/usr/bin/env bash
# subagentStatusLine command (see claude/settings.json). Claude Code runs this
# once per render, not once per row: stdin carries {columns, tasks:[...]} for
# every task in the agent panel, and stdout is parsed as one JSON object per
# line, {"id": <task id>, "content": <rendered line>}. Lines that are not JSON
# matching that shape are discarded and logged, so anything unexpected must
# produce no output rather than a partial or plain-text line.
#
# Content renders "name [status]  <model>  effort:<level>  <ctx%>": a single
# space within the name/status group, double space between that and each
# following field, so model/effort read as distinct from the task name rather
# than concatenated onto it. model, contextWindowSize and effort are
# version-gated (model/contextWindowSize below Claude Code v2.1.205, effort
# below v2.1.214) and are omitted when absent.

set -euo pipefail
trap 'exit 0' ERR

payload="$(cat)"

# No plain-text notice when jq is missing, unlike statusline.sh: every line
# here is schema-checked, so a notice would only surface as a parse error.
command -v jq >/dev/null 2>&1 || exit 0

jq -c '
  .tasks[]?
  | select((.id // "") != "")
  | {
      id: (.id | tostring),
      content: (
        ((.name // .label // .description // "task") as $n
         | (if (.status // "") == "" then $n else "\($n) [\(.status)]" end)) as $head
        | [$head] + [
            (if (.model // "") == "" then empty else "\(.model)" end),
            (if (.effort // "") == "" then empty else "effort:\(.effort)" end),
            (if (.contextWindowSize // 0) > 0 and (.tokenCount // null) != null
             then "\(.tokenCount * 100 / .contextWindowSize | floor)%"
             else empty end)
          ]
        | join("  ")
      )
    }
' <<<"$payload"
