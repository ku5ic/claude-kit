# 19. Run a slim eval suite at release

Date: 2026-11-11

## Status

Accepted

## Context

The full eval baseline for 0.9.0 took about 24 minutes and $16.66 at 86 of 110 runs, which the author ruled unacceptable. A case whose with-kit and without-kit arms score the same can't show what the kit changes.

## Decision

The 0.9.0 baseline ran once with `--ablation with-without`, and its aggregate is committed as `evals/scorecard/0.9.0.json`. Cases whose two arms scored the same were deleted. The remaining suite runs once per case, four at a time, at release only. Decided by the author for 0.10.0.

## Consequences

- A release compares its scores against the last scorecard. A drop is a regression to fix before release.
- A rule change isn't measured until the next release run.
- A new case has to show a difference between the arms, or it doesn't stay.
