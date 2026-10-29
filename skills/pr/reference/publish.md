# /pr publish

Post a draft the user approved: a review, replies, or a description. Arguments: `[PR number]`.

Never push, approve, request changes, resolve threads, or edit or delete existing comments.

## Find the draft

1. With a PR number: the drafts in `kit scratch-dir` named `pr-review-<n>-*` and `pr-reply-<n>-*`, plus any `pr-description-*` whose header `PR:` line names `#<n>`. Without one: the current branch's `pr-description-<branch>-*` (`/` in the branch name becomes `-`), plus the review and reply drafts of its open PR (`gh pr view --json number`), if any.
2. Take the newest file per kind (`ls -t`). None: stop and name the drafting kind to run. More than one kind: ask which via AskUserQuestion.
3. Read the draft's header: `Kind`, `PR` or `Branch`, `Head`. Every `gh pr` call below passes `-R <owner>/<repo>` from the `PR:` line, or from `gh repo view --json nameWithOwner` for a `Branch:` header.
4. Read the draft's post log, `<draft path without .md>.posted.log`, if it exists. Sections it lists are already posted and are skipped. Every section posted: say so and stop.

## Check before posting

1. Head, for a review: `gh pr view <n> --json headRefOid`; for a description: `git rev-parse HEAD`. Different from the header's `Head`: stop, say so, and offer to re-draft. A reply skips this check: pushing the fixes it promises moves the head, and its anchors are comment ids, not lines.
2. Review: every inline anchor's line still falls inside a hunk of `gh pr diff <n>`.
3. Description: whether a PR is open is checked now (`gh pr view <branch> --json number`), never taken from the header. No open PR: `git ls-remote origin "refs/heads/<branch>"` must print the header's `Head`. Missing or different: stop with "push first". Never push.
4. Split the draft into one body file per unposted comment under scratch, through Write, each byte-identical to its section's postable text with surrounding blank lines trimmed. The header, headings, and a reply's leading quote are not comment text. A description gets a title file holding the `Title:` line's text and a body file holding everything after that line.
5. One AskUserQuestion with options `Post` and `Cancel`: "Post N comments to PR <n>?", or for a description "Update the title and body of PR <n>?" or "Open a PR from <branch>?". Nothing is posted without `Post` from this session.

## Post

Review and reply post one comment per API call, at least 30 s apart, in file order, except a review's summary, which goes last. A batched `POST /pulls/<n>/reviews` can't be paced, so never use it.

| Draft section                        | Call                                                                                                                                         |
| ------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------- |
| Review, line                         | `gh api -X POST repos/<owner>/<repo>/pulls/<n>/comments -f commit_id=<sha> -f path=<file> -F line=<line> -f side=RIGHT -F body=@<body-file>` |
| Review, file                         | `gh api -X POST repos/<owner>/<repo>/pulls/<n>/comments -f commit_id=<sha> -f path=<file> -f subject_type=file -F body=@<body-file>`         |
| Review summary, or reply to a review | `gh api -X POST repos/<owner>/<repo>/issues/<n>/comments -F body=@<body-file>`                                                               |
| Reply to a comment                   | `gh api -X POST repos/<owner>/<repo>/pulls/<n>/comments/<thread id>/replies -F body=@<body-file>`                                            |
| Description, PR open                 | `gh pr edit <n> --title "$(cat <title-file>)" --body-file <body-file>`                                                                       |
| Description, no PR                   | `gh pr create --head <branch> --title "$(cat <title-file>)" --body-file <body-file>`                                                         |

The title comes from a file because a title like `feat(pr): /pr skill` would be shell-expanded inside a typed double-quoted string; a command substitution's output isn't.

Run the review or reply loop as one Bash call with `run_in_background`, `sleep 30` between posts inside it; the harness blocks a foreground `sleep`. After each post, append the section heading, the response's `html_url`, and its `created_at` (`--jq`) as one tab-separated line to the post log. A failed post stops the loop: report it, never retry blind; a rerun resumes from the log.

30 s sits well under GitHub's secondary limits for content creation (80 per minute, 500 per hour, [docs](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)). Some endpoints have lower, unnamed limits, so a 403 or 429 means stop and report.

## Check, then reply

1. Fetch each posted comment (a description: `gh pr view <n> --json title,body`) and diff its text against its body file. Any difference is reported, never silently edited.
2. Inline review comments: the returned `path` and `line` match the anchor.
3. Consecutive `created_at` values in the post log are at least 30 s apart.

Reply: the PR URL, the posted count, and any mismatch. Ask before deleting the body files; keep the draft and its post log.
