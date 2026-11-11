# 11. Block lint-like checks only on changed lines

Date: 2026-11-11

## Status

Accepted

## Context

Stop runs a project's checks on the files Claude edited ([10](0010-read-checks-from-the-project-s-own-configs.md)). An edited file can carry findings on lines the turn never touched.

## Decision

A lint-like check blocks only on changed lines. Dead-code tools (knip, vulture, deadcode, tflint's unused declarations) block only on lines changed since the git base, and their findings decide, not their exit codes, since these tools disagree on them. Made in 0.3.0 (PR #7) and 0.4.0 (PR #8).

## Consequences

- A finding on an untouched line doesn't block the turn.
- A check has to report line numbers the kit can match to the diff.
- Debt in the files Claude edits stays until someone fixes it on purpose.
