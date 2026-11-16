# 8. Release 0.9: verify claims before they go out

Date: 2026-11-10

## Status

Accepted

## Context

This record replaces the former ADR 14, and its sources are that record's. The kit caught bad diffs but not bad statements (PR #15). A plan, a subagent report, or a post to Slack could state something nothing in the session backed up.

## Decision

Before a plan is approved, a subagent report or scratch report is shown, or a connector post goes out, a verifier checks its claims against the session's evidence and sends back each claim nothing confirms. A subagent's report is checked against its own transcript. `claims.allow` can only add read-only tools, so it can't turn a write into evidence. Chat replies aren't checked at Stop, because a block there showed the user the reply twice. A per-prompt instruction covers them instead. Made in 0.9.0 (PR #15).

## Consequences

- Each check costs a model run, capped in `kit.yml` `claims:`.
- A chat reply rests on the rule alone.
- `disabled_rules: [claims]` turns it off.
- Release 0.10 ([9](0009-release-0-10-the-engineer-in-the-loop-the-rules-as-a-gated-skill-a-visible-claim-checker.md)) made the verifier a visible agent.
