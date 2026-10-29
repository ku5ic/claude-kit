# /pr review

Turn this session's review findings on someone else's PR into a draft review file: inline comments plus one summary comment. Arguments: `<PR number or URL> [report path]`.

This kind never runs a review, checks out the PR, or runs its checks. It runs last, after `/code-review`, `/verify`, and `decision-review`.

## Inputs

1. Findings, first match wins: the report path in `$ARGUMENTS`; else every review point this session produced, from `/code-review`, `/verify`, `decision-review`, or plain discussion, merged. None at all: stop and name the reviews to run, `/code-review <n>`, then `/verify` and `decision-review` as the change calls for.
2. The PR: from `$ARGUMENTS`, else from the findings. Neither names it: ask. Owner and repo come from the URL, or `gh repo view --json nameWithOwner`. Every `gh pr` call below passes `-R <owner>/<repo>`, so a PR from another repo resolves there and not in the cwd's repo.

## Pin the head

1. `gh pr view <n> -R <owner>/<repo> --json headRefOid,headRefName,title,body,files`.
2. `git fetch <remote> "+pull/<n>/head:refs/pr/<n>"`, where `<remote>` is the one `git remote -v` shows for `<owner>/<repo>`, else `https://github.com/<owner>/<repo>.git`. The `+` lets a later run follow a force-push.
3. `git rev-parse refs/pr/<n>` must equal `headRefOid`. Mismatch: stop and say so. Every later read uses that SHA.

zsh traps: write `"${head}:${file}"`, never `$head:src/...` (the `:s` history modifier fires). Never name a variable `path`; zsh ties it to `PATH`.

## Verify each point

For every finding, before drafting:

1. Name its decisive claim and trace it to the source at the head SHA: `git show <sha>:<file>`, a sibling file, a library's docs or source. Start from the finding's `Evidence:` line when it has one.
2. Confirmed: draft it. Unconfirmed, like library behavior nobody checked: phrase it as a question. Disproved: drop it and say so in one line of the reply.
3. Merge duplicates.

## Anchor each point

- Line: an inline comment on the RIGHT side, on a line inside the diff. Take the line number from `git show <sha>:<file>`, never from hunk offsets: a new or renamed file shows as "new file" in a pathspec-limited `git diff`. Confirm the line falls inside a hunk of `gh pr diff <n> -R <owner>/<repo>`.
- File: a point about a file's location or name is a file-level comment.
- None: a point with no line, like cases to try, goes in the summary comment.

## Write each comment

Shape and tone: `rules/output.md` section 4. No @mention, no praise, no summary of the change.

The summary comment opens with one line pointing at the inline comments, then one bullet per direct answer to a question the PR description asks, then the unanchored points. Nothing beyond the pointer: omit the summary section.

## Write the draft file

Write it to the path `kit scratch-dir pr-review <n>` prints. Inline comments go most important first.

```markdown
<draft header from SKILL.md>

## Summary comment (main thread)

<summary body>

## Inline comments

### [<path>:<line>](https://github.com/<owner>/<repo>/pull/<n>/files#diff-<sha256 of path>R<line>)

<body>

### [<path> (file)](https://github.com/<owner>/<repo>/pull/<n>/files#diff-<sha256 of path>)

<body>
```

The diff anchor is `printf '%s' "<path>" | shasum -a 256`. Reply with the file path, the comment count, the dropped points with the reason, and one next action: review the file, then `/pr publish <n>`. Post nothing.

## Review loop

The user edits the file or asks for changes; apply them with Edit. Deleting a section skips that comment. The file is ready for `publish` once the user says it's approved.
