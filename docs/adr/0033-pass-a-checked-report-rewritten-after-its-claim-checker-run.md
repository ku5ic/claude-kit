# 33. Pass a checked report rewritten after its claim checker run

Date: 2026-11-14

## Status

Accepted

Amends [29. Check only the statements no claim checker run has checked](0029-check-only-the-statements-no-claim-checker-run-has-checked.md)

## Context

[29](0029-check-only-the-statements-no-claim-checker-run-has-checked.md) asks for a run on every new or changed statement (`gateOnChecker` in `go/internal/hooks/claims.go`). The deny tells Claude to check or cut each claim the run reports, then write the report again. Cutting or rewording a flagged claim changes its statement, so the next write asks for another run, and the one after that too. This change's commit records three blocks in a row on one explore report.

The Decision traces to this change's commit and `gateOnChecker`.

## Decision

A report file a claim-checker run finished on goes through when written again, whatever its statements. The asked record (`cache.Claims`, `asked`) gains a third field, the report's path, and `checkedStatements` returns the paths of the finished runs.

Only report files count. A plan or a connector post keeps 29's per-statement gate, since a second post through the same tool is a new message, not a revision.

## Consequences

- Acting on the checker's report takes one write, not a run per revision.
- A claim added to a checked report goes out unchecked. A code edit after the run still voids it, per [31](0031-count-a-bash-edit-as-a-code-edit-for-checked-claims.md).
- An asked record written before this change has no path field and reads as no report.
