# 15. Release 0.14: rules that survive a compaction, and a closed retro loop

Date: 2026-11-26

## Status

Accepted

Amends [9. Release 0.10: the engineer in the loop, the rules as a gated skill, a visible claim checker](0009-release-0-10-the-engineer-in-the-loop-the-rules-as-a-gated-skill-a-visible-claim-checker.md)

Amends [2. Release 0.1: a Go binary of hooks around Claude Code's built-ins](0002-release-0-1-a-go-binary-of-hooks-around-claude-code-s-built-ins.md)

Amends [13. Release 0.13: kit rules outrank harness defaults, and the guards' holes closed](0013-release-0-13-kit-rules-outrank-harness-defaults-and-the-guards-holes-closed.md)

## Context

Sources are the commits on `feat/release-0-14-0-rules-survive-compaction-and-retro-loop`, each named where it's used, and the files they change.

- A compaction dropped the loaded rules and left ADR 9's rules gate open, and a subagent's load could lift the gate through a marker it shared with the skills gate (`fcf8ef4`). A summary could also drop the active plan, its next step, and the edited files (`c5aeda3`).
- Under ADR 2, session start filled the plugin cache's binary while the status line ran the marketplace copy, which stayed empty after an update (`8ba1f58`).
- Under ADR 13, a resubmit's change list came with the approval dialog already open (`1a2cd13`).
- A retro stopped at its findings (`eae156e`), and nothing read how each skill is written (`4e85745`).

## Decision

- The rules gate has its own marker. A `compacted` line in the skills log voids the main session's loads, so its next tool call is denied until kit-rules and every `global_skills` skill load again. A subagent's load counts for that subagent alone (`fcf8ef4`). Four more hard rules get one line each in kit-rules' `SKILL.md`, which a compaction carries over (`ea76790`). This amends ADR 9.
- The gate lets a workflow agent's `StructuredOutput` hand-back through, review-pick holds a `Workflow` launch as it holds `Agent` (`f628738`), and the gate's text names a Read of `SKILL.md` for agents without the Skill tool (`cfb291c`).
- `kit hook compact-note` on `PreCompact`, repeated at the session start after it, names the active plan and its first open step, the last Stop report, and the edited files (`c5aeda3`). Editing a plan with an open step outside plan mode makes it the session's active plan (`e007695`).
- A launcher under the marketplace folder with no binary runs the cache's binary of the same version, never another one, and a mod runs `bin/kit version` at load and on `/reload-plugins`, which downloads it (`8ba1f58`). This amends ADR 2.
- A changed resubmit is denied before the dialog opens, with its changes listed, until the user answers. Rule slug `plan-changes` (`1a2cd13`). Every plan edit but a tick is posted as a diff in the reply that made it (`c85cf76`). This amends ADR 13.
- Every plan step carries a proof, and plan-critic judges it (`ae3ee1c`). A step is called ready only once its proof passed (`fc96ef4`). A held-back instruction is a reply's first line (`de9a9e3`).
- `/meta-improve` turns retro findings into proposals backed by quotes from two sessions (`eae156e`). `/audit-skills` audits how skills, commands, and agents are written, one angle per agent, and a refuter per target tries to show each finding wrong before it's reported (`4e85745`, `963880e`, `de5e90e`). `/meta-retro` checks whether a command ran its own steps (`ae336dd`).
- `skills/kit-rules/reference/subagents.md` section 4 splits any skill's independent parts across agents at run time, with the model per kind of task (`d3cfa9c`, `e38ba62`), and the audits, `/audit-verify`, and `/code-deep-review` follow it (`321ddcc`, `c650a64`, `8dc1443`). Claude may start any procedure skill, and a posting skill still asks first (`b05b85f`).
- New: `kit doctor` (`e4dd760`), `kit stats --unused` (`a39ece2`), the `log-denied` hook for auto-mode denials (`b3ef360`), `maxTurns` on every agent, with a critic cut off at its cap counted as failed (`ad638ba`), and a subagent told its report budget at start (`12ff611`).
- The full gate runs each directory's checks in order and directories at once, its lines still in plan order, and no longer fails a check that changes files. The Stop checks still do (`go/internal/run/run.go`, `docs/checks.md`).
- A commit in a throwaway repo needs no approval. Git's own `rev-parse --absolute-git-dir` must put an existing repo under scratch, or the command must `git init` the repo first and commit only once that worked, and the command may run only git init, add, commit, cd, mkdir, mktemp, and `set` (`43c018d`, `dd321b6`, `b061ad6`, `557d0b2`). This narrows ADR 9's "Claude never commits or pushes unasked".
- A Claude Code settings file under scratch is written with no ask, since no session reads a fixture there (`6ca1ca4`). This narrows ADR 9's ask on settings files, as ADR 13 did for the plugin root's scratch.
- Fixes that change what a user sees: `kit git-base` reads `branch.<name>.gh-merge-base` (`aa7ceb8`), the imports guard allows `claude-code` in a plugin (`7402338`), and `kit scratch-rotate` deletes old checkouts and symlinks (`19502c9`).

## Consequences

- After a compaction, the main session's first tool call is denied until the rules load again (`fcf8ef4`).
- The status line stays empty after a background update until a reload or a new session, when the mod's download runs (`8ba1f58`).
- A stale `gh-merge-base` naming a kept parent widens the diff, and unsetting it restores the old order (`docs/cli.md`).
