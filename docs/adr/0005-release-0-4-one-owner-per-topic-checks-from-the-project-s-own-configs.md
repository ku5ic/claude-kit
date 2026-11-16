# 5. Release 0.4: one owner per topic, checks from the project's own configs

Date: 2026-10-12

## Status

Accepted

## Context

Release 0.4 (v0.4.0, PR #8) settled where guidance lives and where checks come from. This record replaces the former ADRs 8 and 10, and its sources are theirs.

### One owner

Guidance was restated across rules, skills, agents, and the README, and the copies drifted (PR #8). Two files saying different things about the same topic leave Claude to pick one.

### Project configs

A check catalog in `kit.yml` guessed what a project runs instead of reading what it declares (PR #8). Before that, `run-checks` found gates by task name only, so a gate under an odd name, inside an aggregate script, or only in CI never ran (PR #7).

## Decision

### One owner

Every behavior the kit applies everywhere has one owner, listed in the table in `CLAUDE.md`. Other files link to the owner or say nothing about the topic. Data and mechanics belong to code, `kit.yml` and the `kit` binary, and prose never re-describes them. Started in 0.2.1 (PR #5).

### Project configs

The catalog and its keys are deleted. `run-checks`, the Stop hook, per-edit formatting, and the commit-message check read the project's own configs: pre-commit, lint-staged, lefthook, husky, commitlint, turbo, nx, task files, and CI steps (`go/internal/sources`). Task bodies are read with a real shell parser. Gap-fill, a Haiku run through the `claude` CLI, classifies only what those configs leave open. Its verdicts are checked against the files and cached, and a proposal must run one plain binary.

## Consequences

### One owner

- A topic missing from the table gets a row before it gets text anywhere else.
- When two files disagree, the owner wins and the other file is fixed.
- `go/e2e/kit_repo_test.go` fails on a pointer to a missing file or section, so moving or renumbering a section means updating its citations in the same change.
- Release 0.10 ([9](0009-release-0-10-the-engineer-in-the-loop-the-rules-as-a-gated-skill-a-visible-claim-checker.md)) let pages written for people explain the mechanics.

### Project configs

- The kit runs what the project's own people run, and nothing they don't.
- A task named for a check whose body fixes or watches is skipped with the reason, so a gate never rewrites sources or hangs.
- Deploy-like, secret-using, OIDC, and service-backed CI jobs never run.
- Gap-fill needs the `claude` CLI logged in, and costs a capped Haiku call per batch.
