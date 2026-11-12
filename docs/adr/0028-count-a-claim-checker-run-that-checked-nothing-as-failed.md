# 28. Count a claim checker run that checked nothing as failed

Date: 2026-11-12

## Status

Accepted

Amends [24. Fall back to the hook run when the claim checker fails](0024-fall-back-to-the-hook-run-when-the-claim-checker-fails.md)

## Context

`agents/claim-checker.md` listed Read but not Skill, so under [22](0022-let-agents-that-cannot-load-the-rules-through-the-gate.md) the rules gate in `go/internal/hooks/rulesgate.go` gated it, and its only way to load the rules was a Read of each `SKILL.md`. A run verifying PR #16 had those Reads refused by Claude Code's permissions, and the gate then denied every other call the checker made. The checker reported that it had checked nothing, but its Agent call ended normally, and `checkerRun` in `go/internal/hooks/claims.go` counted any normal end as a finished check, so the text it was asked about went through.

## Decision

The claim checker gets the Skill tool, so it loads the rules the way plan-critic does. And a run whose checks all errored counts as failed: when a foreground run ends, or a background run's notice says `completed`, `checkerRun` reads the agent's transcript, named by the `agentId:` line in the Agent tool's result. If the run made checks, meaning tool calls other than a Skill load, and every one returned an error, the text takes the hook's own verifier run, as a failed run does under 24.

## Consequences

- A checker blinded by the rules gate or a permission denial no longer passes a text unchecked. The cost is the hook's model run, and its send-back once.
- A run that needed no check, its claims settled by the evidence in its prompt, still counts.
- The `agentId:` line is undocumented, like the notice's `<status>` tag. If Claude Code drops it, the transcript can't be found and a run counts as finished, as before this ADR.
