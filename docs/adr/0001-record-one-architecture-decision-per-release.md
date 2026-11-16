# 1. Record one architecture decision per release

Date: 2026-11-16

## Status

Accepted

## Context

At 0.11.0 (`ca78f28`), `docs/adr/` held 34 ADRs, all written in 0.10.0 and 0.11.0. Thirteen amended an earlier one, and the claim checker's decision took nine records between 2026-11-10 and 2026-11-15. Many of those were fixes or tunings, recorded within days of the decision they changed, so a reader had to walk a chain to learn what held. The author ruled that noise on 2026-11-16.

## Decision

Each release gets at most one ADR, created with `adr new` at the end of the release and titled after it. It records only an architecture decision or a change in what the kit does for its user. A fix, a tuning, a refactor, or a follow-up to a recent decision gets no ADR, and its commit body says why.

A later release that reverses an earlier decision says so in its own ADR. When it replaces everything an earlier release ADR decided, it supersedes that ADR with `adr new -s`. ADRs are never amended.

The 34 earlier ADRs were replaced on 2026-11-16 by one per past release that made a lasting decision, 2 through 10, each built from the text of the ADRs it replaces and naming the same sources. Old ADRs 1, 22 to 24, and 28 to 34 recorded process or fixes and have no successor. Git history keeps all 34.

## Consequences

- Reading the ADRs of the releases since a version answers what changed and why.
- A release ADR covers several decisions, so a reversal can retire part of one. The later ADR says which part.
- The bar for what needs an ADR, and how to write one, is in `CLAUDE.md`, Architecture decisions.
