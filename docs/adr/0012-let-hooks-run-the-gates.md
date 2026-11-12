# 12. Let hooks run the gates

Date: 2026-10-28

## Status

Accepted

## Context

In a large frontend repo, `kit run-checks` took about 8 minutes and pinned the CPU (PR #10). Its format check also failed on files under `.claude/scratch/`, which only the user's global git excludes ignored. A checker agent ran the gates, and a rule asking for a run is advice ([3](0003-enforce-rules-with-hooks-not-prose.md)).

## Decision

Hooks own running the gates. The Stop hook checks a turn's edits. When a `/code-review` first stops, its SubagentStop hook runs the full gate on the checkout and sends the reviewer back once with any FAIL. A pass records the tree, so Stop doesn't check it again. A review in a worktree is left to the PR's CI. The bash guard blocks Claude from running `kit run-checks`, `--plan` still lists the gates, and the checker agent is gone. The full gate checks what changed since the git base, and `--all` checks the whole tree. Made in 0.5.0 (PR #10).

## Consequences

- Every run shows the user each gate's verdict, and a reply never repeats them.
- A gate the hook's time budget kills is a skip, not a FAIL.
- The gate's scope depends on the git base, so a wrong base checks the wrong files.
