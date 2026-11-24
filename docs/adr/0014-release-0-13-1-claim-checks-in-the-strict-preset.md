# 14. Release 0.13.1: claim checks in the strict preset

Date: 2026-11-24

## Status

Accepted

Amends [8. Release 0.9: verify claims before they go out](0008-release-0-9-verify-claims-before-they-go-out.md)

Amends [13. Release 0.13: kit rules outrank harness defaults, and the guards' holes closed](0013-release-0-13-kit-rules-outrank-harness-defaults-and-the-guards-holes-closed.md)

## Context

Sources are `kit.yml`, `presets/`, and the files named below.

- Under ADRs 8 and 13, each plan, subagent report, scratch report, and post waits for a verifier run, which `kit.yml` `claims:` capped at 240 seconds and $1, in every preset but `light`.
- The switch to turn that off, `disabled_rules: [claims]`, also drops the per-prompt line telling Claude to check its claims (`go/internal/hooks/replylength.go`).

## Decision

- `kit.yml` sets `claims.timeout: 0`, so the claims hook checks nothing by default. `presets/strict.yml` sets it to 240, which turns on every check ADR 13 describes.
- The per-prompt line and the evidence rule stay on in every preset. `presets/light.yml` no longer disables the `claims` rule, so it keeps the line too.
- `kit:claim-checker` still runs when the user asks for a fact-check.

## Consequences

- With the verifier off, the no-model pass on bare references is off too, a chat reply's included (`go/internal/claims/claims.go`, `go/internal/hooks/chat.go`). `check-citations` still checks a plan's and a gatherer report's `file:line`.
- An overlay that sets `claims.timeout` above 0 turns the checks on without the rest of `strict`.
