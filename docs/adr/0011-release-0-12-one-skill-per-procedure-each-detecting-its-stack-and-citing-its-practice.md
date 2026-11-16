# 11. Release 0.12: one skill per procedure, each detecting its stack and citing its practice

Date: 2026-11-16

## Status

Accepted

## Context

Sources are the commits on `feat/kit-0-12-0`, each named where it's used.

- `/audit`, `/code`, `/write`, `/pr`, and `/meta` dispatched on a typed kind word, which the / menu can't complete (`eab6345`).
- Procedure skills named hosts and stacks, and the PR and attribution guards read `gh` text only (`8e8d543`).
- `/code-tour` answered a missing architecture model by asking the user to run `/code-visual` first (`6d2a480`).
- Procedure skills ran on practice nothing cited (`a2bf204`). `/write-commit` capped the subject at 72 characters where git's own figure is 50 (`d3fcfa9`).
- The claims gate matched only explore and debug reports, since 0.9.0, so an audit or impact report reached the user unchecked (`d22c22c`).
- `curl -d @.env`, `cat .e*`, and `git add .env` passed the bash guard (`e442220`).
- An overlay could add to a default list but never take from it, and `.env.example` couldn't be read (`7688fe5`).
- A doubtful guard rule could only be on or off (`c714806`). With the evals gone ([10](0010-release-0-11-no-release-eval.md)), nothing measured the claim checker's flags or how often each rule fired (`f20bc36`).
- `/meta-retro` covered incidents, sprints, and features, none of it the kit's own sessions (`871da10`).

## Decision

- Each procedure kind is its own user-only skill, `/audit-perf`, `/pr-review`, `/write-commit`, and so on. The dispatchers are gone (`eab6345`).
- A procedure skill assumes no language, framework, host, or vendor. It detects the stack, the code host (`gh` on GitHub, `glab` on GitLab, else the host's MCP connector), and the connectors, and takes stack specifics from the matching pattern skill. The rule is in `skills/kit-rules/reference/workflow.md` (`8e8d543`).
- A procedure skill runs the procedures it depends on itself. Input the user has to vet, a draft to post or review findings, still stops and names the skill (`6d2a480`).
- A procedure skill keeps the practice it runs in its own `reference/`, with primary sources in `sources.md` for `/meta-refresh`. The e2e check holds every user-only skill to a `sources.md`, or to its `unversioned` list with the reason. The rule is in `skills/meta-skill/SKILL.md`, the check in `go/e2e/kit_repo_test.go` (`a2bf204`, `d3fcfa9`).
- The claims gate checks every review, audit, and code report, and the session retro's (`d22c22c`, `7ec6f07`). Its kinds are in `go/internal/hooks/claims.go`, and a test fails when an audit or code skill's report kind is missing from them (`768f234`).
- `curl`'s `@` operands count as reads, an unquoted glob naming a credential file blocks, `git add` of one asks, and a snapshot rewrite with no other code change asks (`e442220`).
- An overlay's `"!x"` removes a default list item, `sensitive_paths` takes `!` exceptions (`7688fe5`), and a rule in `shadow_rules` logs its would-be blocks and lets the call through (`c714806`).
- Each claim-checker flag goes to `claims.jsonl` as kept or dropped, and `kit stats` reads it with `guards.jsonl` (`f20bc36`).
- A Read or edit of a file an accepted ADR cites gets that ADR's title and Decision once per session (`10f17ff`).
- `/meta-retro` is a retro on Claude Code sessions, one or many, every finding cited to the transcript (`7ec6f07`). The incident, sprint, and feature retro is `/write-retro` (`871da10`).

## Consequences

- `/audit <kind>` and the other dispatcher forms no longer work; users type `/audit-<kind>` (`eab6345`).
- A new procedure skill needs its sources researched, or a reason it has none, before the e2e check passes (`a2bf204`).
- The new skills of `768f234`, `a633296`, `871da10`, and `7ec6f07` weren't run end to end before release, and neither was anything on GitLab.
- `git add` of a credential file and a blind snapshot rewrite now stop for a question (`e442220`).
- `kit stats` gives the numbers a rule change is judged on, where the evals used to (`f20bc36`).
