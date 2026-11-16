---
name: write-commit
description: Draft a commit message from the staged or pasted diff, in the project's style
argument-hint: "<optional: short hint about the change intent>"
disable-model-invocation: true
allowed-tools:
  - Bash(git status *)
  - Bash(git diff *)
  - Bash(git log *)
  - Bash(git branch --show-current)
  - Bash(git rev-parse *)
---

# /write-commit

Generate a commit message from the staged or pasted diff, matching project style; with no history to match, use Conventional Commits.

Tone and shape: `skills/kit-rules/reference/output.md` section 4, which wins over this file; the sections this file defines stay.

<!-- chat: only
In claude.ai chat, the arguments are the user's message and the material is what they pasted. Nothing can be run: if the diff, log, or text this needs isn't in the conversation, ask for it.
-->

## Procedure

<!-- chat: skip -->

1. Check staged state: `git status --short`
2. Pull the staged diff: `git diff --cached`
3. Pull recent commit history to detect style: `git log --oneline -20`

<!-- chat: end -->

4. Detect the project's commit convention from the history:
   - Conventional Commits (`feat:`, `fix:`, `chore:`, scoped: `feat(auth):`)
   - Ticket prefix (`ABC-123:`, `#456:`)
   - Plain summary line
   - Emoji-based (gitmoji)
5. Write a message matching the detected style.

## Message rules

- Subject line: imperative mood, under 72 characters, no trailing period.
- Blank line, then body if the change warrants it. Body wraps at 72.
- Body explains why, not what. The diff shows what.

<!-- chat: skip -->

- Signatures, footers, and co-author tags: `skills/kit-rules/reference/workflow.md` section 1.

<!-- chat: end -->

- If arguments are provided, use them as the intent hint and shape the subject around them.
- If the staged diff is empty, say so and stop.
- If the staged diff mixes unrelated changes: say so, list the clusters, and propose splitting into separate commits.

## Output

Print the proposed message in a fenced block, then stop. Committing is the user's call.

<!-- chat: skip -->

Arguments: `$ARGUMENTS`

<!-- chat: end -->
