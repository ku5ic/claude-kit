---
name: pr-review
description: Turn this session's review findings on someone else's PR into a draft review with inline comments
argument-hint: "<PR number or URL> [report path]"
allowed-tools:
  - Bash(gh pr view *)
  - Bash(gh pr diff *)
  - Bash(gh repo view *)
  - Bash(glab mr view *)
  - Bash(glab mr diff *)
  - Bash(glab repo view *)
  - Bash(git show *)
  - Bash(git fetch *)
  - Bash(git rev-parse *)
  - Bash(git branch --show-current)
---

# /pr-review

Turn this session's review findings on someone else's PR into a draft review file with inline comments plus one summary comment.

This kind never runs a review, checks out the PR, or runs its checks. It runs last, after `/code-review`, `/verify`, and `/code-challenge`.

Tone and shape: `skills/kit-rules/reference/output.md` section 4, which wins over this file; the sections this file defines stay.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                                 | Read                                                   |
| -------------------------------------- | ------------------------------------------------------ |
| Deciding which findings earn a comment | [reference/review-scope.md](reference/review-scope.md) |

## Inputs

1. Findings, first match wins: the report path in `$ARGUMENTS`; else every review point this session produced, from `/code-review`, `/verify`, `/code-challenge`, or plain discussion, merged. None at all: stop and name the reviews to run, `/code-review <n>`, then `/verify` and `/code-challenge` as the change calls for.
2. The PR: from `$ARGUMENTS`, else from the findings. Neither names it: ask. `<repo>` comes from the URL, or the repository path in `skills/code-host-patterns`. Every call takes it, so a PR from another repo resolves there and not in the cwd's repo.

## Pin the head

1. Read the PR's head SHA, branch, title, body, and files.
2. `git fetch <remote> "<head ref>"` with the host's head ref from `skills/code-host-patterns`, where `<remote>` is the one `git remote -v` shows for `<repo>`, else the repository's clone URL. The `+` lets a later run follow a force-push.
3. `git rev-parse refs/pr/<n>` must equal the head SHA. Mismatch: stop and say so. Every later read uses that SHA.

Write `"${head}:${file}"`; the zsh traps are `skills/bash-patterns/reference/quoting.md`.

## Verify each point

For every finding, before drafting:

1. Name its decisive claim and trace it to the source at the head SHA: `git show <sha>:<file>`, a sibling file, a library's docs or source. Start from the finding's `Evidence:` line when it has one.
2. Draft a confirmed point. Phrase an unconfirmed one, like library behavior nobody checked, as a question. Drop a disproved one and say so in one line of the reply.
3. Merge duplicates.

## Anchor each point

- Line: an inline comment on the new side, on a line inside the diff. Take the line number from `git show <sha>:<file>`, never from hunk offsets, since a new or renamed file shows as "new file" in a pathspec-limited `git diff`. Confirm the line falls inside a hunk of the PR's diff.
- File: a point about a file's location or name is a file-level comment.
- None: a point with no line, like cases to try, goes in the summary comment.

## Write each comment

Shape and tone: `skills/kit-rules/reference/output.md` section 4. No @mention, no praise, no summary of the change.

The summary comment opens with one line pointing at the inline comments, then one bullet per direct answer to a question the PR description asks, then the unanchored points. With nothing beyond the pointer, omit the summary section.

## Write the draft file

Write it to the path `kit scratch-dir pr-review <n>` prints. Inline comments go most important first.

```markdown
<draft header from `skills/pr-publish/SKILL.md`>

## Summary comment (main thread)

<summary body>

## Inline comments

### [<path>:<line>](<link to the line at the head>)

<body>

### [<path> (file)](<link to the file at the head>)

<body>
```

The links take the shape `skills/code-host-patterns` gives. Reply with the file path, the comment count, the dropped points with the reason, and one next action: review the file, then `/pr-publish <n>`. Post nothing.

## Review loop

The user edits the file or asks for changes; apply them with Edit. Deleting a section skips that comment. The file is ready for `publish` once the user says it's approved.
