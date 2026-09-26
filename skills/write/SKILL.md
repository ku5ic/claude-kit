---
description: Draft outward-facing text - commit message, PR description, release notes, devnote, explainer, review comment or reply, stakeholder summary
argument-hint: <commit|pr|release-notes|devnote|explainer|review-comment|review-reply|stakeholder> [kind arguments]
disable-model-invocation: true
allowed-tools:
  - Bash(git status *)
  - Bash(git diff *)
  - Bash(git log *)
  - Bash(git branch --show-current)
  - Bash(git rev-parse *)
  - Bash(gh repo view *)
  - Bash(gh pr view *)
---

## Dispatch

The first word of the arguments is the kind; everything after it is the kind's arguments.

| Kind             | Procedure                                                    |
| ---------------- | ------------------------------------------------------------ |
| `commit`         | [references/commit.md](references/commit.md)                 |
| `pr`             | [references/pr.md](references/pr.md)                         |
| `release-notes`  | [references/release-notes.md](references/release-notes.md)   |
| `devnote`        | [references/devnote.md](references/devnote.md)               |
| `explainer`      | [references/explainer.md](references/explainer.md)           |
| `review-comment` | [references/review-comment.md](references/review-comment.md) |
| `review-reply`   | [references/review-reply.md](references/review-reply.md)     |
| `stakeholder`    | [references/stakeholder.md](references/stakeholder.md)       |

1. Missing or unknown kind: ask via AskUserQuestion with the likeliest kinds as options.
2. Read the kind's reference file and follow it as the procedure.
3. In the reference, the `ARGUMENTS` token (written with a leading dollar sign) means everything after the kind word in the Arguments line below, and a backticked command prefixed with an exclamation mark means run that command via Bash and use its output.

Arguments: `$ARGUMENTS`
