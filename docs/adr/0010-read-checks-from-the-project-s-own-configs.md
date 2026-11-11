# 10. Read checks from the project's own configs

Date: 2026-10-12

## Status

Accepted

## Context

A check catalog in `kit.yml` guessed what a project runs instead of reading what it declares (PR #8). Before that, `run-checks` found gates by task name only, so a gate under an odd name, inside an aggregate script, or only in CI never ran (PR #7).

## Decision

The catalog and its keys are deleted. `run-checks`, the Stop hook, per-edit formatting, and the commit-message check read the project's own configs: pre-commit, lint-staged, lefthook, husky, commitlint, turbo, nx, task files, and CI steps (`go/internal/sources`). Task bodies are read with a real shell parser. Gap-fill, a Haiku run through the `claude` CLI, classifies only what those configs leave open. Its verdicts are checked against the files and cached, and a proposal must run one plain binary. Made in 0.4.0 (PR #8).

## Consequences

- The kit runs what the project's own people run, and nothing they don't.
- A task named for a check whose body fixes or watches is skipped with the reason, so a gate never rewrites sources or hangs.
- Deploy-like, secret-using, OIDC, and service-backed CI jobs never run.
- Gap-fill needs the `claude` CLI logged in, and costs a capped Haiku call per batch.
