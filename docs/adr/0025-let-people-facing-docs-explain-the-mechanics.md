# 25. Let people-facing docs explain the mechanics

Date: 2026-11-12

## Status

Accepted

Amends [8. Give each topic one owner](0008-give-each-topic-one-owner.md)

## Context

[8](0008-give-each-topic-one-owner.md) keeps prose from re-describing `kit.yml` and the `kit` binary, because guidance restated across rules, skills, agents, and the README drifted (PR #8). That drift mattered because Claude read the copies.

The README never followed it for people. It describes every guard, the `kit run-checks` flags, and every `kit.yml` key, and no rule, skill, or agent links it, so Claude never reads it as guidance. At 362 lines, that detail buried what the kit is for. This change moves it to three pages under `docs/`, which held only `docs/rules.md` and the ADRs.

## Decision

`README.md`, `docs/guards.md`, `docs/checks.md`, and `docs/configuration.md` may explain the kit's mechanics to people. They own nothing: when one disagrees with `kit.yml` or the binary, the code wins and the page is fixed. A change to a guard, a check, or a `kit.yml` key updates the page that describes it in the same change. No rule, skill, or agent links them.

## Consequences

- Restated mechanics can drift again, now across four files. `/audit doc-drift` finds it, as it did for 3d6eb19.
- `go/e2e/kit_repo_test.go` checks the pages' pointers as it does the README's, `docs/adr/` excepted, since an accepted ADR changes only in its status line. It doesn't check their content against `kit.yml`.
- `docs/rules.md` keeps its own row in `CLAUDE.md`.
