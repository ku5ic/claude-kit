---
name: pr
description: Draft what lands in a PR thread - your PR's title and body, a review of someone else's PR, replies to the review on yours - then post an approved draft
argument-hint: <description|review|reply|publish> [kind arguments]
disable-model-invocation: true
allowed-tools:
  - Bash(gh pr view *)
  - Bash(gh pr diff *)
  - Bash(gh repo view *)
  - Bash(git show *)
  - Bash(git fetch *)
  - Bash(git rev-parse *)
  - Bash(git branch --show-current)
---

## Dispatch

The first word of the arguments is the kind; everything after it is the kind's arguments.

| Kind          | Use when                            | Procedure                                            |
| ------------- | ----------------------------------- | ---------------------------------------------------- |
| `description` | drafting your PR's title and body   | [reference/description.md](reference/description.md) |
| `review`      | you reviewed someone else's PR      | [reference/review.md](reference/review.md)           |
| `reply`       | answering the review on your PR     | [reference/reply.md](reference/reply.md)             |
| `publish`     | posting an approved draft to GitHub | [reference/publish.md](reference/publish.md)         |

1. Missing or unknown kind: ask via AskUserQuestion with the four kinds above as options.
2. Read the kind's reference file and follow it as the procedure.
3. In the reference, the `ARGUMENTS` token (written with a leading dollar sign) means everything after the kind word in the Arguments line below, and a backticked command prefixed with an exclamation mark means run that command via Bash and use its output.
4. Every kind follows `rules/output.md` section 4 (External communication). On tone and shape it wins over the reference file; the sections a reference file defines, such as the PR template handling, stay.
5. Only `publish` posts. The drafting kinds write a file and post nothing.

## Draft header

Every draft starts with this header; `publish` parses it.

```markdown
# PR <n or branch> <kind> draft (not posted)

Kind: <description|review|reply>
PR: <owner>/<repo>#<n>
Head: <full head SHA the draft was written against>
```

A description for a branch with no PR yet writes `Branch: <name>` in place of the `PR:` line.

Arguments: `$ARGUMENTS`
