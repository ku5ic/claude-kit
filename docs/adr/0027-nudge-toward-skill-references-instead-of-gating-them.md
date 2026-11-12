# 27. Nudge toward skill references instead of gating them

Date: 2026-11-12

## Status

Accepted

Amends [3. Enforce rules with hooks, not prose](0003-enforce-rules-with-hooks-not-prose.md)

## Context

In real sessions, 40 of 168 pattern-skill loads went on to read a reference (664d02b). That commit added `reference-gate`, which denied every tool call until each file in a loaded skill's "Before | Read" table was read. 8bd6664 removed it a day later, because it loaded every reference of every loaded skill before any tool call, whatever the task. [3](0003-enforce-rules-with-hooks-not-prose.md) runs a rule a hook can see as a hook, and says each new block has to earn the turn the user waits through.

## Decision

No hook blocks on a reference read. `<required-skills>` asks for every reference of a global skill (`requiredReferences` in `go/internal/stackctx/stackctx.go`), and each skill's table asks for its own. When a file mapped to a patterns skill is read through Read, Grep with a path or glob, or a Bash command, or edited with `guard_skills` off, `skills-nudge` names the skill and its rows once per skill per session without blocking (701b343, 6b1d2b5). With `guard_skills` on, as the strict preset sets it, an edit still waits until the file's skill is loaded.

## Consequences

- Reading a reference is advice again, and Claude can skip it.
- In trigger evals of 5 runs per case, the nudge at a Read raised python loads to 4 of 5, from 8 of 15, and typescript to 5 of 5, from 5 of 15. At the edit it changed nothing (701b343).
- The light preset turns the nudge off.
