# 34. Count an edit from its result and a Bash call only for what it changed

Date: 2026-11-15

## Status

Accepted

Amends [31. Count a Bash edit as a code edit for checked claims](0031-count-a-bash-edit-as-a-code-edit-for-checked-claims.md)

## Context

[31](0031-count-a-bash-edit-as-a-code-edit-for-checked-claims.md) sets the last code edit at the `tool_use` of an edit tool or of a recorded Bash call. Two commits moved that point; their bodies and `go/internal/hooks/bashedits.go` say why:

- 9472d00: parallel tool calls land as separate entries before their results, so a claim-checker or `/code-review` launched in the same message as an edit counted as after it, though it may have read the old code.
- 6af6399: `BashEdits` diffs the tree against the one after the last Bash call, else the turn's start. A file an Edit changed since then showed as changed on the next Bash call, whatever that call did. A no-op Bash call after `/code-review` recorded a code edit after the review, and Stop blocked the plan's last tick.

## Decision

An edit counts from its call, then again from its result. `readCheckerLog` in `go/internal/hooks/claims.go` moves the last edit to the `tool_result` of an edit it saw called; plan-done's walk keeps the edits still waiting for a result in `CodeEdits` (`go/internal/hooks/plandone.go`).

A Bash call records only the files it left different from what the turn's last Edit wrote. A file whose content equals that Edit's is the Edit's, and already counted (`BashEdits` in `go/internal/hooks/bashedits.go`).

## Consequences

- A run launched beside an edit, in one message, no longer covers it.
- A Bash call that changes nothing after a review doesn't reopen it.
- A Bash call that puts a file back exactly as the last Edit left it isn't recorded.
- 9472d00 found no run in this project's transcripts whose outcome it changes.
