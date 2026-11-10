# /write commit

Generate a commit message from the staged or pasted diff, matching project style; with no history to match, use Conventional Commits. Arguments: `<optional: short hint about the change intent>`.

## Procedure

<!-- chat: skip -->

1. Check staged state: !`git status --short`
2. Pull the staged diff: !`git diff --cached`
3. Pull recent commit history to detect style: !`git log --oneline -20`

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

- Signatures, footers, and co-author tags: `rules/workflow.md` section 1.

<!-- chat: end -->

- If arguments are provided, use them as the intent hint and shape the subject around them.
- If the staged diff is empty, say so and stop.
- If the staged diff mixes unrelated changes: say so, list the clusters, and propose splitting into separate commits.

## Output

Print the proposed message in a fenced block, then stop. Committing is the user's call.
