# 31. Count a Bash edit as a code edit for checked claims

Date: 2026-11-14

## Status

Accepted

Amends [29. Check only the statements no claim checker run has checked](0029-check-only-the-statements-no-claim-checker-run-has-checked.md)

## Context

[29](0029-check-only-the-statements-no-claim-checker-run-has-checked.md) counts a statement as checked once the transcript shows a finished claim-checker run that covered it. b6fbc4f then dropped a run launched before the last code edit (`calls[b.ID] = i > l.lastEdit` in `go/internal/hooks/claims.go`), which 29 doesn't record. That last edit came from `transcript.Edits`, which counts only Edit, Write, MultiEdit, and NotebookEdit (`EditTools` in `go/internal/transcript/transcript.go`). A `sed -i`, a `git checkout`, or a script that rewrote code left an earlier run standing.

## Decision

A claim-checker run checks nothing once code changes after it launched, by an edit tool or by a Bash command. Scratch and the plan dir don't count.

`BashEdits` already diffs the tree after each Bash call. When that diff holds a code file, it appends the payload's `tool_use_id` to a per-session record under `cache.Claims`. `readCheckerLog` sets the last edit at a transcript entry holding a recorded `tool_use`, as it does for an edit tool.

## Consequences

- A statement checked before a Bash command changed code needs a fresh run.
- A Bash command the guard marked read-only isn't diffed, so it's never recorded.
- The record is per session, like 29's.
