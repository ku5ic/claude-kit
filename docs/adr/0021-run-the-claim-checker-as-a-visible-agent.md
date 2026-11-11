# 21. Run the claim checker as a visible agent

Date: 2026-11-11

## Status

Accepted

Amends [14. Verify claims before they go out](0014-verify-claims-before-they-go-out.md)

## Context

The verifier from [14](0014-verify-claims-before-they-go-out.md) was a `claude -p` run the hook started out of sight, and a failed run was only logged. The user couldn't read what it checked or see what it cost.

## Decision

The checker is `agents/claim-checker.md`, an agent anyone can read. The hook still decides when it must run, because a model choosing when to be checked would decide whether its own claims get verified. A plan, a scratch report, or a connector post is denied until the transcript shows `kit:claim-checker` ran on that exact text, keyed by the text's hash, the way the plan gate waits for `kit:plan-critic`. A subagent can't start an agent, so a subagent's report keeps the hook's own run, driven by the same agent file. A plan's claims are checked in `kit:plan-critic`'s run instead of a second one. Made in 0.10.0.

## Consequences

- The run and its cost show in the transcript like any other agent's.
- Whether Claude acted on the checker's findings is visible in the transcript and isn't enforced.
- The gate needs the agent to exist in the session. A session that started before the agent was installed can't satisfy it.
