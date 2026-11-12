# 15. Keep the engineer in the loop

Date: 2026-11-11

## Status

Accepted

## Context

The kit is for developers who review what Claude does. Review is their cost, and a tool that decided for them what needs review would ask them to trust it instead of checking.

## Decision

The kit isn't an autopilot. The engineer approves the plan, reviews each step, and commits. Claude stops after each plan step by default (commit eebcb53, now `step_pause` in `kit.yml`). The kit makes review cheaper and never decides what needs the engineer's judgment: it never marks a step as needing no review and never hides a change. Settled as a constraint for 0.10.0.

## Consequences

- A feature that lowers review cost orders or shortens what the engineer reads. It never skips any of it. The review sheet at each pause lists every changed file, with the ones worth a look first.
- Fewer stops are the engineer's call, through `step_pause`.
- Claude never commits or pushes unasked.
