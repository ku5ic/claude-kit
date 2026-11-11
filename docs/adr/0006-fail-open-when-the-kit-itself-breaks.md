# 6. Fail open when the kit itself breaks

Date: 2026-11-11

## Status

Accepted

## Context

The kit sits in front of every tool call. A bug in it that blocked would stop the user's work for a reason that has nothing to do with their code.

## Decision

A kit bug never blocks the user.

- A crashing hook fails open and is logged in `~/.claude/logs/guards.jsonl` with its error.
- A missing binary fails open. It's reported when a session starts without it.
- An overlay that doesn't merge is dropped, and the kit runs on `kit.yml` alone and says why at session start.

## Consequences

- Every new hook inherits this. A hook that returns an error must let the call through.
- A broken kit is quieter than a working one. The session-start report and the guards log are where it shows.
- An attacker who can crash a hook can get a call past it. [17](0017-guard-only-the-kit-s-own-off-switches.md) covers the kit's own off switches.
