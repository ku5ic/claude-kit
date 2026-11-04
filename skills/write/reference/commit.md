# /write commit

<!-- chat: skip -->

Generate a commit message from the staged diff, matching project style. Arguments: `<optional: short hint about the change intent>`.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, generate a commit message from the diff the user pasted, matching the style of any history they pasted. With no history, use Conventional Commits.
-->

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
- If the staged diff is empty: say so and stop.
- If the staged diff mixes unrelated changes: say so, list the clusters, and propose splitting into separate commits.

## Output

Print the proposed message in a fenced block. Then stop.

<!-- chat: skip -->

Do not run `git commit`.

The user decides whether to pipe it: `git commit -m "$(pbpaste)"` or open an editor.

<!-- chat: end -->
