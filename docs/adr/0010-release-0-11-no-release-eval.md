# 10. Release 0.11: no release eval

Date: 2026-11-15

## Status

Accepted

## Context

Release 0.10 ([9](0009-release-0-10-the-engineer-in-the-loop-the-rules-as-a-gated-skill-a-visible-claim-checker.md)) ran a slim eval suite at each release and compared it against the last scorecard. `ca78f28` (v0.11.0, PR #17) removed it: at 3 runs the suite scored 0.89 for $18.69, and its misses kept no trace to explain them. The trigger cases' read graders counted only Read calls, so a session that read references with `cat` scored 0, and no grader type could see both.

## Decision

No eval suite runs at release. The evals went with the README's "Measured, not promised" section, CONTRIBUTING's Behavior step, and the PR template's rescore box (`ca78f28`).

## Consequences

- A rule change isn't measured by a suite. The skills log still records each reference read, which is how the read rate gets measured (`docs/rules.md`).
- A reply that makes no tool call is now measured by nothing.
- The numbers `docs/rules.md` cites from earlier eval runs stay as history.
