# 29. Check only the statements no claim checker run has checked

Date: 2026-11-13

## Status

Accepted

Amends [21. Run the claim checker as a visible agent](0021-run-the-claim-checker-as-a-visible-agent.md)

Amended by [31. Count a Bash edit as a code edit for checked claims](0031-count-a-bash-edit-as-a-code-edit-for-checked-claims.md)

## Context

Under [21](0021-run-the-claim-checker-as-a-visible-agent.md), a text waits for a `kit:claim-checker` run on that exact text, keyed by the text's hash (`gateOnChecker` in `go/internal/hooks/claims.go`). A report revised after its run, even by one sentence or a heading, got a new hash and needed a full run again. The checker's own report, handed back as a subagent, also went through `VerifyClaims`, which ran the hook's verifier on it.

## Decision

The gate works per statement: each sentence, list item, and table row that states something, as `claims.Statements` finds them. When it denies, it records the claim-check id with the statements that id covers. A statement counts as checked once the transcript shows a finished run on an id that covered it. A text whose statements are all checked goes through. Otherwise the id covers only the unchecked statements, and when some are already checked, the deny lists the rest for the run, with the full text as context.

`VerifyClaims` skips a run whose `agent_type` is the claim checker.

## Consequences

- A revision needs a run only on its new or changed statements, and a heading-only change needs none.
- A statement is judged on its own text. One that stays word for word while the sentences around it change, shifting its meaning, isn't checked again.
- The record is per session, so a new session checks every statement again.
