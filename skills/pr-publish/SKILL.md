---
name: pr-publish
description: Post an approved PR draft: a review, replies, or a description
argument-hint: "[PR number]"
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

# /pr-publish

Post a draft the user approved: a review, replies, or a description.

Never push, approve, request changes, resolve threads, or edit or delete existing comments.

Tone and shape: `skills/kit-rules/reference/output.md` section 4, which wins over this file; the sections this file defines stay.

## Find the draft

1. With a PR number: the drafts in `kit scratch-dir` with prefix `pr-review-<n>` or `pr-reply-<n>`, plus any `pr-description-*.md` whose header `PR:` line names `#<n>`. Without one: the current branch's drafts with prefix `pr-description-<branch>` (`/` in the branch name becomes `-`), plus the review and reply drafts of its open PR, if any. A branch draft counts only when its header's `Branch:` line names the branch or its `PR:` line names that open PR: another branch's draft can share its name. Never a wider glob: `pr-review-1*` picks up PR 12's.
2. Take the newest file per kind (`ls -t`). If there's none, stop and name the drafting kind to run. If there's more than one kind, ask which via AskUserQuestion.
3. Read the draft's header: `Kind`, `PR` or `Branch`, `Head`. `<repo>` comes from the `PR:` line, or from the repository path for a `Branch:` header.
4. Read the draft's post log, `<draft path without .md>.posted.log`, if it exists. Sections it lists are already posted and are skipped. Every section posted: say so and stop.

## Check before posting

1. Head, for a review: the PR's head SHA; for a description: `git rev-parse HEAD`. Different from the header's `Head`: stop, say so, and offer to re-draft. A reply skips this check: pushing the fixes it promises moves the head, and its anchors are comment ids, not lines.
2. Review: every inline anchor's line still falls inside a hunk of the PR's diff.
3. Description: whether the branch has an open PR is checked now, never taken from the header. No open PR: `git ls-remote origin "refs/heads/<branch>"` must print the header's `Head`. Missing or different: stop with "push first". Never push.
4. Split the draft into one body file per unposted comment under scratch, through Write, each byte-identical to its section's postable text with surrounding blank lines trimmed. The header, headings, and a reply's leading quote are not comment text. A description gets a title file holding the `Title:` line's text and a body file holding everything after that line.
5. One AskUserQuestion with options `Post` and `Cancel`: "Post N comments to PR <n>?", or for a description "Update the title and body of PR <n>?" or "Open a PR from <branch>?". Nothing is posted without `Post` from this session.

## Post

Review and reply post one comment per API call, at least 30 s apart, in file order, except a review's summary, which goes last. A batched review can't be paced, so never use one.

Each section's call is the Post table in `skills/code-host-patterns`.

The title comes from a file because a title like `feat(pr): /pr skill` would be shell-expanded inside a typed double-quoted string; a command substitution's output isn't.

Run the review or reply loop as one Bash call with `run_in_background`, `sleep 30` between posts inside it; the harness blocks a foreground `sleep`. After each post, append the section heading and the URL and `created_at` that `skills/code-host-patterns` names as one tab-separated line to the post log. A failed post stops the loop: report it, never retry blind; a rerun resumes from the log.

30 s sits under each host's content limits (`skills/code-host-patterns`). Some endpoints have lower, unnamed limits, so a 403 or 429 means stop and report.

## Check, then reply

1. Fetch each posted comment (a description: the PR's title and body) and diff its text against its body file. Any difference is reported, never silently edited.
2. Inline review comments: the returned `path` and `line` match the anchor.
3. Consecutive `created_at` values in the post log are at least 30 s apart.

Reply: the PR URL, the posted count, and any mismatch. Ask before deleting the body files; keep the draft and its post log.

## Draft header

Every `pr-*` draft starts with this header, and this skill parses it.

```markdown
# PR <n or branch> <kind> draft (not posted)

Kind: <description|review|reply>
PR: <repo>#<n>
Head: <full head SHA the draft was written against>
```

`<repo>` is the host's repository path (`skills/code-host-patterns`). A description for a branch with no PR yet writes `Branch: <name>` in place of the `PR:` line.

Arguments: `$ARGUMENTS`
