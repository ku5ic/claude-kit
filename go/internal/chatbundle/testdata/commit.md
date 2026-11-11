# /write commit

<!-- chat: skip -->

Generate a commit message from the staged diff, matching project style. Arguments: `<optional: short hint about the change intent>`.

<!-- chat: end -->

<!-- chat: only
In claude.ai chat, generate a commit message from the diff in the user's message.
-->

## Procedure

<!-- chat: skip -->

1. Check staged state: !`git status --short`
2. Pull the staged diff: !`git diff --cached`
3. Pull recent commit history to detect style: !`git log --oneline -20`

<!-- chat: end -->

4. Detect the project's commit convention from the history.

## Message rules

- Subject line: imperative mood, under 72 characters, no trailing period.
- Commit register: `skills/kit-rules/reference/output.md` section 4.

<!-- chat: skip -->

- Signatures, footers, and co-author tags: `skills/kit-rules/reference/workflow.md` section 1.

<!-- chat: end -->

- If the staged diff is empty: say so and stop.

```html
<!-- failure -->
<!-- chat: skip -->
```
