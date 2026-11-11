# 8. Give each topic one owner

Date: 2026-10-12

## Status

Accepted

## Context

Guidance was restated across rules, skills, agents, and the README, and the copies drifted (PR #8). Two files saying different things about the same topic leave Claude to pick one.

## Decision

Every behavior the kit applies everywhere has one owner, listed in the table in `CLAUDE.md`. Other files link to the owner or say nothing about the topic. Data and mechanics belong to code, `kit.yml` and the `kit` binary, and prose never re-describes them. Started in 0.2.1 (PR #5), made the rule in 0.4.0 (PR #8).

## Consequences

- A topic missing from the table gets a row before it gets text anywhere else.
- When two files disagree, the owner wins and the other file is fixed.
- `go/e2e/kit_repo_test.go` fails on a pointer to a missing file or section, so moving or renumbering a section means updating its citations in the same change.
