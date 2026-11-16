---
name: pr-reply
description: Draft replies to the review on your PR, each claim checked against the code
argument-hint: "[PR number] [reviewer username] [--all]"
disable-model-invocation: true
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

# /pr-reply

Draft replies to a reviewer's comments on your PR, verifying each claim against the actual code before answering.

Tone and shape: `skills/kit-rules/reference/output.md` section 4, which wins over this file; the sections this file defines stay.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                           | Read                                                       |
| -------------------------------- | ---------------------------------------------------------- |
| Deciding how to answer a comment | [reference/reply-practice.md](reference/reply-practice.md) |

## Procedure

1. Resolve the repository path (`skills/code-host-patterns`, as every call below). If this fails (no remote, no auth), stop and say so.
2. Resolve the current user. Used later to detect already-answered threads.
3. Parse `$ARGUMENTS`:
   - A bare number: PR number.
   - The other non-flag token: reviewer username.
   - `--all`: include every review round, not just the newest (see step 6).
4. If no PR number was given, resolve the current branch's open PR. If that fails (not on a PR branch, no open PR), ask for the PR number or URL via the AskUserQuestion tool instead of guessing.
5. If no reviewer username was given:
   - List distinct reviewers from the review threads, excluding the PR author and the current user.
   - Exactly one candidate: use it.
   - Zero or more than one: ask via the AskUserQuestion tool, listing the candidates, instead of guessing.
6. Fetch the reviewer's comments:
   - Inline: every comment of theirs in the review threads. Keep its id, thread id, path, line, body, URL, and creation time.
   - Reviews, on a host with review bodies: each of theirs, kept as a comment only if its body isn't empty.
7. Scope to the newest round, unless `--all` was passed:
   - Group everything from step 6 by calendar date, from its creation time.
   - Keep only the most recent date's items.
   - Note any older items that were excluded, in one line, at the top of the output file.
8. Drop already-answered threads: a thread that holds a comment by the current user from step 2, posted after the reviewer's, is skipped.
9. For each remaining comment, investigate before answering:
   - Read the referenced file at the given line, plus enough surrounding context to understand the claim (the function, its callers, related types/hooks).
   - Trace the actual behavior: if the comment claims something is dead code, unreachable, redundant, or inconsistent, verify it by reading the code paths involved, not by re-reading the comment.
   - If the comment concerns a contract this repo doesn't own (e.g. a backend API, an external service), check whether a project-specific scout/backend agent is configured (`/agents` in this session) and use it; otherwise verify from what's available (schema files, generated types, docs in-repo) and say plainly what could not be confirmed.
   - Label what wasn't checked against the code, per `skills/kit-rules/reference/evidence.md` section 1.

## Deciding the answer

For each comment, once investigated:

- **Confirmed defect**: agree, state what you verified (file/function), describe the fix in one or two lines. Don't paste a full patch.
- **Style nit**: check for conflicting local precedent first (sibling files, project conventions in CLAUDE.md or loaded pattern skills). Agreeing blindly when it contradicts existing local pattern is a failure mode: name the conflict and let the user pick, don't silently comply or silently refuse.
- **Incorrect premise**: push back with the evidence that contradicts the claim, per `skills/kit-rules/reference/workflow.md` section 4.
- **Design/architecture question** ("why did you do X"): answer with the actual reasoning found in the code, tests, or related hooks. If the separation was accidental rather than intentional, say so; don't invent a rationale after the fact.
- **Genuine open question**: say what you found, say what's still unresolved, and end with a specific decision the user needs to make.

## Output

Write to the path `kit scratch-dir pr-reply <n>` prints, opening with the draft header from `skills/pr-publish/SKILL.md` (`Head:` is the PR's head SHA).

Structure:

```markdown
<draft header>

Replies to <reviewer>. <One line noting the round scoped to, and what was excluded (older rounds, already-answered threads).>

## Replies

### [<path>:<line>](html_url) reply to <thread id>

> <quoted original comment body>

<The reply. One to three sentences; short bullets only when it answers more than one point.>

### [review](html_url) review <review id>

> <quoted review body>

<The reply.>

## Fixes to make

<Numbered list of agreed action items, deduped where multiple comments point at the same fix. Omit if nothing was agreed to.>
```

One `###` section per comment, in the order the reviewer posted them. The id after "reply to" is the thread id the host's reply call takes, per `skills/code-host-patterns`. `publish` posts each section's text after the quote; the heading and the quote are not posted, and neither is "Fixes to make". A `review` section answers a review-level body, which has no thread to reply in.

## Rules

- Verify before agreeing (`skills/kit-rules/reference/workflow.md` section 4): every "confirmed" or "agreed" answer points at something read this session, not a paraphrase of the reviewer's claim.
- Tone and shape: `skills/kit-rules/reference/output.md` section 4. Acknowledging a finding: `skills/kit-rules/reference/workflow.md` section 4.
- Post nothing. Posting the approved file is `/pr-publish`.
- If a comment's thread already has a reply from the current user (step 8), it's excluded silently from the per-comment sections, but counted in the round-summary line at the top.
- If zero comments remain after scoping and filtering, say so and stop; do not write an empty file.
