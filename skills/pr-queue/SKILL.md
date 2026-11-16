---
name: pr-queue
description: Your review queue, PRs waiting on your review and your own PRs needing action, ranked
argument-hint: "[org, group, or repo]"
disable-model-invocation: true
allowed-tools:
  - Bash(gh pr checks *)
  - Bash(gh pr list *)
  - Bash(gh pr view *)
  - Bash(gh search prs *)
  - Bash(glab mr list *)
  - Bash(kit scratch-dir *)
---

# /pr-queue

The PRs waiting on you: review requests, and your own PRs with red CI or unanswered comments, ranked so the one that unblocks the most comes first. Read-only.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before            | Read                                         |
| ----------------- | -------------------------------------------- |
| Ranking the queue | [reference/ranking.md](reference/ranking.md) |

## Procedure

1. The host and the current user, per `skills/kit-rules/reference/tooling.md` section 4 and the code-host-patterns skill's "Current user" row. No CLI and no connector, say so and stop.
2. Scope. The argument's org, group, or repo, else every repo the user can see. The report names it.
3. Review requests, then the user's open PRs, by the code-host-patterns skill's queue rows. One call per PR for size and CI where the list lacks them.
4. Drop PRs the user already reviewed at their current head, and bot PRs unless asked. Mark drafts.
5. Per PR, derive the age since the request, its additions plus deletions, whether it's blocked on the user, red CI, and unanswered comments.
6. Rank per [reference/ranking.md](reference/ranking.md) into "Needs my review" and "My PRs needing action".

## Report

Load the report-format skill and use its format, written to the path `kit scratch-dir queue <scope-slug>` prints. One row per PR with its link, age, size, and why it ranks where it does. A source that failed is named.

## Reply

The report path and the count in each list.

## Stop

Read-only. No comment, approval, review request, merge, or ready-for-review.

Arguments: `$ARGUMENTS`
