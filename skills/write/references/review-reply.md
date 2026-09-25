# /write review-reply

Draft replies to a reviewer's PR comments, verifying each claim against the actual code before answering. Arguments: `<reviewer username> [PR number] [--all]`.

## Procedure

1. Get the scratch directory: `!`scratch-dir.sh``.
2. Resolve the repo slug: `gh repo view --json nameWithOwner -q .nameWithOwner`. If this fails (no remote, no auth), stop and say so.
3. Resolve the current GitHub user: `gh api user -q .login`. Used later to detect already-answered threads.
4. Parse `$ARGUMENTS`:
   - First non-flag token: reviewer username.
   - A bare number: PR number.
   - `--all`: include every review round, not just the newest (see step 7).
5. If no PR number was given, resolve the current branch's PR: `gh pr view --json number,url`. If that fails (not on a PR branch, no open PR), ask for the PR number or URL via the AskUserQuestion tool instead of guessing.
6. If no reviewer username was given:
   - List distinct reviewers from `gh api repos/<slug>/pulls/<n>/reviews` and `gh api repos/<slug>/pulls/<n>/comments`, excluding the PR author and the current user.
   - Exactly one candidate: use it.
   - Zero or more than one: ask via the AskUserQuestion tool, listing the candidates, instead of guessing.
7. Fetch the reviewer's comments:
   - Inline: `gh api repos/<slug>/pulls/<n>/comments --paginate`, filtered to `user.login == <reviewer>`. Keep `id`, `path`, `line` (or `original_line`), `body`, `html_url`, `created_at`, `in_reply_to_id`.
   - Reviews: `gh api repos/<slug>/pulls/<n>/reviews --paginate`, filtered to the same user. Keep `id`, `body`, `html_url`, `submitted_at`. Include a review's own `body` as a comment only if non-empty.
8. Scope to the newest round, unless `--all` was passed:
   - Group everything from step 7 by calendar date (from `created_at` / `submitted_at`).
   - Keep only the most recent date's items.
   - Note any older items that were excluded, in one line, at the top of the output file.
9. Drop already-answered threads: for each inline comment, check whether any other comment in the full (unfiltered) `pulls/comments` list has `in_reply_to_id` equal to its `id` and `user.login` equal to the current user from step 3. If so, skip it.
10. For each remaining comment, investigate before answering:
    - Read the referenced file at the given line, plus enough surrounding context to understand the claim (the function, its callers, related types/hooks).
    - Trace the actual behavior: if the comment claims something is dead code, unreachable, redundant, or inconsistent, verify it by reading the code paths involved, not by re-reading the comment.
    - If the comment concerns a contract this repo doesn't own (e.g. a backend API, an external service), check whether a project-specific scout/backend agent is configured (`/agents` in this session) and use it; otherwise verify from what's available (schema files, generated types, docs in-repo) and say plainly what could not be confirmed.
    - Do not fabricate verification. Anything not actually checked against the code is a judgment call, and gets flagged as one in the answer.

## Deciding the answer

For each comment, once investigated:

- **Confirmed defect** - agree, state what you verified (file/function), describe the fix in one or two lines. Don't paste a full patch.
- **Style nit** - check for conflicting local precedent first (sibling files, project conventions in CLAUDE.md or loaded pattern skills). Agreeing blindly when it contradicts existing local pattern is a failure mode - name the conflict and let the user pick, don't silently comply or silently refuse.
- **Incorrect premise** - push back with the specific evidence (file:line, quoted code) that contradicts the claim. State it plainly, don't soften it into a question unless it genuinely is one.
- **Design/architecture question** ("why did you do X") - answer with the actual reasoning found in the code, tests, or related hooks. If the separation was accidental rather than intentional, say so - don't invent a rationale after the fact.
- **Genuine open question** - say what you found, say what's still unresolved, and end with a specific decision the user needs to make.

## Output

Write to `$(scratch-dir.sh)/pr-<pr-number>-<reviewer>-review-replies-<YYYYMMDD-HHMM>.md`. Print the path.

Structure:

```
# PR #<n> - replies to <reviewer>'s latest review

<One line noting the round scoped to, and what was excluded (older rounds, already-answered threads).>

---

### <n>. <short title for the comment's topic>
[<path>:<line>](<html_url>)
> <quoted original comment body>

**Answer:**
<Short bullets if the answer has more than one point. One or two plain sentences if it doesn't - don't force bullets on a one-liner.>

---
<repeat per comment>

## Fixes to make
<Numbered list of agreed action items, deduped where multiple comments point at the same fix. Omit if nothing was agreed to.>
```

## Rules

- Verify before agreeing. Every "confirmed" or "agreed" answer must point at something actually read this session, not a paraphrase of the reviewer's own claim.
- Tone: friendly, short, peer-to-peer - the PR author replying to a colleague, not a report. No "Thank you for the feedback", no groveling, no corporate hedging.
- Disagree plainly when the code contradicts the reviewer. State the evidence, don't soften it.
- Never post anything to GitHub. This drafts a file only - posting the replies is a separate, explicit action the user takes themselves.
- If a comment's thread already has a reply from the current user (step 9), it's excluded silently from the per-comment sections, but counted in the round-summary line at the top.
- If zero comments remain after scoping and filtering, say so and stop - do not write an empty file.
