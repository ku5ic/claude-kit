# 23. Fall back to the hook run when the claim checker is missing

Date: 2026-11-11

## Status

Accepted

Amends [21. Run the claim checker as a visible agent](0021-run-the-claim-checker-as-a-visible-agent.md)

Amended by [24. Fall back to the hook run when the claim checker fails](0024-fall-back-to-the-hook-run-when-the-claim-checker-fails.md)

## Context

The gate from [21](0021-run-the-claim-checker-as-a-visible-agent.md) denied a scratch report or a post until the transcript showed a `kit:claim-checker` call on that text. A session that started before the agent was installed has no such agent: the call fails with "Agent type 'kit:claim-checker' not found", and the text could never be written. The gate also counted any call, so a failed one let the text through unchecked.

## Decision

Only a call whose result isn't an error counts as a check. A call whose result says the agent wasn't found switches to the hook's own verifier run on that text, the run subagent reports already get. It sends the claims it finds back once, and the same text resent goes through. Any other failed call counts as no check, and the text stays denied.

## Consequences

- A session started before an update can still write a report, at the cost of the hook's model run.
- A run that errors no longer passes a text unchecked.
- The fallback matches the not-found message's wording. If Claude Code changes it, a session without the agent is denied again until it restarts.
