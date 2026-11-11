# 24. Fall back to the hook run when the claim checker fails

Date: 2026-11-12

## Status

Accepted

Amends [23. Fall back to the hook run when the claim checker is missing](0023-fall-back-to-the-hook-run-when-the-claim-checker-is-missing.md)

## Context

Under [23](0023-fall-back-to-the-hook-run-when-the-claim-checker-is-missing.md), a `kit:claim-checker` call that failed for any reason other than a missing agent counted as no check, and the text stayed denied. A checker that kept failing (an API error, a timeout) denied every retry, and only `disabled_rules: [claims]` let the text through. The README's verify-claims row already promised that a failed run never blocks. A CodeRabbit review of this branch named the gap.

The subagents now run in the background. A background Agent call returns "Async agent launched" at once, and its report arrives later in a task notification carrying the call's `tool-use-id` and a `status`. `checkerRun` in `go/internal/hooks/claims.go` counted that launch as a finished check.

## Decision

A launch counts as a check only once its notification says `completed`. A call whose result is an error, or whose notification gives any other status, switches to the hook's own verifier run, the same path a missing agent takes. That run sends the claims it finds back once, and the same text resent goes through.

## Consequences

- No checker failure denies a text for good. The cost is the hook's model run.
- A post written while a background checker is still running is denied until its notification arrives.
- Ending a background run reads the notification's `<status>` tag, which Claude Code doesn't document. If the format changes, a background run counts as never ending, and the text stays denied.
