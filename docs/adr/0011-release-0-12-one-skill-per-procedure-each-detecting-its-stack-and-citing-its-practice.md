# 11. Release 0.12: one skill per procedure, each detecting its stack and citing its practice

Date: 2026-11-16

## Status

Accepted

## Context

Sources are the commits on `feat/kit-0-12-0`, each named where it's used.

- Procedure kinds hid behind dispatchers the / menu can't complete (`eab6345`), and the skills named hosts and stacks (`8e8d543`) and ran on practice nothing cited (`a2bf204`).
- Audit and impact reports skipped the claims gate (`d22c22c`), `curl -d @.env` and `cat .e*` passed the bash guard (`e442220`), and nothing measured a rule since the evals went (`f20bc36`).
- `kit blast-radius` read only JS, TS, and Python (`5d7c66f`), and adding a language had no written path (`820626e`).

## Decision

- Each procedure kind is its own user-only skill, which detects the stack, the code host, and the connectors, runs its own prerequisites, and keeps its practice and sources in its own `reference/` (`eab6345`, `8e8d543`, `6d2a480`, `a2bf204`).
- The claims gate checks every review, audit, and code report (`d22c22c`). Credential reads through `curl @`, globs, and `git add` are caught (`e442220`).
- Overlays can remove a default with `"!x"`, `shadow_rules` logs a rule without blocking, and `kit stats` reads the logs (`7688fe5`, `c714806`, `f20bc36`).
- An edit names the accepted ADRs that cite its file (`10f17ff`).
- The README keeps what decides an install, and `docs/commands.md`, `docs/skills.md`, `docs/cli.md`, `docs/chat.md`, `docs/dependencies.md`, and `docs/adding-a-stack.md` join the people-facing pages (`37dac6a`, `5d7c66f`, `820626e`).
- `codegraph` owns who uses a file in every language, and an e2e test fails when a scanned language lacks comment syntax or the skill map (`5d7c66f`, `5964a1e`).
- Session start prunes old scratch once a day, and `kit clean` removes the kit's own state (`6b0ab0d`).

## Consequences

- `/audit <kind>` and the other dispatcher forms no longer work (`eab6345`).
- The new skills of `768f234`, `a633296`, `871da10`, and `7ec6f07` weren't run end to end, and nothing on GitLab was.
- JS, TS, and Python repos need ast-grep for blast-radius (`5d7c66f`).
